use std::fs::{self, File, OpenOptions};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

use crate::manifest::manager::ManifestManager;
use crate::segment::resource::VectorMetadata;
use crate::segment::writer::SegmentWriter;
use crate::shared::results::Result;

use super::replayer::WalReplayer;
use super::resource::OperationCode;

/*
 * WalWriter has responsibilities to:
 * -> Appends binary records to tail.log
 * -> Rotates when tail.log is too big,
 * -> compact frozen WALs into segments
 * */
pub struct WalWriter {
    file: File,
    bytes_written: u64,
    rotate_mb: u64,
    wal_directory: PathBuf,
    data_directory: PathBuf,
}

impl WalWriter {
    pub fn open(data_directory: &Path, rotate_mb: u64) -> Result<Self> {
        let wal_directory = data_directory.join("wal");
        fs::create_dir_all(&wal_directory)?;

        let file = OpenOptions::new().create(true).append(true).open(wal_directory.join("tail.log"))?;
        let bytes_written = file.metadata()?.len();

        Ok(Self {
            file,
            bytes_written,
            rotate_mb,
            wal_directory,
            data_directory: data_directory.to_path_buf(),
        })
    }

    pub fn append_insert(&mut self, id: &str, vector: &[f32], metadata: &VectorMetadata) -> Result<bool> {
        let meta_bytes = serde_json::to_vec(metadata)?;
        let meta_bytes_size = meta_bytes.len() as u32;

        let id_bytes = id.as_bytes();
        let id_bytes_size = id_bytes.len() as u32;

        let vector_bytes_size = (vector.len() * 4) as u32;

        self.file.write_all(&[OperationCode::Insert as u8])?;

        self.file.write_all(&id_bytes_size.to_le_bytes())?;
        self.file.write_all(id_bytes)?;

        self.file.write_all(&vector_bytes_size.to_le_bytes())?;
        let vector_data: &[u8] = unsafe { std::slice::from_raw_parts(vector.as_ptr() as *const u8, vector.len() * 4) };

        self.file.write_all(vector_data)?;
        self.file.write_all(&meta_bytes_size.to_le_bytes())?;
        self.file.write_all(&meta_bytes)?;

        self.file.flush()?;

        self.bytes_written += 1 + 4 + id_bytes_size as u64 + 4 + vector_bytes_size as u64 + 4 + meta_bytes_size as u64;

        let should_rotate = self.bytes_written >= self.rotate_mb * 1024 * 1024;
        if should_rotate {
            self.rotate()?;
        }

        // will return boolean indicating if a rotation happen
        Ok(should_rotate)
    }

    pub fn append_delete(&mut self, id: &str) -> Result<bool> {
        let id_bytes = id.as_bytes();
        let id_bytes_size = id_bytes.len() as u32;
        self.file.write_all(&[OperationCode::Delete as u8])?;
        self.file.write_all(&id_bytes_size.to_le_bytes())?;
        self.file.write_all(id_bytes)?;
        self.file.flush()?;

        self.bytes_written += 1 + 4 + id_bytes_size as u64;

        let should_rotate = self.should_rotate();
        if should_rotate {
            self.rotate()?;
        }

        // will return boolean indicating if a rotation happen
        Ok(should_rotate)
    }

    pub fn replay_all(data_directory: &Path) -> Result<Vec<(String, Vec<PathBuf>)>> {
        let mut entries = Vec::new();
        let tail_log_path = data_directory.join("wal").join("tail.log");
        if tail_log_path.exists() {
            entries.extend(WalReplayer::replay_file(&tail_log_path)?);
        }

        let wal_directory = data_directory.join("wal");
        if wal_directory.exists() {
            let mut frozen_tail_log_files: Vec<_> = fs::read_dir(&wal_directory)?
                .filter_map(|entry| entry.ok())
                .filter(|entry| {
                    let name = entry.file_name().to_string_lossy().into_owned();
                    name.starts_with("tail.") && name.ends_with(".log")
                })
                .collect();

            frozen_tail_log_files.sort_by_key(|entry| entry.file_name());

            for entry in frozen_tail_log_files {
                entries.extend(WalReplayer::replay_file(&entry.path())?);
            }
        }

        let collections = WalReplayer::group_by_collection(&entries);
        let mut manifests: Vec<(String, Vec<PathBuf>)> = Vec::new();

        for name in collections.keys() {
            let path = data_directory.join(name).join("manifest.json");
            if !path.exists() {
                continue;
            }
            if let Ok(segments) = ManifestManager::load_segments(&path) {
                manifests.push((name.clone(), segments));
            }
        }

        Ok(manifests)
    }

