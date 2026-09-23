use crate::engine::knn::{KNN, KNNSearchParams, ScoredVector};
use crate::shared::results::Result;

use super::resource::{Index, InputVector};

pub struct BruteIndex;

impl Index for BruteIndex {
    fn search(&self, vectors: &std::collections::HashMap<String, Vec<f32>>, query: &[f32], top_k: usize, metric: u8, dim: u32) -> Result<Vec<ScoredVector>> {
        let params = KNNSearchParams { query, top_k, metric, dim };
        KNN::search(vectors, &params)
    }

    fn insert(&self, _id: &str, _vector: &[f32]) -> Result<()> {
        Ok(())
    }

    fn insert_batch(&self, _items: &[InputVector]) -> Result<()> {
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use super::super::resource::Index;
    use super::*;

    #[test]
    fn given_brute_index_then_search_delegates_to_knn_returns_nearest() {
        let vectors: HashMap<String, Vec<f32>> = vec![("near".to_string(), vec![1.0, 0.0]), ("far".to_string(), vec![9.0, 0.0])].into_iter().collect();
        let query = [0.0, 0.0];
        let results = BruteIndex.search(&vectors, &query, 1, 0, 2).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "near");
    }

    #[test]
    fn given_brute_index_then_insert_returns_ok_without_persisting() {
        let vectors: HashMap<String, Vec<f32>> = HashMap::new();
        BruteIndex.insert("anything", &[0.0]).unwrap();
        let query = [0.0];
        let results = BruteIndex.search(&vectors, &query, 0, 0, 1).unwrap();
        assert!(results.is_empty());
    }
}
