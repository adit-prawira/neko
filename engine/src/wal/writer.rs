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

    pub fn append_insert(&mut self, id: &str, vector: &[f32], metadata: &VectorMetadata) -> Result<()> {
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

        Ok(())
    }

    pub fn append_delete(&mut self, id: &str) -> Result<()> {
        let id_bytes = id.as_bytes();
        let id_bytes_size = id_bytes.len() as u32;
        self.file.write_all(&[OperationCode::Delete as u8])?;
        self.file.write_all(&id_bytes_size.to_le_bytes())?;
        self.file.write_all(id_bytes)?;
        self.file.flush()?;

        self.bytes_written += 1 + 4 + id_bytes_size as u64;
        Ok(())
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

    fn rotate(&mut self) -> Result<()> {
        self.file.flush()?;
        self.file.sync_all()?;

        let frozen_path = self.next_frozen_path();
        fs::rename(self.wal_directory.join("tail.log"), &frozen_path)?;

        self.file = OpenOptions::new().create(true).append(true).open(self.wal_directory.join("tail.log"))?;
        self.bytes_written = 0;
        self.compact_frozen(&frozen_path)
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
        fs::remove_file(frozen_path)?;
        Ok(())
    }
}
