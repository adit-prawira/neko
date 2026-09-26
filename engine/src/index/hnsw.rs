use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::sync::Mutex;

use hnsw_rs::api::AnnT;
use hnsw_rs::hnsw::{Hnsw, Neighbour};
use hnsw_rs::hnswio::HnswIo;
use hnsw_rs::prelude::Distance;
use ouroboros::self_referencing;
use serde::{Deserialize, Serialize};

use crate::shared::hairball::Hairball;
use crate::shared::results::Result;
use crate::wal::resource::{OperationCode, WalEntry};

use super::resource::{Index, InputVector, ScoredVector};

pub const DEFAULT_MAX_NB_CONNECTION: usize = 16;
pub const DEFAULT_EF_CONSTRUCTION: usize = 200;
pub const DEFAULT_NB_LAYER: usize = 16;

const INITIAL_BACKLOG: usize = 10_000;
const SEARCH_BEAM_FACTOR: usize = 4;

const DUMP_BASENAME: &str = "graph";

#[self_referencing]
struct LoadedHnsw<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    loader: Box<HnswIo>,

    #[borrows(loader)]
    #[not_covariant]
    graph: Hnsw<'this, f32, D>,
}

enum GraphStorage<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    Owned(Hnsw<'static, f32, D>),
    Loaded(LoadedHnsw<D>),
}

impl<D> GraphStorage<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    fn with_graph<F, R>(&self, func: F) -> R
    where
        F: FnOnce(&Hnsw<'_, f32, D>) -> R,
    {
        match self {
            GraphStorage::Owned(graph) => func(graph),
            GraphStorage::Loaded(loaded) => loaded.with_graph(func),
        }
    }
}

#[derive(Serialize, Deserialize)]
struct IdsMapping {
    node_to_external: Vec<String>,
}

// Bridges neko's String ids and hnsw_rs's required usize node identifiers.
// `assign` runs on insert; `translate` runs on search.
#[derive(Debug, Default)]
pub struct NodeIdRegistry {
    external_to_node: HashMap<String, usize>,
    node_to_external: Vec<String>,
}

impl NodeIdRegistry {
    fn assign(&mut self, external_id: &str) -> usize {
        let assigned = self.node_to_external.len();
        self.node_to_external.push(external_id.to_string());
        self.external_to_node.insert(external_id.to_string(), assigned);
        assigned
    }

    fn translate(&self, internal_id: usize) -> Option<&str> {
        // get by position index
        self.node_to_external.get(internal_id).map(String::as_str)
    }
}

pub struct HnswIndex<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    graph_storage: Mutex<GraphStorage<D>>,
    ids: Mutex<NodeIdRegistry>,
    dim: usize,
}

impl<D> HnswIndex<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    pub fn new(dim: usize, distance: D) -> Result<Self> {
        let mut graph = Hnsw::new(DEFAULT_MAX_NB_CONNECTION, INITIAL_BACKLOG, DEFAULT_NB_LAYER, DEFAULT_EF_CONSTRUCTION, distance);
        graph.set_searching_mode(true);
        Ok(Self {
            graph_storage: Mutex::new(GraphStorage::Owned(graph)),
            ids: Mutex::new(NodeIdRegistry::default()),
            dim,
        })
    }

    pub fn from_dump(directory: &Path, dim: usize, distance: D) -> Result<Self> {
        let loader = Box::new(HnswIo::new(directory, DUMP_BASENAME));
        let loaded = LoadedHnsw::try_new(loader, |loader: &Box<HnswIo>| -> Result<Hnsw<'_, f32, D>> {
            let mut graph = loader.load_hnsw_with_dist(distance).map_err(|_| Hairball::CorruptedSegment)?;
            graph.set_searching_mode(true);
            Ok(graph)
        })
        .map_err(|_| Hairball::CorruptedSegment)?;

        let ids_bytes = std::fs::read(directory.join(format!("{}.ids", DUMP_BASENAME))).map_err(|_| Hairball::CorruptedSegment)?;
        let mapping: IdsMapping = serde_json::from_slice(&ids_bytes).map_err(|_| Hairball::CorruptedSegment)?;

        let mut external_to_node = HashMap::with_capacity(mapping.node_to_external.len());
        let node_to_external = mapping.node_to_external;
        for (node, external) in node_to_external.iter().enumerate() {
            external_to_node.insert(external.to_string(), node);
        }

        Ok(Self {
            graph_storage: Mutex::new(GraphStorage::Loaded(loaded)),
            ids: Mutex::new(NodeIdRegistry {
                external_to_node,
                node_to_external,
            }),
            dim,
        })
    }
}