    pub fn compact(&mut self) -> Result<()> {
        let mut frozen_files: Vec<_> = fs::read_dir(&self.wal_directory)?
            .filter_map(|entry| entry.ok())
            .filter(|entry| {
                let name = entry.file_name().to_string_lossy().into_owned();
                name.starts_with("tail.") && name.ends_with(".log") && name != "tail.log"
            })
            .map(|entry| entry.path())
            .collect();
        frozen_files.sort();

        for frozen_path in &frozen_files {
            self.compact_frozen(&frozen_path)?;
            fs::remove_file(&frozen_path)?;
        }

        Ok(())
    }

    fn rotate(&mut self) -> Result<()> {
        self.file.flush()?;
        self.file.sync_all()?;

        let frozen_path = self.next_frozen_path();
        fs::rename(self.wal_directory.join("tail.log"), &frozen_path)?;

        self.file = OpenOptions::new().create(true).append(true).open(self.wal_directory.join("tail.log"))?;
        self.bytes_written = 0;
        Ok(())
    }

    fn next_frozen_path(&self) -> PathBuf {
        let current_total_tail_logs = fs::read_dir(&self.wal_directory)
            .map(|directory| {
                directory
                    .filter_map(|entry| entry.ok())
                    .filter(|entry| {
                        let name = entry.file_name().to_string_lossy().into_owned();

                        // check if file is tail.log
                        name.starts_with("tail.") && name.ends_with(".log")
                    })
                    .count()
            })
            .unwrap_or(0);
        self.wal_directory.join(format!("tail.{:03}.log", current_total_tail_logs + 1))
    }

    fn should_rotate(&self) -> bool {
        self.bytes_written >= self.rotate_mb * 1024 * 1024
    }

