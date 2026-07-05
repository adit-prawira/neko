use std::collections::HashMap;
use std::fs::File;
use std::io::Read;
use std::path::Path;

use crate::segment::resource::VectorMetadata;
use crate::shared::results::Result;

use super::resource::{OperationCode, WalEntry};

/*
 * WalReplayer has reponsiblities to:
 * --> Decodes a binaryu WAL file into WalEntry
 * --> Group buckets of entries by name
 * */
pub struct WalReplayer;

impl WalReplayer {
    pub fn group_by_collection(entries: &[WalEntry]) -> HashMap<String, Vec<&WalEntry>> {
        let mut map: HashMap<String, Vec<&WalEntry>> = HashMap::new();

        for entry in entries {
            map.entry(entry.collection.clone()).or_default().push(entry);
        }

        map
    }

    pub fn replay_file(path: &Path) -> Result<Vec<WalEntry>> {
        let mut file = File::open(path)?;
        let mut buffer = Vec::new();

        file.read_to_end(&mut buffer)?;

        let mut entries = Vec::new();
        let mut position = 0usize;

        while position < buffer.len() {
            if position + 1 > buffer.len() {
                break;
            };

            let operation = OperationCode::from_u8(buffer[position])?;
            position += 1;

            if position + 4 > buffer.len() {
                break;
            };
            let id_bytes_size = u32::from_le_bytes(buffer[position..position + 4].try_into().unwrap()) as usize;

            position += 4;

            if position + id_bytes_size > buffer.len() {
                break;
            };
            let full_id = String::from_utf8_lossy(&buffer[position..position + id_bytes_size]).to_string();

            position += id_bytes_size;

            let mut parts = full_id.splitn(2, ':');
            let collection = parts.next().unwrap_or("").to_string();
            let id = parts.next().unwrap_or("").to_string();

            if operation == OperationCode::Insert {
                if position + 4 > buffer.len() {
                    break;
                };
                let vector_bytes_size = u32::from_le_bytes(buffer[position..position + 4].try_into().unwrap()) as usize;
                position += 4;

                if position + vector_bytes_size > buffer.len() {
                    break;
                };
                let float_count = vector_bytes_size / 4;
                let mut vector = Vec::with_capacity(float_count);
                unsafe {
                    std::ptr::copy_nonoverlapping(buffer[position..position + vector_bytes_size].as_ptr(), vector.as_mut_ptr() as *mut u8, vector_bytes_size);
                    vector.set_len(float_count);
                }

                position += vector_bytes_size;

                if position + 4 > buffer.len() {
                    break;
                };
                let meta_bytes_size = u32::from_le_bytes(buffer[position..position + 4].try_into().unwrap()) as usize;
                position += 4;

                if position + meta_bytes_size > buffer.len() {
                    break;
                };
                let metadata: VectorMetadata = serde_json::from_slice(&buffer[position..position + meta_bytes_size])?;
                position += meta_bytes_size;
                entries.push(WalEntry {
                    operation_code: operation,
                    collection,
                    id,
                    vector,
                    metadata,
                });
            } else {
                entries.push(WalEntry {
                    operation_code: operation,
                    collection,
                    id,
                    vector: vec![],
                    metadata: VectorMetadata {
                        id: String::new(),
                        created_at: 0,
                        deleted: false,
                        custom: String::new(),
                    },
                });
            }
        }
        Ok(entries)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segment::resource::VectorMetadata;
    use crate::shared::hairball::Hairball;
    use crate::wal::resource::{OperationCode, WalEntry};
    use std::fs;
    use std::io::Write;

    fn make_insert_entry(collection: &str, id: &str, vector: &[f32]) -> WalEntry {
        WalEntry {
            operation_code: OperationCode::Insert,
            collection: collection.to_string(),
            id: id.to_string(),
            vector: vector.to_vec(),
            metadata: VectorMetadata {
                id: id.to_string(),
                created_at: 0,
                deleted: false,
                custom: String::new(),
            },
        }
    }

    fn make_delete_entry(collection: &str, id: &str) -> WalEntry {
        WalEntry {
            operation_code: OperationCode::Delete,
            collection: collection.to_string(),
            id: id.to_string(),
            vector: vec![],
            metadata: VectorMetadata {
                id: String::new(),
                created_at: 0,
                deleted: false,
                custom: String::new(),
            },
        }
    }

    fn write_wal_file(path: &std::path::Path, entries: &[WalEntry]) {
        let mut file = fs::File::create(path).unwrap();
        for entry in entries {
            file.write_all(&[entry.operation_code as u8]).unwrap();
            let full_id = format!("{}:{}", entry.collection, entry.id);
            let id_bytes = full_id.as_bytes();
            let id_bytes_size = id_bytes.len() as u32;
            file.write_all(&id_bytes_size.to_le_bytes()).unwrap();
            file.write_all(id_bytes).unwrap();

            if entry.operation_code == OperationCode::Insert {
                let vector_data: &[u8] = unsafe { std::slice::from_raw_parts(entry.vector.as_ptr() as *const u8, entry.vector.len() * 4) };
                let vector_bytes_size = (entry.vector.len() * 4) as u32;
                file.write_all(&vector_bytes_size.to_le_bytes()).unwrap();
                file.write_all(vector_data).unwrap();

                let meta_bytes = serde_json::to_vec(&entry.metadata).unwrap();
                let meta_bytes_size = meta_bytes.len() as u32;
                file.write_all(&meta_bytes_size.to_le_bytes()).unwrap();
                file.write_all(&meta_bytes).unwrap();
            }
        }
    }

    #[test]
    fn given_empty_entry_list_then_group_by_collection_returns_empty_map() {
        let entries: Vec<WalEntry> = vec![];
        let groups = WalReplayer::group_by_collection(&entries);
        assert!(groups.is_empty());
    }

    #[test]
    fn given_entries_from_multiple_collections_then_group_by_collection_buckets_by_name() {
        let entries = vec![
            make_insert_entry("docs", "doc1", &[1.0, 2.0]),
            make_insert_entry("docs", "doc2", &[3.0, 4.0]),
            make_insert_entry("images", "img1", &[5.0, 6.0]),
        ];
        let groups = WalReplayer::group_by_collection(&entries);
        assert_eq!(groups.len(), 2);
        assert_eq!(groups.get("docs").unwrap().len(), 2);
        assert_eq!(groups.get("images").unwrap().len(), 1);
    }

    #[test]
    fn given_entries_from_single_collection_then_group_by_collection_returns_one_bucket() {
        let entries = vec![make_insert_entry("docs", "doc1", &[1.0, 2.0]), make_insert_entry("docs", "doc2", &[3.0, 4.0])];
        let groups = WalReplayer::group_by_collection(&entries);
        assert_eq!(groups.len(), 1);
        assert_eq!(groups.get("docs").unwrap().len(), 2);
    }

    #[test]
    fn given_wal_file_with_insert_entry_then_replay_decodes_correct_fields() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_insert");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let entries = vec![make_insert_entry("docs", "d1", &[1.0, 2.0, 3.0])];
        write_wal_file(&wal_path, &entries);

        let replayed = WalReplayer::replay_file(&wal_path).unwrap();
        assert_eq!(replayed.len(), 1);
        let entry = &replayed[0];
        assert_eq!(entry.operation_code, OperationCode::Insert);
        assert_eq!(entry.collection, "docs");
        assert_eq!(entry.id, "d1");
        assert_eq!(entry.vector, vec![1.0, 2.0, 3.0]);
    }

