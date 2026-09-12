use std::ffi::{CStr, CString, c_char, c_int};
use std::path::Path;

use self::engine::engine::{CreateClowderDto, ENGINE, Engine, InputVectorDto};
use self::segment::resource::VectorMetadata;
use self::shared::hairball::Hairball;
use self::shared::results::{NekoMetadata, NekoSearchResult, NekoStats};

pub mod engine;
pub mod manifest;
pub mod segment;
pub mod shared;
pub mod wal;

unsafe fn c_str_to_string(ptr: *const c_char) -> Option<String> {
    if ptr.is_null() {
        None
    } else {
        let string = unsafe { CStr::from_ptr(ptr) }.to_string_lossy().to_string();
        Some(string)
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn neko_version() -> c_int {
    0
}

/// Initialize the engine. Subsequent calls are no-ops.
///
/// # Safety
/// `data_directory` must be a valid, null-terminated C string pointing to a writable path.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_init(data_directory: *const c_char) -> c_int {
    let raw_string = unsafe { c_str_to_string(data_directory) };
    let path = match raw_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };
    match Engine::init(Path::new(&path)) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}

#[unsafe(no_mangle)]
pub extern "C" fn neko_shutdown() -> c_int {
    0
}

/// Create a new collection.
///
/// # Safety
/// `name` must be a valid, null-terminated C string. `model` may be null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_create(name: *const c_char, dim: u32, metric: u8, model: *const c_char) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InvalidName as c_int,
    };

    let raw_model_string = unsafe { c_str_to_string(model) };
    let model_str = raw_model_string.filter(|string| !string.is_empty());
    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    match engine.write().unwrap().create_clowder(CreateClowderDto {
        name: &name_str,
        dim,
        metric,
        model: model_str.as_deref(),
    }) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}

/// List all collection names. Caller must free with `neko_free_strings`.
///
/// # Safety
/// `names` and `count` must be valid, non-null pointers to writable memory.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_list(names: *mut *mut *mut c_char, count: *mut u32) -> c_int {
    if names.is_null() || count.is_null() {
        return Hairball::InternalError as c_int;
    }
    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };
    let collection_names = engine.read().unwrap().list_clowders();
    let total_collections = collection_names.len() as u32;
    let mut c_strings: Vec<*mut c_char> = collection_names.iter().map(|name_str| CString::new(name_str.as_str()).unwrap().into_raw()).collect();
    if c_strings.is_empty() {
        unsafe {
            *names = std::ptr::null_mut();
            *count = 0;
        }
        return 0;
    }

    let ptr = c_strings.as_mut_ptr();
    std::mem::forget(c_strings);
    unsafe {
        *names = ptr;
        *count = total_collections;
    };
    0
}

/// Remove a collection and its data directory.
///
/// # Safety
/// `name` must be a valid, null-terminated C string.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_drop(name: *const c_char) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InvalidName as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    match engine.write().unwrap().drop_clowder(&name_str) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}

/// Get stats for a collection.
///
/// # Safety
/// `name` must be a valid, null-terminated C string. `stats` must be a valid, non-null pointer to writable memory.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_stats(name: *const c_char, stats: *mut NekoStats) -> c_int {
    if stats.is_null() {
        return Hairball::InternalError as c_int;
    }
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InvalidName as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    match engine.write().unwrap().get_stats(&name_str) {
        Ok(stats_value) => {
            unsafe {
                *stats = stats_value;
            }
            0
        }
        Err(err) => err as c_int,
    }
}

/// Free strings allocated by `neko_list_collections`.
///
/// # Safety
/// `strings` must have been allocated by `neko_list_collections`. `count` must match.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_free_strings(strings: *mut *mut c_char, count: u32) {
    if strings.is_null() || count == 0 {
        return;
    }
    unsafe {
        let pointers: Vec<*mut c_char> = Vec::from_raw_parts(strings, count as usize, count as usize);
        for pointer in pointers {
            if pointer.is_null() {
                continue;
            }
            let _ = CString::from_raw(pointer);
        }
    }
}

