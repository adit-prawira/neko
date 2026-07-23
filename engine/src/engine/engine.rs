use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, OnceLock, RwLock};

use crate::manifest::manager::ManifestManager;
use crate::manifest::resource::Manifest;
use crate::segment::resource::VectorMetadata;
use crate::shared::hairball::Hairball;
use crate::shared::results::{NekoStats, Result};
use crate::wal::replayer::WalReplayer;
use crate::wal::resource::WalEntry;
use crate::wal::writer::WalWriter;

use super::resource::Clowder;
use super::validator::EngineValidator;

pub static ENGINE: OnceLock<RwLock<Engine>> = OnceLock::new();

pub struct Engine {
    pub clowders: HashMap<String, Arc<Clowder>>,
    pub data_directory: PathBuf,
    pub wal: Option<WalWriter>,
}

pub struct CreateClowderDto<'a> {
    pub name: &'a str,
    pub dim: u32,
    pub metric: u8,
    pub model: Option<&'a str>,
}

/*
 * The engine will responsible to
 * --> Register new namespace for vectors (neko create)
 * --> Showing all registered collections & their config (neko list)
 * --> Destroying a collection and all of its data (neko drop)
 * --> Show vector DB statistics (neko stats)
 * */
impl Engine {
    pub fn init(data_directory: &Path) -> Result<()> {
        // return early if already initialised
        if ENGINE.get().is_some() {
            return Ok(());
        }

        // ~/.neko/collections
        let collection_directory = data_directory.join("collections");
        fs::create_dir_all(&collection_directory)?;

        let mut clowders = HashMap::new();
        if !collection_directory.exists() {
            return Ok(());
        }
        let entries = fs::read_dir(&collection_directory)?;
        for entry in entries {
            let entry = entry?;
            let is_directory = entry.file_type()?.is_dir();
            if !is_directory {
                continue;
            };

            let name = entry.file_name().to_string_lossy().to_string();
            let manifest_path = entry.path().join("manifest.json");
            let Ok(manifest) = ManifestManager::load_manifest(&manifest_path) else {
                continue;
            };

            if manifest.collection_name != name {
                continue;
            };

            clowders.insert(
                name.clone(),
                Arc::new(Clowder {
                    name,
                    dim: manifest.dim,
                    metric: manifest.metric,
                    model: manifest.model,
                    vectors: Mutex::new(HashMap::new()),
                }),
            );
        }
        let wal_entries = Self::replay_wal_entries(&collection_directory)?;
        for entry in &wal_entries {
            let Some(clowder) = clowders.get(&entry.collection) else {
                continue;
            };
            match entry.operation_code {
                crate::wal::resource::OperationCode::Insert => {
                    clowder.vectors.lock().unwrap().insert(entry.id.clone(), entry.vector.clone());
                }
                crate::wal::resource::OperationCode::Delete => {
                    clowder.vectors.lock().unwrap().remove(&entry.id);
                }
            }
        }
        let wal = WalWriter::open(&collection_directory, 64)
            .map_err(|err| {
                eprintln!("WAL: failed to open write-ahead log ({}); insert will not be persisted", err);
                err
            })
            .ok();
        let engine = Self {
            clowders,
            data_directory: data_directory.to_path_buf(),
            wal,
        };
        match ENGINE.set(RwLock::new(engine)) {
            Ok(_) | Err(_) => Ok(()),
        }
    }