    fn compact_frozen(&self, frozen_path: &Path) -> Result<()> {
        let entries = WalReplayer::replay_file(frozen_path)?;
        let collections = WalReplayer::group_by_collection(&entries);
        for (name, grouped_entries) in &collections {
            let dim = entries
                .iter()
                .find(|entry| entry.collection == *name && entry.operation_code == OperationCode::Insert)
                .map(|entry| entry.vector.len() as u32)
                .unwrap_or(0);
            if dim == 0 {
                continue;
            };

            let segment_name = format!("seg_{:016x}", SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_nanos());

            let mut writer = SegmentWriter::new(&self.data_directory, &segment_name, dim)?;
            for entry in grouped_entries {
                if entry.operation_code == OperationCode::Insert {
                    writer.append(&entry.vector, &entry.metadata)?;
                };
            }

            writer.finish()?;
            ManifestManager::add_segment(&self.data_directory.join(name).join("manifest.json"), name, dim, 0, &segment_name)?;
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use crate::manifest::manager::ManifestManager;
    use crate::wal::replayer::WalReplayer;

    use super::*;

    fn temp_dir(name: &str) -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("neko_test_{}", name));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    #[test]
    fn given_a_new_wal_writer_then_tail_log_is_created() {
        let dir = temp_dir("wal_writer_open");
        let _wal = WalWriter::open(&dir, 64).unwrap();

        let tail_log = dir.join("wal").join("tail.log");
        assert!(tail_log.exists(), "tail.log should exist after WalWriter::open");
    }

    #[test]
    fn given_an_insert_then_entry_is_written_and_file_grows() {
        let dir = temp_dir("wal_writer_insert");
        let mut wal = WalWriter::open(&dir, 64).unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let size_before = wal.bytes_written;
        wal.append_insert("col:doc1", &vector, &metadata).unwrap();

        assert!(wal.bytes_written > size_before, "bytes_written should increase after an insert");
        let tail_log = dir.join("wal").join("tail.log");
        assert!(tail_log.metadata().unwrap().len() > 0, "tail.log should contain data after insert");
    }

    #[test]
    fn given_a_delete_then_bytes_written_increases() {
        let dir = temp_dir("wal_writer_delete");
        let mut wal = WalWriter::open(&dir, 64).unwrap();

        let size_before = wal.bytes_written;
        wal.append_delete("col:doc1").unwrap();

        assert!(wal.bytes_written > size_before, "bytes_written should increase after a delete");
    }

    #[test]
    fn given_insert_and_delete_then_replayed_entries_match() {
        let dir = temp_dir("wal_writer_roundtrip");
        let mut wal = WalWriter::open(&dir, 64).unwrap();

        let vector = vec![4.0_f32, 5.0_f32, 6.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 12345,
            deleted: false,
            custom: String::new(),
        };
        wal.append_insert("col:doc1", &vector, &metadata).unwrap();
        wal.append_delete("col:doc2").unwrap();

        let entries = WalReplayer::replay_file(&dir.join("wal").join("tail.log")).unwrap();
        assert_eq!(entries.len(), 2);

        assert_eq!(entries[0].operation_code, OperationCode::Insert);
        assert_eq!(entries[0].collection, "col");
        assert_eq!(entries[0].id, "doc1");
        assert_eq!(entries[0].vector, vec![4.0, 5.0, 6.0]);
        assert_eq!(entries[0].metadata.created_at, 12345);

        assert_eq!(entries[1].operation_code, OperationCode::Delete);
        assert_eq!(entries[1].collection, "col");
        assert_eq!(entries[1].id, "doc2");
    }

    #[test]
    fn given_insert_exceeds_rotate_threshold_then_append_insert_returns_true() {
        let dir = temp_dir("wal_writer_insert_rotates");
        let mut wal = WalWriter::open(&dir, 0).unwrap();
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let rotated = wal.append_insert("col:doc1", &[1.0_f32, 0.0_f32], &metadata).unwrap();

        assert!(rotated);
    }

    #[test]
    fn given_delete_exceeds_rotate_threshold_then_append_delete_returns_true() {
        let dir = temp_dir("wal_writer_delete_rotates");
        let mut wal = WalWriter::open(&dir, 0).unwrap();

        let rotated = wal.append_delete("col:doc1").unwrap();

        assert!(rotated);
    }

    #[test]
    fn given_frozen_wal_files_then_compact_creates_segments_and_removes_frozen_files() {
        let dir = temp_dir("wal_writer_compact");
        let collection_dir = dir.join("col");
        fs::create_dir_all(&collection_dir).unwrap();
        let mut wal = WalWriter::open(&dir, 0).unwrap();
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        wal.append_insert("col:doc1", &[1.0_f32, 0.0_f32], &metadata).unwrap();

        wal.compact().unwrap();

        let wal_dir = dir.join("wal");
        let frozen_files: Vec<_> = fs::read_dir(&wal_dir)
            .unwrap()
            .filter_map(|entry| entry.ok())
            .filter(|entry| {
                let name = entry.file_name().to_string_lossy().into_owned();
                name.starts_with("tail.") && name.ends_with(".log") && name != "tail.log"
            })
            .collect();
        assert!(frozen_files.is_empty(), "compact should remove frozen WAL files");

        let manifest = ManifestManager::load_manifest(&dir.join("col").join("manifest.json")).unwrap();
        assert_eq!(manifest.segments.len(), 1, "compact should create one segment");
    }
}
