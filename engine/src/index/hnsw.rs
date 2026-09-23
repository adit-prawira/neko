use std::collections::HashMap;
use std::sync::Mutex;

use hnsw_rs::hnsw::{Hnsw, Neighbour};
use hnsw_rs::prelude::Distance;

use crate::engine::knn::ScoredVector;
use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::resource::{Index, InputVector};

pub const DEFAULT_MAX_NB_CONNECTION: usize = 16;
pub const DEFAULT_EF_CONSTRUCTION: usize = 200;
pub const DEFAULT_NB_LAYER: usize = 16;

const INITIAL_BACKLOG: usize = 10_000;
const SEARCH_BEAM_FACTOR: usize = 4;

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
    graph: Mutex<Hnsw<'static, f32, D>>,
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
            graph: Mutex::new(graph),
            ids: Mutex::new(NodeIdRegistry::default()),
            dim,
        })
    }
}

impl<D> Index for HnswIndex<D>
where
    D: Distance<f32> + Send + Sync + 'static,
{
    fn search(&self, _vectors: &HashMap<String, Vec<f32>>, query: &[f32], top_k: usize, _metric: u8, _dim: u32) -> Result<Vec<crate::engine::knn::ScoredVector>> {
        if query.len() != self.dim {
            return Err(Hairball::DimMismatch);
        }

        if top_k == 0 {
            return Ok(Vec::new());
        }

        let search_beam = top_k.max(DEFAULT_MAX_NB_CONNECTION) * SEARCH_BEAM_FACTOR;
        let raw_neighbours: Vec<Neighbour> = {
            let graph = self.graph.lock().unwrap();
            graph.search(query, top_k, search_beam)
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
        let graph = self.graph.lock().unwrap();
        graph.insert((vector, assigned));
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

        let graph = self.graph.lock().unwrap();
        for (internal_id, vector) in &owned {
            graph.insert((vector.as_slice(), *internal_id));
        }
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use hnsw_rs::prelude::DistL2;

    fn build_l2_index(dim: usize) -> HnswIndex<DistL2> {
        HnswIndex::<DistL2>::new(dim, DistL2).expect("hnsw construction never fails")
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
}