    #[test]
    fn given_wal_file_with_delete_entry_then_replay_decodes_empty_vector() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_delete");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let entries = vec![make_delete_entry("docs", "d1")];
        write_wal_file(&wal_path, &entries);

        let replayed = WalReplayer::replay_file(&wal_path).unwrap();
        assert_eq!(replayed.len(), 1);
        let entry = &replayed[0];
        assert_eq!(entry.operation_code, OperationCode::Delete);
        assert_eq!(entry.collection, "docs");
        assert_eq!(entry.id, "d1");
        assert!(entry.vector.is_empty());
    }

    #[test]
    fn given_wal_file_with_adjacent_insert_and_delete_then_replay_decodes_both_in_order() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_mixed");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let entries = vec![make_insert_entry("docs", "d1", &[1.0]), make_delete_entry("docs", "d2")];
        write_wal_file(&wal_path, &entries);

        let replayed = WalReplayer::replay_file(&wal_path).unwrap();
        assert_eq!(replayed.len(), 2);
        assert_eq!(replayed[0].operation_code, OperationCode::Insert);
        assert_eq!(replayed[1].operation_code, OperationCode::Delete);
    }

    #[test]
    fn given_an_invalid_operation_byte_then_replay_file_returns_corrupted_segment_error() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_corrupt");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let mut file = fs::File::create(&wal_path).unwrap();
        file.write_all(&[0xFF]).unwrap();

        let result = WalReplayer::replay_file(&wal_path);
        assert!(matches!(result, Err(Hairball::CorruptedSegment)));
    }

    #[test]
    fn given_wal_file_with_opcode_only_no_id_bytes_then_replay_returns_empty_list() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_trunc1");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let mut file = fs::File::create(&wal_path).unwrap();
        file.write_all(&[0x01]).unwrap();

        let entries = WalReplayer::replay_file(&wal_path).unwrap();
        assert!(entries.is_empty());
    }

    #[test]
    fn given_wal_file_with_id_length_but_missing_id_body_then_replay_returns_empty_list() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_trunc2");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let mut file = fs::File::create(&wal_path).unwrap();
        file.write_all(&[0x01]).unwrap();
        file.write_all(&10u32.to_le_bytes()).unwrap();
        file.write_all("ab".as_bytes()).unwrap();

        let entries = WalReplayer::replay_file(&wal_path).unwrap();
        assert!(entries.is_empty());
    }

    #[test]
    fn given_insert_entry_with_full_id_but_no_vector_header_then_replay_returns_empty_list() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_trunc3");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let mut file = fs::File::create(&wal_path).unwrap();
        let full_id = b"docs:doc";
        file.write_all(&[0x01]).unwrap();
        file.write_all(&(full_id.len() as u32).to_le_bytes()).unwrap();
        file.write_all(full_id).unwrap();

        let entries = WalReplayer::replay_file(&wal_path).unwrap();
        assert!(entries.is_empty());
    }

    #[test]
    fn given_insert_entry_with_partial_vector_data_then_replay_returns_empty_list() {
        let directory = std::env::temp_dir().join("neko_test_wal_replayer_trunc4");
        let _ = fs::remove_dir_all(&directory);
        fs::create_dir_all(&directory).unwrap();
        let wal_path = directory.join("tail.log");

        let mut file = fs::File::create(&wal_path).unwrap();
        let full_id = b"docs:xxx";
        file.write_all(&[0x01]).unwrap();
        file.write_all(&(full_id.len() as u32).to_le_bytes()).unwrap();
        file.write_all(full_id).unwrap();
        file.write_all(&16u32.to_le_bytes()).unwrap();
        file.write_all(&[0xAAu8; 4]).unwrap();

        let entries = WalReplayer::replay_file(&wal_path).unwrap();
        assert!(entries.is_empty());
    }
}