/// Insert a vector into a collection
///
/// # Safety
/// `name` and `id` must be valid, null-terminated C strings. `vector` must point to `len` valid f32 values. `metadata` may be null.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_insert(name: *const c_char, id: *const c_char, vector: *const f32, len: u32, metadata: *const c_char) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };
    let raw_id_string = unsafe { c_str_to_string(id) };
    let id_str = match raw_id_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    if vector.is_null() || len == 0 {
        return Hairball::DimTooSmall as c_int;
    }

    let raw_metadata_string = unsafe { c_str_to_string(metadata) };
    let vector_metadata: VectorMetadata = match raw_metadata_string {
        Some(string) if !string.is_empty() => VectorMetadata {
            id: id_str.clone(),
            created_at: 0,
            deleted: false,
            custom: string,
        },
        _ => VectorMetadata {
            id: id_str.clone(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        },
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    let vector_slice = unsafe { std::slice::from_raw_parts(vector, len as usize) };
    match engine.write().unwrap().insert_vector(&name_str, &id_str, vector_slice.to_vec(), &vector_metadata) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}

/// id must point to count valid C String pointers
/// vectors must point to a valid f32 value
/// dim must point to a valid u32 value
/// metadata must be valid C String pointers where it can nullble
#[repr(C)]
pub struct NekoInputVector {
    pub id: *const c_char,
    pub vector: *const f32,
    pub dim: u32,
    pub metadata: *const c_char,
}

/// Insert many vector into a collection in one FFI call
///
/// # Safety
/// name must be valid, as null will terminate C String.
/// Each `NekoInputVector.id` must be a valid
///     C string (null is rejected with `InternalError`). Each `NekoInputVector.vector` must
///     point to at least `dim` valid f32 values. Each `NekoInputVector.metadata` may be
///     null to indicate no metadata for that item.
/// count is total of how many vector to be inserted
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_insert_many(name: *const c_char, input_vectors: *const NekoInputVector, count: u32) -> c_int {
    if name.is_null() || input_vectors.is_null() {
        return Hairball::InternalError as c_int;
    }

    if count == 0 {
        return Hairball::InternalError as c_int;
    }

    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_string = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let raw_input_vectors = unsafe { std::slice::from_raw_parts(input_vectors, count as usize) };
    let mut processed_input_vectors = Vec::with_capacity(count as usize);

    for raw_input_vector in raw_input_vectors {
        let raw_id_string = unsafe { c_str_to_string(raw_input_vector.id) };
        let id_string = match raw_id_string {
            Some(string) => string,
            None => return Hairball::InternalError as c_int,
        };

        if raw_input_vector.vector.is_null() {
            return Hairball::InternalError as c_int;
        }

        let vector_owned = unsafe { std::slice::from_raw_parts(raw_input_vector.vector, raw_input_vector.dim as usize) }.to_vec();
        let raw_metadata_string = unsafe { c_str_to_string(raw_input_vector.metadata) };
        let vector_metadata: VectorMetadata = match raw_metadata_string {
            Some(string) if !string.is_empty() => VectorMetadata {
                id: id_string.clone(),
                created_at: 0,
                deleted: false,
                custom: string,
            },
            _ => VectorMetadata {
                id: id_string.clone(),
                created_at: 0,
                deleted: false,
                custom: String::new(),
            },
        };

        processed_input_vectors.push(InputVectorDto {
            id: id_string,
            vector: vector_owned,
            metadata: vector_metadata,
        });
    }

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    match engine.write().unwrap().insert_many_vector(&name_string, processed_input_vectors) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}

/// Upsert (insert-or-update) a vector into a collection.
///
/// # Safety
/// name and id must be valid, null-terminated C strings.
/// vector must point to len of valid f32 values.
/// metadata may be null
/// created may be null.
///     - If non-null, then it must point to writable memory and receives 1 if vector was newly created
///     - Otherwise, 0 if it was updated
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_upsert(name: *const c_char, id: *const c_char, vector: *const f32, len: u32, metadata: *const c_char, created: *mut u8) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let raw_id_string = unsafe { c_str_to_string(id) };
    let id_str = match raw_id_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    if vector.is_null() || len == 0 {
        return Hairball::DimTooSmall as c_int;
    }

    let raw_metadata_string = unsafe { c_str_to_string(metadata) };
    let vector_metadata: VectorMetadata = match raw_metadata_string {
        Some(string) if !string.is_empty() => serde_json::from_str::<VectorMetadata>(&string).unwrap_or(VectorMetadata {
            id: id_str.clone(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        }),
        _ => VectorMetadata {
            id: id_str.clone(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        },
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };
    let vector_slice = unsafe { std::slice::from_raw_parts(vector, len as usize) };

    let was_created = {
        let clowder = match engine.read().unwrap().clowders.get(&name_str) {
            Some(clowder) => clowder.clone(),
            None => return Hairball::NotFound as c_int,
        };
        let is_existed = clowder.vectors.lock().unwrap().contains_key(&id_str);
        match engine.write().unwrap().upsert_vector(&name_str, &id_str, vector_slice.to_vec(), &vector_metadata) {
            Ok(_) => !is_existed,
            Err(err) => return err as c_int,
        }
    };

    if !created.is_null() {
        unsafe {
            *created = was_created as u8;
        }
    }
    0
}

/// Retrieve a vector by ID from a collection.
///
/// # Safety
/// `name` and `id` must be valid, null-terminated C strings. `vector_out` must point to writable memory of at least `dim * sizeof(f32)` bytes.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_get(name: *const c_char, id: *const c_char, vector: *mut f32, dim: u32) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };
    let raw_id_string = unsafe { c_str_to_string(id) };
    let id_str = match raw_id_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    if vector.is_null() {
        return Hairball::InternalError as c_int;
    }

    match engine.write().unwrap().get_vector(&name_str, &id_str) {
        Ok(vec) => {
            if vec.len() != dim as usize {
                return Hairball::InternalError as c_int;
            }
            unsafe {
                std::ptr::copy_nonoverlapping(vec.as_ptr(), vector, dim as usize);
            }
            0
        }
        Err(err) => err as c_int,
    }
}

/// Search top-K nearest neighbors in a collection.
///
/// # Safety
/// `name` must be a valid null-terminated C string. `query` must point to `dim` valid f32 values.
/// `results` must point to writable memory. `filter` may be null (accepted but not evaluated).
/// Caller must free results via `neko_free_result`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_search(name: *const c_char, query: *const f32, dim: u32, top_k: u32, results: *mut NekoSearchResult) -> c_int {
    if results.is_null() || query.is_null() {
        return Hairball::InternalError as c_int;
    }
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };
    let query_slice = unsafe { std::slice::from_raw_parts(query, dim as usize) };
    let scored_vectors = match engine.read().unwrap().search(&name_str, query_slice, top_k as usize) {
        Ok(vector) => vector,
        Err(err) => return err as c_int,
    };

    let total = scored_vectors.len() as u32;
    if total == 0 {
        unsafe {
            (*results).total = 0;
            (*results).ids = std::ptr::null_mut();
            (*results).scores = std::ptr::null_mut();
        }
        return 0;
    }

    let mut ids: Vec<*mut c_char> = scored_vectors.iter().map(|sv| CString::new(sv.id.clone()).unwrap().into_raw()).collect();
    let mut scores: Vec<f32> = scored_vectors.iter().map(|sv| sv.score).collect();

    unsafe {
        (*results).total = total;
        (*results).ids = ids.as_mut_ptr();
        (*results).scores = scores.as_mut_ptr();
    }
    std::mem::forget(ids);
    std::mem::forget(scores);
    0
}