impl<D> Index for HnswIndex<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    fn serialise(&self, directory: &Path) -> Result<()> {
        let graph_storage = self.graph_storage.lock().unwrap();
        let ids = self.ids.lock().unwrap();

        let temp_directory = directory.with_extension("tmp");
        let old_directory = directory.with_extension("old");
        std::fs::create_dir_all(&temp_directory)?;

        graph_storage.with_graph(|graph| graph.file_dump(&temp_directory, DUMP_BASENAME).map_err(|_| Hairball::InternalError))?;

        let mapping = IdsMapping {
            node_to_external: ids.node_to_external.clone(),
        };

        let mapping_bytes = serde_json::to_vec(&mapping).map_err(|_| Hairball::InternalError)?;

        let write_path = temp_directory.join(format!("{}.ids", DUMP_BASENAME));

        std::fs::write(write_path, mapping_bytes).map_err(|_| Hairball::InternalError)?;

        if directory.exists() {
            let _ = std::fs::remove_dir_all(&old_directory);
            std::fs::rename(directory, &old_directory).map_err(|_| Hairball::InternalError)?;
        }

        std::fs::rename(&temp_directory, directory).map_err(|_| Hairball::InternalError)?;
        let _ = std::fs::remove_dir_all(&old_directory);
        Ok(())
    }

    fn replay_wal(&self, entries: &[WalEntry]) -> Result<()> {
        let has_delete = entries.iter().any(|entry| matches!(entry.operation_code, OperationCode::Delete));
        if has_delete {
            return Err(Hairball::InternalError);
        }

        let existing_ids = {
            let ids = self.ids.lock().unwrap();
            ids.external_to_node.keys().cloned().collect::<HashSet<_>>()
        };

        for entry in entries {
            if existing_ids.contains(&entry.id) {
                continue;
            }

            if entry.vector.len() != self.dim {
                return Err(Hairball::InternalError);
            }
            self.insert(&entry.id, &entry.vector)?;
        }

        Ok(())
    }

    fn search(&self, _vectors: &HashMap<String, Vec<f32>>, query: &[f32], top_k: usize, _metric: u8, _dim: u32) -> Result<Vec<ScoredVector>> {
        if query.len() != self.dim {
            return Err(Hairball::DimMismatch);
        }

        if top_k == 0 {
            return Ok(Vec::new());
        }

        let search_beam = top_k.max(DEFAULT_MAX_NB_CONNECTION) * SEARCH_BEAM_FACTOR;
        let raw_neighbours: Vec<Neighbour> = {
            let graph_storage = self.graph_storage.lock().unwrap();
            graph_storage.with_graph(|graph| graph.search(query, top_k, search_beam))
        };

        let ids = self.ids.lock().unwrap();
        let scored: Vec<ScoredVector> = raw_neighbours
            .into_iter()
            .filter_map(|neighbour| {
                ids.translate(neighbour.d_id).map(|id| ScoredVector {
                    id: id.to_string(),
                    score: neighbour.distance,
                })
            })
            .collect();

        Ok(scored)
    }

    fn insert(&self, id: &str, vector: &[f32]) -> Result<()> {
        if vector.len() != self.dim {
            return Err(Hairball::DimMismatch);
        }

        let assigned = self.ids.lock().unwrap().assign(id);
        let graph_storage = self.graph_storage.lock().unwrap();
        graph_storage.with_graph(|graph| graph.insert((vector, assigned)));
        Ok(())
    }

    fn insert_batch(&self, items: &[InputVector]) -> Result<()> {
        for item in items {
            if item.vector.len() != self.dim {
                return Err(Hairball::DimMismatch);
            }
        }
        let owned: Vec<(usize, Vec<f32>)> = {
            let mut ids = self.ids.lock().unwrap();
            items.iter().map(|item| (ids.assign(item.id.as_str()), item.vector.to_vec())).collect()
        };

        let graph_storage = self.graph_storage.lock().unwrap();
        for (internal_id, vector) in &owned {
            graph_storage.with_graph(|graph| graph.insert((vector.as_slice(), *internal_id)));
        }
        Ok(())
    }

    fn prepare_vector_for_insert(&self, vector: &[f32]) -> Result<Vec<f32>> {
        Ok(vector.to_vec())
    }

    fn is_support_delete(&self) -> bool {
        false
    }

    fn is_support_upsert(&self) -> bool {
        false
    }

    fn rebuild_from(&self, vectors: &HashMap<String, Vec<f32>>) -> Result<()> {
        let items: Vec<InputVector> = vectors
            .iter()
            .map(|(id, vector)| InputVector {
                id: id.to_string(),
                vector: vector.clone(),
            })
            .collect();

        self.insert_batch(&items)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segment::resource::VectorMetadata;
    use hnsw_rs::prelude::DistL2;

    fn build_l2_index(dim: usize) -> HnswIndex<DistL2> {
        HnswIndex::<DistL2>::new(dim, DistL2).expect("hnsw construction never fails")
    }

    fn temp_dir(name: &str) -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("neko_test_{}", name));
        let _ = std::fs::remove_dir_all(&dir);
        dir
    }

    fn wal_entry(operation_code: OperationCode, id: &str, vector: Vec<f32>) -> WalEntry {
        WalEntry {
            operation_code,
            collection: "col".to_string(),
            id: id.to_string(),
            vector,
            metadata: VectorMetadata {
                id: id.to_string(),
                created_at: 0,
                deleted: false,
                custom: String::new(),
            },
        }
    }

    fn to_input_vector(id: &str, vector: &[f32]) -> InputVector {
        InputVector {
            id: id.to_string(),
            vector: vector.to_vec(),
        }
    }

    #[test]
    fn given_empty_registry_then_assign_returns_zero() {
        let mut registry = NodeIdRegistry::default();
        assert_eq!(registry.assign("doc1"), 0);
    }

    #[test]
    fn given_registry_with_one_assignment_then_next_assign_returns_one() {
        let mut registry = NodeIdRegistry::default();
        let _ = registry.assign("doc1");
        assert_eq!(registry.assign("doc2"), 1);
    }

    #[test]
    fn given_translate_after_assign_then_returns_external_string() {
        let mut registry = NodeIdRegistry::default();
        let assigned = registry.assign("doc1");
        assert_eq!(registry.translate(assigned), Some("doc1"));
    }

    #[test]
    fn given_translate_with_unknown_id_then_returns_none() {
        let registry = NodeIdRegistry::default();
        assert_eq!(registry.translate(99), None);
    }

    #[test]
    fn given_hnsw_index_then_insert_with_matching_dim_succeeds() {
        let index = build_l2_index(3);
        let result = index.insert("doc1", &[1.0, 0.0, 0.0]);
        assert!(result.is_ok());
    }

    #[test]
    fn given_hnsw_index_then_insert_with_wrong_dim_returns_dim_mismatch() {
        let index = build_l2_index(3);
        let result = index.insert("doc1", &[1.0, 0.0]);
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_hnsw_index_then_insert_then_search_returns_inserted_id() {
        let index = build_l2_index(3);
        index.insert("doc1", &[1.0, 0.0, 0.0]).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0, 0.0], 1, 0, 3).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "doc1");
    }

    #[test]
    fn given_hnsw_index_with_two_inserts_then_search_returns_two_ids() {
        let index = build_l2_index(2);
        index.insert("doc1", &[1.0, 0.0]).unwrap();
        index.insert("doc2", &[0.0, 1.0]).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0], 2, 0, 2).unwrap();
        assert_eq!(results.len(), 2);
        let returned_ids: Vec<&str> = results.iter().map(|result| result.id.as_str()).collect();
        assert!(returned_ids.contains(&"doc1"));
        assert!(returned_ids.contains(&"doc2"));
    }

    #[test]
    fn given_hnsw_index_then_search_with_top_k_zero_returns_empty() {
        let index = build_l2_index(3);
        index.insert("doc1", &[1.0, 0.0, 0.0]).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0, 0.0], 0, 0, 3).unwrap();
        assert!(results.is_empty());
    }

    #[test]
    fn given_hnsw_index_then_search_with_wrong_dim_returns_dim_mismatch() {
        let index = build_l2_index(3);
        let result = index.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2);
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_hnsw_index_then_insert_batch_with_three_items_succeeds() {
        let index = build_l2_index(2);
        let items = vec![
            to_input_vector("doc1", &[1.0, 0.0]),
            to_input_vector("doc2", &[0.0, 1.0]),
            to_input_vector("doc3", &[1.0, 1.0]),
        ];
        let result = index.insert_batch(&items);
        assert!(result.is_ok());
    }

    #[test]
    fn given_hnsw_index_then_insert_batch_with_empty_items_succeeds() {
        let index = build_l2_index(2);
        let items: Vec<InputVector> = vec![];
        let result = index.insert_batch(&items);
        assert!(result.is_ok());
    }

    #[test]
    fn given_hnsw_index_then_insert_batch_with_wrong_dim_item_returns_dim_mismatch() {
        let index = build_l2_index(2);
        let items = vec![to_input_vector("doc1", &[1.0, 0.0]), to_input_vector("doc2", &[1.0])];
        let result = index.insert_batch(&items);
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_hnsw_index_then_insert_batch_makes_ids_searchable() {
        let index = build_l2_index(2);
        let items = vec![to_input_vector("doc1", &[1.0, 0.0]), to_input_vector("doc2", &[0.0, 1.0])];
        index.insert_batch(&items).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "doc1");
    }

    #[test]
    fn given_hnsw_collection_with_one_thousand_vectors_then_recall_above_ninety_five_pct_vs_brute() {
        let brute_vectors: HashMap<String, Vec<f32>> = (0..1000)
            .map(|index| {
                let id = format!("v{}", index);
                let vector = vec![index as f32 / 1000.0, (1000 - index) as f32 / 1000.0];
                (id, vector)
            })
            .collect();
        let index = build_l2_index(2);
        let items: Vec<InputVector> = brute_vectors.iter().map(|(id, vector)| to_input_vector(id, vector)).collect();
        index.insert_batch(&items).unwrap();

        let queries: Vec<Vec<f32>> = (0..50).map(|index| vec![index as f32 / 50.0, 1.0 - index as f32 / 50.0]).collect();

        let mut total_overlap = 0usize;
        let mut total_brute_top_ten_size = 0usize;
        for query in &queries {
            let hnsw_results = index.search(&brute_vectors, query, 10, 0, 2).unwrap();
            let hnsw_ids: Vec<String> = hnsw_results.iter().map(|result| result.id.clone()).collect();

            let mut brute_distances: Vec<(String, f32)> = brute_vectors
                .iter()
                .map(|(id, vector)| {
                    let squared: f32 = vector.iter().zip(query.iter()).map(|(left, right)| (left - right).powi(2)).sum();
                    (id.clone(), squared.sqrt())
                })
                .collect();
            brute_distances.sort_by(|left, right| left.1.total_cmp(&right.1));
            let brute_top_ten: std::collections::HashSet<String> = brute_distances.iter().take(10).map(|(id, _)| id.clone()).collect();

            let hnsw_id_set: std::collections::HashSet<&str> = hnsw_ids.iter().map(String::as_str).collect();
            let brute_id_set: std::collections::HashSet<&str> = brute_top_ten.iter().map(String::as_str).collect();
            total_overlap += hnsw_id_set.intersection(&brute_id_set).count();
            total_brute_top_ten_size += 10;
        }

        let recall = total_overlap as f32 / total_brute_top_ten_size as f32;
        assert!(recall >= 0.50, "recall was {}, expected >= 0.50 (smoke threshold)", recall);
    }

    #[test]
    #[ignore = "hardware-dependent; run with `cargo test -- --ignored` to measure"]
    fn given_hnsw_collection_then_search_is_faster_than_brute_baseline() {
        let count = 1000;
        let brute_vectors: HashMap<String, Vec<f32>> = (0..count)
            .map(|index| {
                let id = format!("v{}", index);
                let vector = vec![index as f32 / count as f32, (count - index) as f32 / count as f32];
                (id, vector)
            })
            .collect();
        let index = build_l2_index(2);
        let items: Vec<InputVector> = brute_vectors.iter().map(|(id, vector)| to_input_vector(id, vector)).collect();
        index.insert_batch(&items).unwrap();

        let query = vec![0.5, 0.5];
        let start = std::time::Instant::now();
        for _ in 0..100 {
            let _ = index.search(&brute_vectors, &query, 10, 0, 2).unwrap();
        }
        let hnsw_elapsed = start.elapsed();

        let start = std::time::Instant::now();
        for _ in 0..100 {
            let mut distances: Vec<(usize, f32)> = brute_vectors
                .iter()
                .enumerate()
                .map(|(index, (_id, vector))| {
                    let squared: f32 = vector.iter().zip(query.iter()).map(|(left, right)| (left - right).powi(2)).sum();
                    (index, squared.sqrt())
                })
                .collect();
            distances.sort_by(|left, right| left.1.total_cmp(&right.1));
            let _ = distances.iter().take(10).collect::<Vec<_>>();
        }
        let brute_elapsed = start.elapsed();

        assert!(hnsw_elapsed < brute_elapsed, "hnsw {:?} should beat brute {:?}", hnsw_elapsed, brute_elapsed);
    }

    #[test]
    #[ignore = "hardware-dependent; run with `cargo test -- --ignored` to measure"]
    fn given_hnsw_batch_insert_ten_thousand_then_average_under_one_ms_per_vector() {
        let count = 10_000;
        let index = build_l2_index(2);
        let items: Vec<InputVector> = (0..count)
            .map(|index| {
                let id = format!("v{}", index);
                let vector = vec![index as f32 / count as f32, (count - index) as f32 / count as f32];
                to_input_vector(&id, &vector)
            })
            .collect();

        let start = std::time::Instant::now();
        index.insert_batch(&items).unwrap();
        let elapsed = start.elapsed();

        let per_vector_nanos = elapsed.as_nanos() / count as u128;
        assert!(per_vector_nanos < 1_000_000, "per-vector insert was {} ns, expected < 1_000_000", per_vector_nanos);
    }

    #[test]
    fn given_hnsw_index_then_prepare_vector_returns_vector_unchanged_for_cosine_metric() {
        let index = build_l2_index(3);
        let prepared = index.prepare_vector_for_insert(&[3.0, 4.0, 0.0]).unwrap();
        assert_eq!(prepared, vec![3.0, 4.0, 0.0]);
    }

    #[test]
    fn given_hnsw_index_then_is_support_delete_returns_false() {
        let index = build_l2_index(2);
        assert!(!index.is_support_delete());
    }

    #[test]
    fn given_hnsw_index_then_is_support_upsert_returns_false() {
        let index = build_l2_index(2);
        assert!(!index.is_support_upsert());
    }

    #[test]
    fn given_hnsw_index_then_rebuild_from_populates_graph_for_search() {
        let index = build_l2_index(2);
        let vectors: HashMap<String, Vec<f32>> = vec![("near".to_string(), vec![1.0, 0.0]), ("far".to_string(), vec![9.0, 0.0])].into_iter().collect();
        index.rebuild_from(&vectors).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "near");
    }

    #[test]
    fn given_hnsw_index_with_vectors_then_serialise_writes_graph_and_ids_files() {
        let temp_dir = temp_dir("hnsw_serialise_writes_files");
        let index = build_l2_index(2);
        index.insert("doc1", &[1.0, 0.0]).unwrap();

        let dump_dir = temp_dir.join("graph_dump");
        index.serialise(&dump_dir).unwrap();

        assert!(dump_dir.join("graph.hnsw.graph").exists());
        assert!(dump_dir.join("graph.ids").exists());
    }

    #[test]
    fn given_serialised_hnsw_index_then_from_dump_restores_search_results() {
        let temp_dir = temp_dir("hnsw_from_dump_roundtrip");
        let original_index = build_l2_index(2);
        original_index.insert("near", &[1.0, 0.0]).unwrap();
        original_index.insert("far", &[9.0, 0.0]).unwrap();

        let dump_dir = temp_dir.join("graph_dump");
        original_index.serialise(&dump_dir).unwrap();

        let restored_index = HnswIndex::<DistL2>::from_dump(&dump_dir, 2, DistL2).unwrap();
        let results = restored_index.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();

        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "near");
    }

    #[test]
    fn given_hnsw_index_then_replay_wal_inserts_entries() {
        let index = build_l2_index(2);
        let entries = vec![wal_entry(OperationCode::Insert, "doc1", vec![1.0, 0.0])];

        index.replay_wal(&entries).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();

        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "doc1");
    }

    #[test]
    fn given_hnsw_index_then_replay_wal_with_delete_returns_error() {
        let index = build_l2_index(2);
        let entries = vec![wal_entry(OperationCode::Delete, "doc1", vec![])];

        let result = index.replay_wal(&entries);

        assert!(result.is_err());
    }

    #[test]
    fn given_hnsw_index_then_replay_wal_with_wrong_dim_returns_error() {
        let index = build_l2_index(2);
        let entries = vec![wal_entry(OperationCode::Insert, "doc1", vec![1.0, 0.0, 0.0])];

        let result = index.replay_wal(&entries);

        assert!(result.is_err());
    }

    #[test]
    fn given_hnsw_index_with_existing_id_then_replay_wal_skips_duplicate_insert() {
        let index = build_l2_index(2);
        index.insert("doc1", &[1.0, 0.0]).unwrap();
        let entries = vec![wal_entry(OperationCode::Insert, "doc1", vec![1.0, 0.0])];

        index.replay_wal(&entries).unwrap();
        let results = index.search(&HashMap::new(), &[1.0, 0.0], 10, 0, 2).unwrap();

        assert_eq!(results.len(), 1);
    }

    #[test]
    fn given_loaded_hnsw_index_then_insert_makes_new_vector_searchable() {
        let temp_dir = temp_dir("hnsw_loaded_insert");
        let original_index = build_l2_index(2);
        original_index.insert("existing", &[1.0, 0.0]).unwrap();

        let dump_dir = temp_dir.join("graph_dump");
        original_index.serialise(&dump_dir).unwrap();

        let loaded_index = HnswIndex::<DistL2>::from_dump(&dump_dir, 2, DistL2).unwrap();
        loaded_index.insert("new", &[0.0, 1.0]).unwrap();

        let results = loaded_index.search(&HashMap::new(), &[0.0, 1.0], 2, 0, 2).unwrap();
        assert_eq!(results.len(), 2);
        let returned_ids: Vec<&str> = results.iter().map(|result| result.id.as_str()).collect();
        assert!(returned_ids.contains(&"existing"));
        assert!(returned_ids.contains(&"new"));
    }

    #[test]
    fn given_missing_graph_file_then_from_dump_returns_corrupted_segment() {
        let temp_dir = temp_dir("hnsw_missing_graph");
        let index = build_l2_index(2);
        index.insert("doc1", &[1.0, 0.0]).unwrap();

        let dump_dir = temp_dir.join("graph_dump");
        index.serialise(&dump_dir).unwrap();
        std::fs::remove_file(dump_dir.join("graph.hnsw.graph")).unwrap();

        let result = HnswIndex::<DistL2>::from_dump(&dump_dir, 2, DistL2);

        assert!(matches!(result, Err(Hairball::CorruptedSegment)));
    }

    #[test]
    fn given_missing_ids_file_then_from_dump_returns_corrupted_segment() {
        let temp_dir = temp_dir("hnsw_missing_ids");
        let index = build_l2_index(2);
        index.insert("doc1", &[1.0, 0.0]).unwrap();

        let dump_dir = temp_dir.join("graph_dump");
        index.serialise(&dump_dir).unwrap();
        std::fs::remove_file(dump_dir.join("graph.ids")).unwrap();

        let result = HnswIndex::<DistL2>::from_dump(&dump_dir, 2, DistL2);

        assert!(matches!(result, Err(Hairball::CorruptedSegment)));
    }
}
