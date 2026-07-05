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
                let floats: &[f32] = unsafe { std::slice::from_raw_parts(buffer[position..position + vector_bytes_size].as_ptr() as *const f32, vector_bytes_size / 4) };

                let vector = floats.to_vec();
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