/// Free search results allocated by `neko_search`. Must be called exactly once per search.
///
/// # Safety
/// `results` must have been allocated by `neko_search`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_free_result(results: *mut NekoSearchResult) {
    if results.is_null() {
        return;
    }

    let results = unsafe { &mut *results };
    let total = results.total as usize;
    if !results.ids.is_null() && total > 0 {
        let ids = unsafe { Vec::from_raw_parts(results.ids, total, total) };
        for id in ids {
            if id.is_null() {
                continue;
            };
            let _ = unsafe { CString::from_raw(id) };
        }
    }

    if !results.scores.is_null() && total > 0 {
        let _ = unsafe { Vec::from_raw_parts(results.scores, total, total) };
    }
}

/// Delete a vector by ID from a collection.
///
/// # Safety
/// `name` and `id` must be valid, null-terminated C strings.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_delete(name: *const c_char, id: *const c_char) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let raw_id_string = unsafe { c_str_to_string(id) };
    let id_str = match raw_id_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    match engine.write().unwrap().delete_vector(&name_str, &id_str) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
}
/// Retrieve a vector + metadata by ID. Caller owns the returned
/// `NekoMetadata.metadata` pointer and must free it via `neko_free_metadata`.
///
/// # Safety
/// `name` and `id` must be valid null-terminated C strings. `vector_out` must
/// point to writable memory of at least `dim * sizeof(f32)` bytes.
/// `metadata_out` must point to a writable `NekoMetadata`.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_get_vector(name: *const c_char, id: *const c_char, vector_out: *mut f32, dim: u32, metadata_out: *mut NekoMetadata) -> c_int {
    let raw_name_string = unsafe { c_str_to_string(name) };
    let name_str = match raw_name_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let raw_id_string = unsafe { c_str_to_string(id) };
    let id_str = match raw_id_string {
        Some(string) => string,
        None => return Hairball::InternalError as c_int,
    };

    let engine = match ENGINE.get() {
        Some(engine) => engine,
        None => return Hairball::InternalError as c_int,
    };

    if vector_out.is_null() || metadata_out.is_null() {
        return Hairball::InternalError as c_int;
    };

    let clowder = match engine.read().unwrap().clowders.get(&name_str) {
        Some(clowder) => clowder.clone(),
        None => return Hairball::NotFound as c_int,
    };

    let vector_owned = {
        let vectors = clowder.vectors.lock().unwrap();
        match vectors.get(&id_str) {
            Some(vector) => vector.clone(),
            None => return Hairball::NotFound as c_int,
        }
    };

    if vector_owned.len() != dim as usize {
        return Hairball::InternalError as c_int;
    }

    let metadata_string = clowder.metadata.lock().unwrap().get(&id_str).cloned().unwrap_or_default();

    unsafe {
        std::ptr::copy_nonoverlapping(vector_owned.as_ptr(), vector_out, dim as usize);
        let c_string = std::ffi::CString::new(metadata_string).unwrap_or_default();
        (*metadata_out).metadata = c_string.into_raw();
    }
    0
}

