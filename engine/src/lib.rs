use std::ffi::{CStr, CString, c_char, c_int};
use std::path::Path;

use self::engine::engine::{CreateClowderDto, ENGINE, Engine};
use self::segment::resource::VectorMetadata;
use self::shared::hairball::Hairball;
use self::shared::results::NekoStats;

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
    match engine.write().unwrap().insert_vector(&name_str, &id_str, vector_slice.to_vec(), &vector_metadata) {
        Ok(_) => 0,
        Err(err) => err as c_int,
    }
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
        assert_eq!(out, vec![7.0, 8.0, 9.0]);
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
        assert_eq!(out, vec![9.0, 10.0]);
    }
}