    pub fn create_clowder<'a>(&mut self, payload: CreateClowderDto<'a>) -> Result<()> {
        EngineValidator::collection_name(payload.name)?;
        EngineValidator::dim(payload.dim)?;
        EngineValidator::metric(payload.metric)?;

        if self.clowders.contains_key(payload.name) {
            return Err(Hairball::AlreadyExists);
        }
        // access ~.neko/collections/<name>
        let collection_directory = self.data_directory.join("collections").join(payload.name);
        fs::create_dir_all(&collection_directory)?;

        let manifest = Manifest {
            version: 1,
            collection_name: payload.name.to_string(),
            dim: payload.dim,
            metric: payload.metric,
            model: payload.model.map(|model| model.to_string()),
            segments: Vec::new(),
        };
        let manifest_path = collection_directory.join("manifest.json");
        ManifestManager::save_manifest(&manifest_path, &manifest)?;

        self.clowders.insert(
            payload.name.to_string(),
            Arc::new(Clowder {
                name: payload.name.to_string(),
                dim: payload.dim,
                metric: payload.metric,
                model: payload.model.map(|model| model.to_string()),
                vectors: Mutex::new(HashMap::new()),
            }),
        );
        Ok(())
    }

    pub fn list_clowders(&self) -> Vec<String> {
        let mut names: Vec<String> = self.clowders.keys().cloned().collect();
        names.sort();
        names
    }

    pub fn drop_clowder(&mut self, name: &str) -> Result<()> {
        let is_exist = self.clowders.contains_key(name);
        if !is_exist {
            return Err(Hairball::NotFound);
        }

        let collection_directory = self.data_directory.join("collections").join(name);
        if collection_directory.exists() {
            fs::remove_dir_all(collection_directory)?;
        }

        self.clowders.remove(name);
        Ok(())
    }

    pub fn get_stats(&self, name: &str) -> Result<NekoStats> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        Ok(NekoStats {
            vector_count: 0,
            dim: clowder.dim,
            metric: clowder.metric,
            storage_bytes: 0,
            index_type: 0,
        })
    }

    pub fn insert_vector(&mut self, name: &str, id: &str, vector: Vec<f32>, metadata: &VectorMetadata) -> Result<()> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        if vector.len() != clowder.dim as usize {
            return Err(Hairball::DimMismatch);
        }
        let wal_id = format!("{}:{}", name, id);
        if let Some(ref mut wal) = self.wal {
            wal.append_insert(&wal_id, &vector, metadata)?;
        }

        clowder.vectors.lock().unwrap().insert(id.to_string(), vector);
        Ok(())
    }

    pub fn get_vector(&self, name: &str, id: &str) -> Result<Vec<f32>> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        let vectors = clowder.vectors.lock().unwrap();
        vectors.get(id).cloned().ok_or(Hairball::NotFound)
    }

    pub fn replay_wal_entries(collection_directory: &Path) -> Result<Vec<WalEntry>> {
        let mut entries = Vec::new();

        let wal_directory = collection_directory.join("wal");
        if !wal_directory.exists() {
            return Ok(Vec::new());
        }
        let mut tail_log_entries: Vec<_> = fs::read_dir(&wal_directory)?
            .filter_map(|entry| entry.ok())
            .filter(|entry| {
                let name = entry.file_name().to_string_lossy().into_owned();
                name.starts_with("tail.") && name.ends_with(".log") && name != "tail.log"
            })
            .collect();
        tail_log_entries.sort_by_key(|entry| entry.file_name());
        for entry in tail_log_entries {
            entries.extend(WalReplayer::replay_file(&entry.path())?);
        }

        let tail_log_path = collection_directory.join("wal").join("tail.log");
        if tail_log_path.exists() {
            entries.extend(WalReplayer::replay_file(&tail_log_path)?);
        }
        Ok(entries)
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;
    use std::fs;

    use crate::manifest::manager::ManifestManager;
    use crate::manifest::resource::Manifest;
    use crate::segment::resource::VectorMetadata;

    use super::*;

    fn new_engine(temp_dir: &std::path::Path) -> Engine {
        let data_dir = temp_dir.to_path_buf();
        fs::create_dir_all(&data_dir).unwrap();
        Engine {
            clowders: HashMap::new(),
            data_directory: data_dir,
            wal: None,
        }
    }

    fn temp_dir(name: &str) -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("neko_test_{}", name));
        let _ = fs::remove_dir_all(&dir);
        dir
    }

    #[test]
    fn given_valid_clowder_dto_then_create_adds_to_registry_and_persists_to_disk() {
        let dir = temp_dir("engine_create");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 384,
                metric: 1,
                model: None,
            })
            .unwrap();

        assert!(engine.clowders.contains_key("docs"));
        assert_eq!(engine.clowders["docs"].dim, 384);
        assert_eq!(engine.clowders["docs"].metric, 1);

        let manifest_path = dir.join("collections").join("docs").join("manifest.json");
        assert!(manifest_path.exists(), "manifest.json should exist on disk");
        let manifest: Manifest = ManifestManager::load_manifest(&manifest_path).unwrap();
        assert_eq!(manifest.collection_name, "docs");
        assert_eq!(manifest.dim, 384);
        assert_eq!(manifest.metric, 1);
    }

    #[test]
    fn given_create_then_list_returns_clowder_names_sorted() {
        let dir = temp_dir("engine_list");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "zebra",
                dim: 128,
                metric: 0,
                model: None,
            })
            .unwrap();
        engine
            .create_clowder(CreateClowderDto {
                name: "alpha",
                dim: 256,
                metric: 2,
                model: None,
            })
            .unwrap();

        let names = engine.list_clowders();
        assert_eq!(names, vec!["alpha", "zebra"]);
    }

    #[test]
    fn given_duplicate_name_then_create_returns_already_exists() {
        let dir = temp_dir("engine_duplicate");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 384,
                metric: 1,
                model: None,
            })
            .unwrap();
        let result = engine.create_clowder(CreateClowderDto {
            name: "docs",
            dim: 512,
            metric: 0,
            model: None,
        });

        assert_eq!(result.unwrap_err(), Hairball::AlreadyExists);
        assert_eq!(engine.clowders.len(), 1);
    }

    #[test]
    fn given_invalid_name_then_create_returns_hairball_error() {
        let dir = temp_dir("engine_invalid_name");
        let mut engine = new_engine(&dir);

        let result = engine.create_clowder(CreateClowderDto {
            name: "",
            dim: 128,
            metric: 0,
            model: None,
        });
        assert_eq!(result.unwrap_err(), Hairball::InvalidName);

        let result = engine.create_clowder(CreateClowderDto {
            name: "-bad",
            dim: 128,
            metric: 0,
            model: None,
        });
        assert_eq!(result.unwrap_err(), Hairball::InvalidName);
    }

    #[test]
    fn given_dim_too_large_then_create_returns_hairball_error() {
        let dir = temp_dir("engine_dim_large");
        let mut engine = new_engine(&dir);

        let result = engine.create_clowder(CreateClowderDto {
            name: "docs",
            dim: 4097,
            metric: 0,
            model: None,
        });
        assert_eq!(result.unwrap_err(), Hairball::DimTooLarge);
        assert!(engine.clowders.is_empty());
    }

    #[test]
    fn given_dim_zero_then_create_returns_hairball_error() {
        let dir = temp_dir("engine_dim_zero");
        let mut engine = new_engine(&dir);

        let result = engine.create_clowder(CreateClowderDto {
            name: "docs",
            dim: 0,
            metric: 0,
            model: None,
        });
        assert_eq!(result.unwrap_err(), Hairball::DimTooSmall);
    }

    #[test]
    fn given_invalid_metric_then_create_returns_hairball_error() {
        let dir = temp_dir("engine_invalid_metric");
        let mut engine = new_engine(&dir);

        let result = engine.create_clowder(CreateClowderDto {
            name: "docs",
            dim: 128,
            metric: 3,
            model: None,
        });
        assert_eq!(result.unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_drop_existing_clowder_then_removes_from_registry_and_disk() {
        let dir = temp_dir("engine_drop");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 384,
                metric: 1,
                model: None,
            })
            .unwrap();
        let coll_dir = dir.join("collections").join("docs");
        assert!(coll_dir.exists());

        engine.drop_clowder("docs").unwrap();
        assert!(!engine.clowders.contains_key("docs"));
        assert!(!coll_dir.exists(), "collection directory should be removed");
    }

    #[test]
    fn given_drop_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_drop_nonexistent");
        let mut engine = new_engine(&dir);

        let result = engine.drop_clowder("nonexistent");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_get_stats_existing_clowder_then_returns_correct_config() {
        let dir = temp_dir("engine_stats");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 768,
                metric: 2,
                model: None,
            })
            .unwrap();

        let stats = engine.get_stats("docs").unwrap();
        assert_eq!(stats.dim, 768);
        assert_eq!(stats.metric, 2);
        assert_eq!(stats.vector_count, 0);
        assert_eq!(stats.storage_bytes, 0);
        assert_eq!(stats.index_type, 0);
    }

    #[test]
    fn given_get_stats_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_stats_nonexistent");
        let engine = new_engine(&dir);

        let result = engine.get_stats("nonexistent");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_create_clowder_with_model_then_manifest_stores_model() {
        let dir = temp_dir("engine_model");
        let mut engine = new_engine(&dir);

        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 384,
                metric: 1,
                model: Some("all-MiniLM-L6-v2"),
            })
            .unwrap();

        let manifest_path = dir.join("collections").join("docs").join("manifest.json");
        let manifest: Manifest = ManifestManager::load_manifest(&manifest_path).unwrap();
        assert_eq!(manifest.model.as_deref(), Some("all-MiniLM-L6-v2"));

        let clowder = engine.clowders.get("docs").unwrap();
        assert_eq!(clowder.model.as_deref(), Some("all-MiniLM-L6-v2"));
    }

    #[test]
    fn given_create_then_simulate_restart_by_reading_manifest() {
        let dir = temp_dir("engine_restart");
        {
            let mut engine = new_engine(&dir);
            engine
                .create_clowder(CreateClowderDto {
                    name: "docs",
                    dim: 384,
                    metric: 1,
                    model: None,
                })
                .unwrap();
        }

        let manifest_path = dir.join("collections").join("docs").join("manifest.json");
        assert!(manifest_path.exists());

        let manifest: Manifest = ManifestManager::load_manifest(&manifest_path).unwrap();
        assert_eq!(manifest.collection_name, "docs");
        assert_eq!(manifest.dim, 384);
        assert_eq!(manifest.metric, 1);
        assert_eq!(manifest.version, 1);
    }

    #[test]
    fn given_valid_insert_then_get_returns_correct_vector() {
        let dir = temp_dir("engine_insert_get");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 1,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        engine.insert_vector("docs", "doc1", vector.clone(), &metadata).unwrap();
        let retrieved = engine.get_vector("docs", "doc1").unwrap();
        assert_eq!(retrieved, vec![1.0, 2.0, 3.0]);
    }

    #[test]
    fn given_insert_wrong_dim_then_returns_dim_mismatch() {
        let dir = temp_dir("engine_insert_dim");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 1,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let result = engine.insert_vector("docs", "doc1", vector, &metadata);
        assert_eq!(result.unwrap_err(), Hairball::DimMismatch);
    }

    #[test]
    fn given_insert_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_insert_nonexistent");
        let mut engine = new_engine(&dir);
        let vector = vec![1.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let result = engine.insert_vector("no_such_clowder", "doc1", vector, &metadata);
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_get_nonexistent_id_then_returns_not_found() {
        let dir = temp_dir("engine_get_nonexistent");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 1,
                model: None,
            })
            .unwrap();

        let result = engine.get_vector("docs", "no_such_doc");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_empty_wal_directory_then_replay_returns_empty() {
        let dir = temp_dir("engine_replay_empty");
        std::fs::create_dir_all(dir.join("wal")).unwrap();

        let entries = Engine::replay_wal_entries(&dir).unwrap();
        assert!(entries.is_empty());
    }
}