/// Free a NekoMetadata previously returned by a neko_get_vector
///
/// # Safety
/// meta must point to the NekoMetadata whose metadata field was produced
/// by neko_get_vector, or be a null (in which case this is a no-op)
#[unsafe(no_mangle)]
pub unsafe extern "C" fn neko_free_metadata(meta: *mut NekoMetadata) {
    if meta.is_null() {
        return;
    }

    unsafe {
        let m = &*meta;
        if !m.metadata.is_null() {
            drop(std::ffi::CString::from_raw(m.metadata));
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::ffi::CString;
    use std::fs;
    use std::path::PathBuf;

    fn ffi_temp_dir() -> PathBuf {
        std::env::temp_dir().join("neko_test_ffi")
    }

    #[test]
    fn given_valid_data_dir_then_init_succeeds() {
        let dir = ffi_temp_dir();
        fs::create_dir_all(&dir).unwrap();
        let c_dir = CString::new(dir.to_string_lossy().as_ref()).unwrap();
        let result = unsafe { neko_init(c_dir.as_ptr()) };
        assert_eq!(result, 0, "init should succeed with valid path");
    }

    #[test]
    fn given_null_data_dir_then_init_returns_internal_error() {
        let result = unsafe { neko_init(std::ptr::null()) };
        assert_eq!(result, Hairball::InternalError as i32);
    }

    fn ffi_init() {
        let dir = ffi_temp_dir();
        fs::create_dir_all(&dir).unwrap();
        let c_dir = CString::new(dir.to_string_lossy().as_ref()).unwrap();
        let _ = unsafe { neko_init(c_dir.as_ptr()) };
    }

    fn ffi_cleanup(name: &str) {
        let c_name = CString::new(name).unwrap();
        let _ = unsafe { neko_drop(c_name.as_ptr()) };
    }

    #[test]
    fn given_valid_params_then_create_succeeds() {
        ffi_init();
        ffi_cleanup("ffi_test_create");
        let name = CString::new("ffi_test_create").unwrap();
        let result = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(result, 0, "create should succeed with valid params");
    }

    #[test]
    fn given_duplicate_name_then_create_returns_already_exists() {
        ffi_init();
        ffi_cleanup("ffi_test_dup");
        let name = CString::new("ffi_test_dup").unwrap();
        let first = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(first, 0);
        let second = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(second, Hairball::AlreadyExists as i32);
    }

    #[test]
    fn given_invalid_name_then_create_returns_invalid_name() {
        ffi_init();
        let name = CString::new("!!!bad name!!!").unwrap();
        let result = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(result, Hairball::InvalidName as i32);
    }

    #[test]
    fn given_list_after_create_then_returns_collection_name() {
        ffi_init();
        ffi_cleanup("ffi_test_list");
        let name = CString::new("ffi_test_list").unwrap();
        let _ = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };

        let mut c_names: *mut *mut c_char = std::ptr::null_mut();
        let mut count: u32 = 0;
        let result = unsafe { neko_list(&mut c_names, &mut count) };
        assert_eq!(result, 0);
        assert!(count >= 1, "list should include the newly created collection");
        let slice = unsafe { std::slice::from_raw_parts(c_names, count as usize) };
        let names: Vec<String> = slice.iter().map(|p| unsafe { CStr::from_ptr(*p) }.to_string_lossy().to_string()).collect();
        assert!(names.contains(&"ffi_test_list".to_string()), "list should contain 'ffi_test_list', got {:?}", names);
        unsafe { neko_free_strings(c_names, count) };
    }

    #[test]
    fn given_list_null_pointer_then_returns_internal_error() {
        let result = unsafe { neko_list(std::ptr::null_mut(), std::ptr::null_mut()) };
        assert_eq!(result, Hairball::InternalError as i32);
    }

    #[test]
    fn given_stats_existing_collection_then_returns_correct_config() {
        ffi_init();
        ffi_cleanup("ffi_test_stats");

        let name = CString::new("ffi_test_stats").unwrap();
        let _ = unsafe { neko_create(name.as_ptr(), 512, 2, std::ptr::null()) };

        let mut stats = NekoStats {
            vector_count: 0,
            dim: 0,
            metric: 0,
            storage_bytes: 0,
            index_type: 0,
        };
        let result = unsafe { neko_stats(name.as_ptr(), &mut stats) };
        assert_eq!(result, 0);
        assert_eq!(stats.dim, 512);
        assert_eq!(stats.metric, 2);
        assert_eq!(stats.vector_count, 0);
    }

    #[test]
    fn given_drop_existing_collection_then_succeeds() {
        ffi_init();
        ffi_cleanup("ffi_test_drop");

        let name = CString::new("ffi_test_drop").unwrap();
        let create_result = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(create_result, 0, "create should succeed after cleanup");

        let result = unsafe { neko_drop(name.as_ptr()) };
        assert_eq!(result, 0, "drop of existing collection should succeed");
    }

    #[test]
    fn given_drop_nonexistent_collection_then_returns_not_found() {
        ffi_init();
        ffi_cleanup("ffi_test_nonexistent");

        let name = CString::new("ffi_test_nonexistent").unwrap();
        let result = unsafe { neko_drop(name.as_ptr()) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_free_strings_with_zero_count_then_no_crash() {
        unsafe { neko_free_strings(std::ptr::null_mut(), 0) };
    }

    #[test]
    fn given_model_in_create_then_collection_persists_with_model() {
        ffi_init();
        ffi_cleanup("ffi_test_model");

        let name = CString::new("ffi_test_model").unwrap();
        let model = CString::new("all-MiniLM-L6-v2").unwrap();
        let result = unsafe { neko_create(name.as_ptr(), 384, 1, model.as_ptr()) };
        assert_eq!(result, 0);

        let mut stats = NekoStats {
            vector_count: 0,
            dim: 0,
            metric: 0,
            storage_bytes: 0,
            index_type: 0,
        };
        let _ = unsafe { neko_stats(name.as_ptr(), &mut stats) };
        assert_eq!(stats.dim, 384);
    }

    #[test]
    fn given_create_with_null_model_then_succeeds() {
        ffi_init();
        ffi_cleanup("ffi_test_null_model");

        let name = CString::new("ffi_test_null_model").unwrap();
        let result = unsafe { neko_create(name.as_ptr(), 384, 1, std::ptr::null()) };
        assert_eq!(result, 0);
    }

    #[test]
    fn given_valid_insert_and_get_via_ffi_then_round_trips() {
        ffi_init();
        ffi_cleanup("ffi_test_roundtrip");

        let collection = CString::new("ffi_test_roundtrip").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [7.0, 8.0, 9.0];

        let result = unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };
        assert_eq!(result, 0);

        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 3];
        let result = unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3) };
        assert_eq!(result, 0);
        let inv_norm = 1.0 / (7.0_f32 * 7.0 + 8.0 * 8.0 + 9.0 * 9.0).sqrt();
        assert_eq!(out, vec![7.0 * inv_norm, 8.0 * inv_norm, 9.0 * inv_norm]);
    }

    #[test]
    fn given_insert_wrong_dim_via_ffi_then_returns_dim_mismatch() {
        ffi_init();
        ffi_cleanup("ffi_test_dim");

        let collection = CString::new("ffi_test_dim").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 2] = [1.0, 2.0];

        unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };

        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 2, std::ptr::null()) };
        assert_eq!(result, Hairball::DimMismatch as i32);
    }

    #[test]
    fn given_get_nonexistent_via_ffi_then_returns_not_found() {
        ffi_init();
        ffi_cleanup("ffi_test_get_nf");

        let collection = CString::new("ffi_test_get_nf").unwrap();
        let doc_id = CString::new("no_such_doc").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };

        let mut out = vec![0.0_f32; 3];
        let result = unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_insert_nonexistent_collection_via_ffi_then_returns_not_found() {
        ffi_init();

        let collection = CString::new("no_such_collection_zzz").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];

        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null()) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_insert_null_vector_via_ffi_then_returns_dim_too_small() {
        ffi_init();
        ffi_cleanup("ffi_test_nullvec");

        let collection = CString::new("ffi_test_nullvec").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };

        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), std::ptr::null(), 3, std::ptr::null()) };
        assert_eq!(result, Hairball::DimTooSmall as i32);
    }

    #[test]
    fn given_insert_with_metadata_via_ffi_then_round_trips() {
        ffi_init();
        ffi_cleanup("ffi_test_meta");

        let collection = CString::new("ffi_test_meta").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 2] = [9.0, 10.0];
        let metadata_json = CString::new(r#"{"key":"value","score":42}"#).unwrap();

        unsafe { neko_create(collection.as_ptr(), 2, 1, std::ptr::null()) };

        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 2, metadata_json.as_ptr()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 2];
        let result = unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 2) };
        assert_eq!(result, 0);
        let inv_norm = 1.0 / (9.0_f32 * 9.0 + 10.0 * 10.0).sqrt();
        assert_eq!(out, vec![9.0 * inv_norm, 10.0 * inv_norm]);
    }

    #[test]
    fn given_valid_upsert_and_get_via_ffi_then_round_trips() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_roundtrip");

        let collection = CString::new("ffi_test_upsert_roundtrip").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [7.0, 8.0, 9.0];

        let result = unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };
        assert_eq!(result, 0);

        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null(), std::ptr::null_mut()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 3];
        let result = unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3) };
        assert_eq!(result, 0);
        let inv_norm = 1.0 / (7.0_f32 * 7.0 + 8.0 * 8.0 + 9.0 * 9.0).sqrt();
        assert_eq!(out, vec![7.0 * inv_norm, 8.0 * inv_norm, 9.0 * inv_norm]);
    }

    #[test]
    fn given_upsert_existing_via_ffi_then_value_is_replaced() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_replace");

        let collection = CString::new("ffi_test_upsert_replace").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let first: [f32; 3] = [1.0, 0.0, 0.0];
        let second: [f32; 3] = [0.0, 1.0, 0.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), first.as_ptr(), 3, std::ptr::null()) };
        unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), second.as_ptr(), 3, std::ptr::null(), std::ptr::null_mut()) };

        let mut out = vec![0.0_f32; 3];
        unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3) };
        assert_eq!(out, vec![0.0_f32, 1.0, 0.0]);
    }

    #[test]
    fn given_upsert_wrong_dim_via_ffi_then_returns_dim_mismatch() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_dim");

        let collection = CString::new("ffi_test_upsert_dim").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 2] = [1.0, 2.0];

        unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };

        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 2, std::ptr::null(), std::ptr::null_mut()) };
        assert_eq!(result, Hairball::DimMismatch as i32);
    }

    #[test]
    fn given_upsert_nonexistent_collection_via_ffi_then_returns_not_found() {
        ffi_init();

        let collection = CString::new("no_such_collection_zzz_upsert").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];

        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null(), std::ptr::null_mut()) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_upsert_null_vector_via_ffi_then_returns_dim_too_small() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_nullvec");

        let collection = CString::new("ffi_test_upsert_nullvec").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 1, std::ptr::null()) };

        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), std::ptr::null(), 3, std::ptr::null(), std::ptr::null_mut()) };
        assert_eq!(result, Hairball::DimTooSmall as i32);
    }

    #[test]
    fn given_empty_clowder_then_search_returns_zero_results() {
        ffi_init();
        ffi_cleanup("ffi_test_search_empty");
        let collection = CString::new("ffi_test_search_empty").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let query: [f32; 3] = [1.0, 0.0, 0.0];
        let mut results = NekoSearchResult {
            total: 0,
            ids: std::ptr::null_mut(),
            scores: std::ptr::null_mut(),
        };
        let code = unsafe { neko_search(collection.as_ptr(), query.as_ptr(), 3, 5, &mut results) };
        assert_eq!(code, 0);
        assert_eq!(results.total, 0);
        assert!(results.ids.is_null());
        assert!(results.scores.is_null());
        unsafe { neko_free_result(&mut results) };
    }

    #[test]
    fn given_l2_vectors_then_search_returns_nearest_by_score() {
        ffi_init();
        ffi_cleanup("ffi_test_search_l2");
        let collection = CString::new("ffi_test_search_l2").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let doc_a = CString::new("a").unwrap();
        let doc_b = CString::new("b").unwrap();
        let doc_c = CString::new("c").unwrap();
        let vec_a: [f32; 3] = [10.0, 0.0, 0.0];
        let vec_b: [f32; 3] = [2.0, 0.0, 0.0];
        let vec_c: [f32; 3] = [5.0, 0.0, 0.0];
        unsafe {
            neko_insert(collection.as_ptr(), doc_a.as_ptr(), vec_a.as_ptr(), 3, std::ptr::null());
            neko_insert(collection.as_ptr(), doc_b.as_ptr(), vec_b.as_ptr(), 3, std::ptr::null());
            neko_insert(collection.as_ptr(), doc_c.as_ptr(), vec_c.as_ptr(), 3, std::ptr::null());
        }

        let query: [f32; 3] = [1.0, 0.0, 0.0];
        let mut results = NekoSearchResult {
            total: 0,
            ids: std::ptr::null_mut(),
            scores: std::ptr::null_mut(),
        };
        let code = unsafe { neko_search(collection.as_ptr(), query.as_ptr(), 3, 2, &mut results) };
        assert_eq!(code, 0);
        assert_eq!(results.total, 2);

        let ids = unsafe { std::slice::from_raw_parts(results.ids, 2) };
        let scores = unsafe { std::slice::from_raw_parts(results.scores, 2) };
        let id0 = unsafe { CStr::from_ptr(ids[0]) }.to_str().unwrap();
        let id1 = unsafe { CStr::from_ptr(ids[1]) }.to_str().unwrap();
        assert_eq!(id0, "b");
        assert_eq!(id1, "c");
        assert!(scores[0] < scores[1]);

        unsafe { neko_free_result(&mut results) };
    }

    #[test]
    fn given_nonexistent_clowder_then_search_returns_not_found() {
        ffi_init();
        let collection = CString::new("no_such_search_collection").unwrap();
        let query: [f32; 3] = [1.0, 0.0, 0.0];
        let mut results = NekoSearchResult {
            total: 0,
            ids: std::ptr::null_mut(),
            scores: std::ptr::null_mut(),
        };
        let code = unsafe { neko_search(collection.as_ptr(), query.as_ptr(), 3, 5, &mut results) };
        assert_eq!(code, Hairball::NotFound as i32);
    }

    #[test]
    fn given_null_query_ptr_then_search_returns_internal_error() {
        ffi_init();
        ffi_cleanup("ffi_test_search_nullq");
        let collection = CString::new("ffi_test_search_nullq").unwrap();
        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let mut results = NekoSearchResult {
            total: 0,
            ids: std::ptr::null_mut(),
            scores: std::ptr::null_mut(),
        };
        let code = unsafe { neko_search(collection.as_ptr(), std::ptr::null(), 3, 5, &mut results) };
        assert_eq!(code, Hairball::InternalError as i32);
    }

    #[test]
    fn given_free_result_null_then_no_crash() {
        unsafe { neko_free_result(std::ptr::null_mut()) };
    }

    #[test]
    fn given_delete_existing_via_ffi_then_get_returns_not_found() {
        ffi_init();
        ffi_cleanup("ffi_test_delete");

        let collection = CString::new("ffi_test_delete").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };
        unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null()) };

        let result = unsafe { neko_delete(collection.as_ptr(), doc_id.as_ptr()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 3];
        let get_result = unsafe { neko_get(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3) };
        assert_eq!(get_result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_delete_nonexistent_id_via_ffi_then_returns_not_found() {
        ffi_init();
        ffi_cleanup("ffi_test_delete_nf");

        let collection = CString::new("ffi_test_delete_nf").unwrap();
        let doc_id = CString::new("ghost").unwrap();

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let result = unsafe { neko_delete(collection.as_ptr(), doc_id.as_ptr()) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_delete_nonexistent_clowder_via_ffi_then_returns_not_found() {
        ffi_init();

        let collection = CString::new("no_such_clowder_delete").unwrap();
        let doc_id = CString::new("doc1").unwrap();

        let result = unsafe { neko_delete(collection.as_ptr(), doc_id.as_ptr()) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_delete_null_name_via_ffi_then_returns_internal_error() {
        ffi_init();

        let doc_id = CString::new("doc1").unwrap();
        let result = unsafe { neko_delete(std::ptr::null(), doc_id.as_ptr()) };
        assert_eq!(result, Hairball::InternalError as i32);
    }

    #[test]
    fn given_insert_with_metadata_via_ffi_then_get_vector_round_trips_metadata() {
        ffi_init();
        ffi_cleanup("ffi_test_get_vector_meta");

        let collection = CString::new("ffi_test_get_vector_meta").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];
        let metadata_json = CString::new(r#"{"author":"alice"}"#).unwrap();

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };
        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, metadata_json.as_ptr()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 3];
        let mut meta: NekoMetadata = NekoMetadata { metadata: std::ptr::null_mut() };
        let result = unsafe { neko_get_vector(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3, &mut meta) };
        assert_eq!(result, 0);
        assert!(!meta.metadata.is_null(), "expected metadata pointer to be non-null after get");

        let metadata_string = unsafe { CStr::from_ptr(meta.metadata) }.to_str().unwrap();
        assert_eq!(metadata_string, r#"{"author":"alice"}"#);

        unsafe { neko_free_metadata(&mut meta) };
    }

    #[test]
    fn given_insert_without_metadata_via_ffi_then_get_vector_returns_null_metadata_pointer() {
        ffi_init();
        ffi_cleanup("ffi_test_get_vector_no_meta");

        let collection = CString::new("ffi_test_get_vector_no_meta").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [4.0, 5.0, 6.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };
        let result = unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null()) };
        assert_eq!(result, 0);

        let mut out = vec![0.0_f32; 3];
        let mut meta: NekoMetadata = NekoMetadata { metadata: std::ptr::null_mut() };
        let result = unsafe { neko_get_vector(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3, &mut meta) };
        assert_eq!(result, 0);
        // No metadata was attached, so the engine writes an empty CString
        // (never null) — what matters is the CStr content is empty.
        let metadata_string = unsafe { CStr::from_ptr(meta.metadata) }.to_str().unwrap();
        assert_eq!(metadata_string, "");

        unsafe { neko_free_metadata(&mut meta) };
    }

    #[test]
    fn given_get_vector_with_nonexistent_id_via_ffi_then_returns_not_found() {
        ffi_init();
        ffi_cleanup("ffi_test_get_vector_nf_id");

        let collection = CString::new("ffi_test_get_vector_nf_id").unwrap();
        let doc_id = CString::new("ghost").unwrap();

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let mut out = vec![0.0_f32; 3];
        let mut meta: NekoMetadata = NekoMetadata { metadata: std::ptr::null_mut() };
        let result = unsafe { neko_get_vector(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3, &mut meta) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_get_vector_with_nonexistent_clowder_via_ffi_then_returns_not_found() {
        ffi_init();

        let collection = CString::new("no_such_clowder_get_vector").unwrap();
        let doc_id = CString::new("doc1").unwrap();

        let mut out = vec![0.0_f32; 3];
        let mut meta: NekoMetadata = NekoMetadata { metadata: std::ptr::null_mut() };
        let result = unsafe { neko_get_vector(collection.as_ptr(), doc_id.as_ptr(), out.as_mut_ptr(), 3, &mut meta) };
        assert_eq!(result, Hairball::NotFound as i32);
    }

    #[test]
    fn given_free_metadata_null_then_no_crash() {
        unsafe { neko_free_metadata(std::ptr::null_mut()) };
    }

    #[test]
    fn given_upsert_new_via_ffi_then_created_flag_is_one() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_created");

        let collection = CString::new("ffi_test_upsert_created").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let mut created: u8 = 0;
        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null(), &mut created) };
        assert_eq!(result, 0);
        assert_eq!(created, 1, "first upsert of a new vector must report created=1");
    }

    #[test]
    fn given_upsert_existing_via_ffi_then_created_flag_is_zero() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_created_zero");

        let collection = CString::new("ffi_test_upsert_created_zero").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let first: [f32; 3] = [1.0, 0.0, 0.0];
        let second: [f32; 3] = [0.0, 1.0, 0.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };
        unsafe { neko_insert(collection.as_ptr(), doc_id.as_ptr(), first.as_ptr(), 3, std::ptr::null()) };

        let mut created: u8 = 99;
        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), second.as_ptr(), 3, std::ptr::null(), &mut created) };
        assert_eq!(result, 0);
        assert_eq!(created, 0, "upsert of an existing vector must report created=0");
    }

    #[test]
    fn given_upsert_via_ffi_with_null_created_pointer_then_no_crash() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_null_created");

        let collection = CString::new("ffi_test_upsert_null_created").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [4.0, 5.0, 6.0];

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, std::ptr::null(), std::ptr::null_mut()) };
        assert_eq!(result, 0, "upsert must accept a null created pointer and still succeed");
    }

    #[test]
    fn given_upsert_via_ffi_with_metadata_then_created_flag_still_set() {
        ffi_init();
        ffi_cleanup("ffi_test_upsert_meta_created");

        let collection = CString::new("ffi_test_upsert_meta_created").unwrap();
        let doc_id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [7.0, 8.0, 9.0];
        let metadata = CString::new(r#"{"author":"alice"}"#).unwrap();

        unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) };

        let mut created: u8 = 0;
        let result = unsafe { neko_upsert(collection.as_ptr(), doc_id.as_ptr(), vector.as_ptr(), 3, metadata.as_ptr(), &mut created) };
        assert_eq!(result, 0);
        assert_eq!(created, 1, "metadata must not affect the created flag");
    }

    #[test]
    fn given_valid_batch_via_ffi_then_all_vectors_retrievable() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_basic");
        let collection = CString::new("ffi_test_insert_many_basic").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) }, 0);

        let id_one = CString::new("doc1").unwrap();
        let id_two = CString::new("doc2").unwrap();
        let id_three = CString::new("doc3").unwrap();
        let vector_one: [f32; 3] = [1.0, 2.0, 3.0];
        let vector_two: [f32; 3] = [4.0, 5.0, 6.0];
        let vector_three: [f32; 3] = [7.0, 8.0, 9.0];
        let items = vec![
            NekoInputVector {
                id: id_one.as_ptr(),
                vector: vector_one.as_ptr(),
                dim: 3,
                metadata: std::ptr::null(),
            },
            NekoInputVector {
                id: id_two.as_ptr(),
                vector: vector_two.as_ptr(),
                dim: 3,
                metadata: std::ptr::null(),
            },
            NekoInputVector {
                id: id_three.as_ptr(),
                vector: vector_three.as_ptr(),
                dim: 3,
                metadata: std::ptr::null(),
            },
        ];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), items.len() as u32) };
        assert_eq!(code, 0, "batch insert of three valid vectors must succeed");

        let mut retrieved_two: [f32; 3] = [0.0; 3];
        let get_code = unsafe { neko_get(collection.as_ptr(), id_two.as_ptr(), retrieved_two.as_mut_ptr(), 3) };
        assert_eq!(get_code, 0);
        assert_eq!(retrieved_two, [4.0, 5.0, 6.0], "l2 metric does not normalise; raw values must round-trip");
    }

    #[test]
    fn given_batch_with_wrong_dim_via_ffi_then_returns_dim_mismatch() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_dim");
        let collection = CString::new("ffi_test_insert_many_dim").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) }, 0);

        let id = CString::new("doc1").unwrap();
        let wrong_dim_vector: [f32; 2] = [1.0, 2.0];
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: wrong_dim_vector.as_ptr(),
            dim: 2,
            metadata: std::ptr::null(),
        }];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), 1) };
        assert_eq!(code, Hairball::DimMismatch as c_int);
    }

    #[test]
    fn given_batch_with_nonexistent_clowder_via_ffi_then_returns_not_found() {
        ffi_init();

        let collection = CString::new("no_such_clowder_insert_many").unwrap();
        let id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: vector.as_ptr(),
            dim: 3,
            metadata: std::ptr::null(),
        }];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), 1) };
        assert_eq!(code, Hairball::NotFound as c_int);
    }

    #[test]
    fn given_batch_with_count_zero_via_ffi_then_returns_internal_error() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_count_zero");
        let collection = CString::new("ffi_test_insert_many_count_zero").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) }, 0);

        let code = unsafe { neko_insert_many(collection.as_ptr(), std::ptr::null(), 0) };
        assert_eq!(code, Hairball::InternalError as c_int);
    }

    #[test]
    fn given_batch_with_null_name_via_ffi_then_returns_internal_error() {
        ffi_init();
        let id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: vector.as_ptr(),
            dim: 3,
            metadata: std::ptr::null(),
        }];

        let code = unsafe { neko_insert_many(std::ptr::null(), items.as_ptr(), 1) };
        assert_eq!(code, Hairball::InternalError as c_int);
    }

    #[test]
    fn given_batch_with_null_input_vectors_via_ffi_then_returns_internal_error() {
        ffi_init();
        let collection = CString::new("ffi_test_insert_many_null_items").unwrap();

        let code = unsafe { neko_insert_many(collection.as_ptr(), std::ptr::null(), 5) };
        assert_eq!(code, Hairball::InternalError as c_int);
    }

    #[test]
    fn given_batch_with_null_item_vector_via_ffi_then_returns_internal_error() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_null_vec");
        let collection = CString::new("ffi_test_insert_many_null_vec").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) }, 0);

        let id = CString::new("doc1").unwrap();
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: std::ptr::null(),
            dim: 3,
            metadata: std::ptr::null(),
        }];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), 1) };
        assert_eq!(code, Hairball::InternalError as c_int);
    }

    #[test]
    fn given_batch_with_metadata_via_ffi_then_metadata_persisted() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_meta");
        let collection = CString::new("ffi_test_insert_many_meta").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 3, 0, std::ptr::null()) }, 0);

        let id = CString::new("doc1").unwrap();
        let vector: [f32; 3] = [1.0, 2.0, 3.0];
        let metadata = CString::new(r#"{"author":"alice"}"#).unwrap();
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: vector.as_ptr(),
            dim: 3,
            metadata: metadata.as_ptr(),
        }];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), 1) };
        assert_eq!(code, 0);

        let mut retrieved_vector: [f32; 3] = [0.0; 3];
        let mut retrieved_metadata = NekoMetadata { metadata: std::ptr::null_mut() };
        let get_code = unsafe { neko_get_vector(collection.as_ptr(), id.as_ptr(), retrieved_vector.as_mut_ptr(), 3, &mut retrieved_metadata) };
        assert_eq!(get_code, 0);
        assert!(!retrieved_metadata.metadata.is_null());
        let metadata_string = unsafe { CStr::from_ptr(retrieved_metadata.metadata) }.to_str().unwrap();
        assert_eq!(metadata_string, r#"{"author":"alice"}"#);
        unsafe { neko_free_metadata(&mut retrieved_metadata) };
    }

    #[test]
    fn given_batch_via_ffi_with_cosine_metric_then_vectors_normalised() {
        ffi_init();
        ffi_cleanup("ffi_test_insert_many_cosine");
        let collection = CString::new("ffi_test_insert_many_cosine").unwrap();
        assert_eq!(unsafe { neko_create(collection.as_ptr(), 2, 1, std::ptr::null()) }, 0);

        let id = CString::new("doc1").unwrap();
        let vector: [f32; 2] = [3.0, 4.0];
        let items = vec![NekoInputVector {
            id: id.as_ptr(),
            vector: vector.as_ptr(),
            dim: 2,
            metadata: std::ptr::null(),
        }];

        let code = unsafe { neko_insert_many(collection.as_ptr(), items.as_ptr(), 1) };
        assert_eq!(code, 0);

        let mut retrieved: [f32; 2] = [0.0; 2];
        let get_code = unsafe { neko_get(collection.as_ptr(), id.as_ptr(), retrieved.as_mut_ptr(), 2) };
        assert_eq!(get_code, 0);
        assert_eq!(retrieved, [0.6, 0.8], "cosine metric must normalise [3, 4] to [3/5, 4/5]");
    }
}
