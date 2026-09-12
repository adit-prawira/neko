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

use super::knn::{KNN, KNNSearchParams, ScoredVector};
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

pub struct InputVectorDto {
    pub id: String,
    pub vector: Vec<f32>,
    pub metadata: VectorMetadata,
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
                    metadata: Mutex::new(HashMap::new()),
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
                    if !entry.metadata.custom.is_empty() {
                        clowder.metadata.lock().unwrap().insert(entry.id.clone(), entry.metadata.custom.clone());
                    }
                }
                crate::wal::resource::OperationCode::Delete => {
                    clowder.vectors.lock().unwrap().remove(&entry.id);
                    clowder.metadata.lock().unwrap().remove(&entry.id);
                }
            }
        }
        let wal = WalWriter::open(&collection_directory, 64)
            .inspect_err(|err| eprintln!("WAL: failed to open write-ahead log ({}); insert will not be persisted", err))
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
                metadata: Mutex::new(HashMap::new()),
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

        let mut normalised_vector = vector;
        if clowder.metric == 1 {
            let normalised_squares: f32 = normalised_vector.iter().map(|x| x * x).sum();
            if normalised_squares > 1e-18 {
                let inverse_normal = 1.0 / normalised_squares.sqrt();
                for component in &mut normalised_vector {
                    *component *= inverse_normal;
                }
            }
        }

        let wal_id = format!("{}:{}", name, id);
        if let Some(ref mut wal) = self.wal {
            wal.append_insert(&wal_id, &normalised_vector, metadata)?;
        }
        clowder.vectors.lock().unwrap().insert(id.to_string(), normalised_vector);
        let has_custom_metadata = !metadata.custom.is_empty();
        if has_custom_metadata {
            clowder.metadata.lock().unwrap().insert(id.to_string(), metadata.custom.clone());
        } else {
            clowder.metadata.lock().unwrap().remove(id);
        }
        Ok(())
    }

    pub fn insert_many_vector(&mut self, name: &str, input_vectors: Vec<InputVectorDto>) -> Result<()> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        Engine::validate_vectors(clowder, &input_vectors)?;

        let normalised_input_vectors: Vec<InputVectorDto> = input_vectors
            .into_iter()
            .map(|input_vector| {
                let mut normalised_vector = input_vector.vector;
                if clowder.metric == 1 {
                    let normalised_squares: f32 = normalised_vector.iter().map(|x| x * x).sum();
                    if normalised_squares > 1e-18 {
                        let inverse_normal = 1.0 / normalised_squares.sqrt();
                        for component in &mut normalised_vector {
                            *component *= inverse_normal;
                        }
                    }
                }
                InputVectorDto {
                    id: input_vector.id,
                    vector: normalised_vector,
                    metadata: input_vector.metadata,
                }
            })
            .collect();

        if let Some(ref mut wal) = self.wal {
            for normalised_input_vector in &normalised_input_vectors {
                Engine::resolve_tail_log(wal, name, normalised_input_vector)?;
            }
        }

        {
            let mut vector_store = clowder.vectors.lock().unwrap();
            for normalised_input_vector in &normalised_input_vectors {
                Engine::resolve_vector_store(&mut vector_store, normalised_input_vector)?;
            }
        }

        {
            let mut metadata_store = clowder.metadata.lock().unwrap();
            for normalised_input_vector in &normalised_input_vectors {
                Engine::resolve_metadata_store(&mut metadata_store, normalised_input_vector)?;
            }
        }
        Ok(())
    }

    fn resolve_tail_log(wal: &mut WalWriter, name: &str, input_vector: &InputVectorDto) -> Result<()> {
        let wal_id = format!("{}:{}", name, input_vector.id);
        wal.append_insert(&wal_id, &input_vector.vector, &input_vector.metadata)?;
        Ok(())
    }

    fn resolve_vector_store(vector_store: &mut HashMap<String, Vec<f32>>, input_vector: &InputVectorDto) -> Result<()> {
        vector_store.insert(input_vector.id.to_string(), input_vector.vector.clone());
        Ok(())
    }

    fn resolve_metadata_store(metadata_store: &mut HashMap<String, String>, input_vector: &InputVectorDto) -> Result<()> {
        let has_custom_metadata = !input_vector.metadata.custom.is_empty();
        if has_custom_metadata {
            metadata_store.insert(input_vector.id.clone(), input_vector.metadata.custom.clone());
        } else {
            metadata_store.remove(&input_vector.id);
        }
        Ok(())
    }

    fn validate_vectors(clowder: &Clowder, input_vectors: &Vec<InputVectorDto>) -> Result<()> {
        for input_vector in input_vectors {
            if input_vector.vector.len() != clowder.dim as usize {
                return Err(Hairball::DimMismatch);
            }
        }
        Ok(())
    }

    pub fn upsert_vector(&mut self, name: &str, id: &str, vector: Vec<f32>, metadata: &VectorMetadata) -> Result<()> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        if vector.len() != clowder.dim as usize {
            return Err(Hairball::DimMismatch);
        }
        let mut normalised_vector = vector;
        if clowder.metric == 1 {
            let normalised_squares: f32 = normalised_vector.iter().map(|x| x * x).sum();
            if normalised_squares > 1e-18 {
                let inverse_normal = 1.0 / normalised_squares.sqrt();
                for component in &mut normalised_vector {
                    *component *= inverse_normal;
                }
            }
        }

        let wal_id = format!("{}:{}", name, id);
        if let Some(ref mut wal) = self.wal {
            wal.append_delete(&wal_id)?;
            wal.append_insert(&wal_id, &normalised_vector, metadata)?;
        }
        clowder.vectors.lock().unwrap().insert(id.to_string(), normalised_vector);
        let has_custom_metadata = !metadata.custom.is_empty();
        if has_custom_metadata {
            clowder.metadata.lock().unwrap().insert(id.to_string(), metadata.custom.clone());
        } else {
            clowder.metadata.lock().unwrap().remove(id);
        }
        Ok(())
    }

    pub fn get_vector(&self, name: &str, id: &str) -> Result<Vec<f32>> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        let vectors = clowder.vectors.lock().unwrap();
        vectors.get(id).cloned().ok_or(Hairball::NotFound)
    }

    pub fn delete_vector(&mut self, name: &str, id: &str) -> Result<()> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;
        let exist = clowder.vectors.lock().unwrap().contains_key(id);

        if !exist {
            return Err(Hairball::NotFound);
        }

        let wal_id = format!("{}:{}", name, id);
        if let Some(ref mut wal) = self.wal {
            wal.append_delete(&wal_id)?;
        }
        clowder.vectors.lock().unwrap().remove(id);
        clowder.metadata.lock().unwrap().remove(id);
        Ok(())
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

    pub fn search(&self, name: &str, query: &[f32], top_k: usize) -> Result<Vec<ScoredVector>> {
        let clowder = self.clowders.get(name).ok_or(Hairball::NotFound)?;

        if query.len() != clowder.dim as usize {
            return Err(Hairball::DimMismatch);
        }
        let vectors = clowder.vectors.lock().unwrap();
        KNN::search(
            &vectors,
            &KNNSearchParams {
                query,
                top_k,
                metric: clowder.metric,
                dim: clowder.dim,
            },
        )
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
        let inv_norm = 1.0 / (1.0_f32 * 1.0 + 2.0 * 2.0 + 3.0 * 3.0).sqrt();
        assert_eq!(retrieved, vec![1.0 * inv_norm, 2.0 * inv_norm, 3.0 * inv_norm]);
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
    fn given_valid_upsert_then_get_returns_correct_vector() {
        let dir = temp_dir("engine_upsert_get");
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

        engine.upsert_vector("docs", "doc1", vector.clone(), &metadata).unwrap();
        let retrieved = engine.get_vector("docs", "doc1").unwrap();
        let inv_norm = 1.0 / (1.0_f32 * 1.0 + 2.0 * 2.0 + 3.0 * 3.0).sqrt();
        assert_eq!(retrieved, vec![1.0 * inv_norm, 2.0 * inv_norm, 3.0 * inv_norm]);
    }

    #[test]
    fn given_upsert_existing_vector_then_value_is_replaced() {
        let dir = temp_dir("engine_upsert_replace");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        engine.insert_vector("docs", "doc1", vec![1.0_f32, 0.0, 0.0], &metadata).unwrap();
        engine.upsert_vector("docs", "doc1", vec![0.0_f32, 1.0, 0.0], &metadata).unwrap();
        let retrieved = engine.get_vector("docs", "doc1").unwrap();
        assert_eq!(retrieved, vec![0.0_f32, 1.0, 0.0]);
    }

    #[test]
    fn given_upsert_wrong_dim_then_returns_dim_mismatch() {
        let dir = temp_dir("engine_upsert_dim");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let result = engine.upsert_vector("docs", "doc1", vec![1.0_f32, 2.0_f32], &metadata);
        assert_eq!(result.unwrap_err(), Hairball::DimMismatch);
    }

    #[test]
    fn given_upsert_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_upsert_nonexistent");
        let mut engine = new_engine(&dir);
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };

        let result = engine.upsert_vector("no_such_clowder", "doc1", vec![1.0_f32], &metadata);
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

    #[test]
    fn given_l2_vectors_then_search_returns_nearest_by_distance() {
        let dir = temp_dir("engine_search_l2");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "pts",
                dim: 2,
                metric: 0,
                model: None,
            })
            .unwrap();

        let meta = VectorMetadata {
            id: "".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        engine.insert_vector("pts", "a", vec![1.0, 0.0], &meta).unwrap();
        engine.insert_vector("pts", "b", vec![9.0, 0.0], &meta).unwrap();
        engine.insert_vector("pts", "c", vec![3.0, 0.0], &meta).unwrap();

        let results = engine.search("pts", &[0.0, 0.0], 3).unwrap();
        assert_eq!(results.len(), 3);
        assert_eq!(results[0].id, "a");
        assert!(results[0].score <= results[1].score);
        assert!(results[1].score <= results[2].score);
    }

    #[test]
    fn given_search_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_search_nf");
        let engine = new_engine(&dir);
        let result = engine.search("ghost", &[1.0, 2.0], 5);
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_search_wrong_query_dim_then_returns_dim_mismatch() {
        let dir = temp_dir("engine_search_dim");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "pts",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let result = engine.search("pts", &[1.0, 2.0], 5);
        assert_eq!(result.unwrap_err(), Hairball::DimMismatch);
    }

    #[test]
    fn given_search_top_k_one_then_returns_single_best() {
        let dir = temp_dir("engine_search_top1");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "pts",
                dim: 1,
                metric: 0,
                model: None,
            })
            .unwrap();

        let meta = VectorMetadata {
            id: "".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        engine.insert_vector("pts", "far", vec![100.0], &meta).unwrap();
        engine.insert_vector("pts", "near", vec![2.0], &meta).unwrap();

        let results = engine.search("pts", &[0.0], 1).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "near");
    }

    #[test]
    fn given_delete_existing_vector_then_get_returns_not_found() {
        let dir = temp_dir("engine_delete_get");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
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
        engine.insert_vector("docs", "doc1", vector, &metadata).unwrap();
        engine.delete_vector("docs", "doc1").unwrap();

        let result = engine.get_vector("docs", "doc1");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_delete_nonexistent_id_then_returns_not_found() {
        let dir = temp_dir("engine_delete_bad_id");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let result = engine.delete_vector("docs", "ghost");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_delete_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_delete_bad_clowder");
        let mut engine = new_engine(&dir);

        let result = engine.delete_vector("ghost", "some_id");
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_delete_vector_then_search_excludes_deleted() {
        let dir = temp_dir("engine_delete_search");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "pts",
                dim: 2,
                metric: 0,
                model: None,
            })
            .unwrap();

        let metadata = VectorMetadata {
            id: "".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        engine.insert_vector("pts", "keep", vec![1.0, 0.0], &metadata).unwrap();
        engine.insert_vector("pts", "toss", vec![5.0, 0.0], &metadata).unwrap();

        engine.delete_vector("pts", "toss").unwrap();

        let results = engine.search("pts", &[0.0, 0.0], 2).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "keep");
    }

    #[test]
    fn given_insert_with_non_empty_custom_metadata_then_metadata_map_stores_value() {
        let dir = temp_dir("engine_insert_meta_store");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "{\"author\":\"alice\"}".to_string(),
        };

        engine.insert_vector("docs", "doc1", vector, &metadata).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert_eq!(stored.get("doc1"), Some(&"{\"author\":\"alice\"}".to_string()));
    }

    #[test]
    fn given_reinsert_with_empty_custom_metadata_then_metadata_map_removes_id() {
        let dir = temp_dir("engine_reinsert_empty_meta");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];

        // First insert with metadata present.
        let with_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "first".to_string(),
        };
        engine.insert_vector("docs", "doc1", vector.clone(), &with_metadata).unwrap();

        // Precondition: metadata must be in the map after the first insert.
        {
            let clowder = engine.clowders.get("docs").unwrap();
            let stored = clowder.metadata.lock().unwrap();
            assert_eq!(stored.get("doc1"), Some(&"first".to_string()));
        }

        // Re-insert the same id with empty custom — the previous metadata
        // entry must be cleared, not retained.
        let without_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        engine.insert_vector("docs", "doc1", vector, &without_metadata).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert!(!stored.contains_key("doc1"));
    }

    #[test]
    fn given_delete_then_metadata_map_clears_id() {
        let dir = temp_dir("engine_delete_meta_clear");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "anything".to_string(),
        };
        engine.insert_vector("docs", "doc1", vector, &metadata).unwrap();

        // Precondition: metadata must be in the map after insert.
        {
            let clowder = engine.clowders.get("docs").unwrap();
            let stored = clowder.metadata.lock().unwrap();
            assert_eq!(stored.get("doc1"), Some(&"anything".to_string()));
        }

        engine.delete_vector("docs", "doc1").unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert!(!stored.contains_key("doc1"));
    }

    #[test]
    fn given_upsert_with_non_empty_custom_metadata_then_metadata_map_stores_value() {
        let dir = temp_dir("engine_upsert_meta_store");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "{\"author\":\"alice\"}".to_string(),
        };

        engine.upsert_vector("docs", "doc1", vector, &metadata).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert_eq!(stored.get("doc1"), Some(&"{\"author\":\"alice\"}".to_string()));
    }

    #[test]
    fn given_upsert_replaces_existing_metadata_then_metadata_map_value_updated() {
        let dir = temp_dir("engine_upsert_meta_replace");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let first_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "first".to_string(),
        };
        engine.upsert_vector("docs", "doc1", vector.clone(), &first_metadata).unwrap();

        let second_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "second".to_string(),
        };
        engine.upsert_vector("docs", "doc1", vector, &second_metadata).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert_eq!(stored.get("doc1"), Some(&"second".to_string()));
    }

    #[test]
    fn given_upsert_with_empty_custom_metadata_then_metadata_map_removes_id() {
        let dir = temp_dir("engine_upsert_meta_clear");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let vector = vec![1.0_f32, 2.0_f32, 3.0_f32];
        let with_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: "anything".to_string(),
        };
        engine.upsert_vector("docs", "doc1", vector.clone(), &with_metadata).unwrap();

        // Precondition: metadata must be in the map after the first upsert.
        {
            let clowder = engine.clowders.get("docs").unwrap();
            let stored = clowder.metadata.lock().unwrap();
            assert_eq!(stored.get("doc1"), Some(&"anything".to_string()));
        }

        // Upsert with empty custom — the previous metadata entry must be cleared.
        let without_metadata = VectorMetadata {
            id: "doc1".to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        };
        engine.upsert_vector("docs", "doc1", vector, &without_metadata).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert!(!stored.contains_key("doc1"));
    }

    #[test]
    fn given_valid_batch_then_all_vectors_inserted() {
        let dir = temp_dir("engine_insert_many_basic");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let items = vec![
            InputVectorDto {
                id: "doc1".to_string(),
                vector: vec![1.0, 2.0, 3.0],
                metadata: empty_metadata("doc1"),
            },
            InputVectorDto {
                id: "doc2".to_string(),
                vector: vec![4.0, 5.0, 6.0],
                metadata: empty_metadata("doc2"),
            },
            InputVectorDto {
                id: "doc3".to_string(),
                vector: vec![7.0, 8.0, 9.0],
                metadata: empty_metadata("doc3"),
            },
        ];
        engine.insert_many_vector("docs", items).unwrap();

        assert_eq!(engine.get_vector("docs", "doc1").unwrap(), vec![1.0, 2.0, 3.0]);
        assert_eq!(engine.get_vector("docs", "doc2").unwrap(), vec![4.0, 5.0, 6.0]);
        assert_eq!(engine.get_vector("docs", "doc3").unwrap(), vec![7.0, 8.0, 9.0]);
    }

    #[test]
    fn given_batch_with_wrong_dim_item_then_returns_dim_mismatch() {
        let dir = temp_dir("engine_insert_many_dim");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let items = vec![
            InputVectorDto {
                id: "doc1".to_string(),
                vector: vec![1.0, 2.0, 3.0],
                metadata: empty_metadata("doc1"),
            },
            InputVectorDto {
                id: "doc2".to_string(),
                vector: vec![4.0, 5.0],
                metadata: empty_metadata("doc2"),
            },
        ];
        let result = engine.insert_many_vector("docs", items);
        assert_eq!(result.unwrap_err(), Hairball::DimMismatch);
    }

    #[test]
    fn given_batch_with_nonexistent_clowder_then_returns_not_found() {
        let dir = temp_dir("engine_insert_many_nf");
        let mut engine = new_engine(&dir);

        let items = vec![InputVectorDto {
            id: "doc1".to_string(),
            vector: vec![1.0, 2.0, 3.0],
            metadata: empty_metadata("doc1"),
        }];
        let result = engine.insert_many_vector("ghost", items);
        assert_eq!(result.unwrap_err(), Hairball::NotFound);
    }

    #[test]
    fn given_batch_with_cosine_metric_then_vectors_normalised() {
        let dir = temp_dir("engine_insert_many_cosine");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 2,
                metric: 1,
                model: None,
            })
            .unwrap();

        let items = vec![InputVectorDto {
            id: "doc1".to_string(),
            vector: vec![3.0, 4.0],
            metadata: empty_metadata("doc1"),
        }];
        engine.insert_many_vector("docs", items).unwrap();
        assert_eq!(engine.get_vector("docs", "doc1").unwrap(), vec![0.6, 0.8]);
    }

    #[test]
    fn given_batch_with_non_empty_metadata_then_metadata_map_stores_value() {
        let dir = temp_dir("engine_insert_many_meta");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        let items = vec![InputVectorDto {
            id: "doc1".to_string(),
            vector: vec![1.0, 2.0, 3.0],
            metadata: with_custom_metadata("doc1", r#"{"author":"alice"}"#),
        }];
        engine.insert_many_vector("docs", items).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert_eq!(stored.get("doc1").map(|s| s.as_str()), Some(r#"{"author":"alice"}"#));
    }

    #[test]
    fn given_batch_with_empty_custom_metadata_then_existing_metadata_is_cleared() {
        let dir = temp_dir("engine_insert_many_meta_clear");
        let mut engine = new_engine(&dir);
        engine
            .create_clowder(CreateClowderDto {
                name: "docs",
                dim: 3,
                metric: 0,
                model: None,
            })
            .unwrap();

        engine.insert_vector("docs", "doc1", vec![1.0, 2.0, 3.0], &with_custom_metadata("doc1", "first")).unwrap();
        {
            let clowder = engine.clowders.get("docs").unwrap();
            let stored = clowder.metadata.lock().unwrap();
            assert_eq!(stored.get("doc1").map(|s| s.as_str()), Some("first"));
        }

        let items = vec![InputVectorDto {
            id: "doc1".to_string(),
            vector: vec![4.0, 5.0, 6.0],
            metadata: empty_metadata("doc1"),
        }];
        engine.insert_many_vector("docs", items).unwrap();

        let clowder = engine.clowders.get("docs").unwrap();
        let stored = clowder.metadata.lock().unwrap();
        assert!(!stored.contains_key("doc1"), "empty custom metadata must clear existing entry");
    }

    fn empty_metadata(id: &str) -> VectorMetadata {
        VectorMetadata {
            id: id.to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        }
    }

    fn with_custom_metadata(id: &str, custom: &str) -> VectorMetadata {
        VectorMetadata {
            id: id.to_string(),
            created_at: 0,
            deleted: false,
            custom: custom.to_string(),
        }
    }
}
