use std::collections::HashMap;

use crate::segment::resource::Metric;
use crate::shared::results::Result;

use super::knn::{KNN, KNNSearchParams};
use super::resource::{Index, InputVector, ScoredVector};

pub struct BruteIndex {
    metric: u8,
}

impl BruteIndex {
    pub fn new(metric: u8) -> Self {
        Self { metric }
    }

    fn normalize_for_cosine(vector: &[f32]) -> Vec<f32> {
        let squared: f32 = vector.iter().map(|x| x * x).sum();
        if squared <= 1e-18 {
            return vector.to_vec();
        }

        let inverse_normal = 1.0 / squared.sqrt();
        vector.iter().map(|component| component * inverse_normal).collect()
    }
}

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

    fn prepare_vector_for_insert(&self, vector: &[f32]) -> Result<Vec<f32>> {
        if self.metric == Metric::Cosine as u8 {
            return Ok(BruteIndex::normalize_for_cosine(vector));
        }

        Ok(vector.to_vec())
    }

    fn is_support_delete(&self) -> bool {
        true
    }

    fn is_support_upsert(&self) -> bool {
        true
    }

    fn rebuild_from(&self, _vectors: &HashMap<String, Vec<f32>>) -> Result<()> {
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
        let brute = BruteIndex::new(0);
        let results = brute.search(&vectors, &query, 1, 0, 2).unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "near");
    }

    #[test]
    fn given_brute_index_then_insert_returns_ok_without_persisting() {
        let vectors: HashMap<String, Vec<f32>> = HashMap::new();
        let brute = BruteIndex::new(0);
        brute.insert("anything", &[0.0]).unwrap();
        let query = [0.0];
        let results = brute.search(&vectors, &query, 0, 0, 1).unwrap();
        assert!(results.is_empty());
    }

    #[test]
    fn given_brute_index_with_metric_cosine_then_prepare_vector_returns_unit_normalized() {
        let brute = BruteIndex::new(Metric::Cosine as u8);
        let prepared = brute.prepare_vector_for_insert(&[3.0, 4.0]).unwrap();
        assert_eq!(prepared, vec![0.6, 0.8]);
    }

    #[test]
    fn given_brute_index_with_metric_l2_then_prepare_vector_returns_vector_unchanged() {
        let brute = BruteIndex::new(Metric::L2 as u8);
        let prepared = brute.prepare_vector_for_insert(&[3.0, 4.0]).unwrap();
        assert_eq!(prepared, vec![3.0, 4.0]);
    }

    #[test]
    fn given_brute_index_with_metric_dot_then_prepare_vector_returns_vector_unchanged() {
        let brute = BruteIndex::new(Metric::Dot as u8);
        let prepared = brute.prepare_vector_for_insert(&[3.0, 4.0]).unwrap();
        assert_eq!(prepared, vec![3.0, 4.0]);
    }

    #[test]
    fn given_brute_index_with_metric_cosine_and_zero_vector_then_prepare_vector_returns_zero_vector() {
        let brute = BruteIndex::new(Metric::Cosine as u8);
        let zero_vector = vec![0.0_f32, 0.0, 0.0];
        let prepared = brute.prepare_vector_for_insert(&zero_vector).unwrap();
        assert_eq!(prepared, zero_vector);
        for component in &prepared {
            assert!(component.is_finite());
        }
    }

    #[test]
    fn given_brute_index_then_is_support_delete_returns_true() {
        let brute = BruteIndex::new(0);
        assert!(brute.is_support_delete());
    }

    #[test]
    fn given_brute_index_then_is_support_upsert_returns_true() {
        let brute = BruteIndex::new(0);
        assert!(brute.is_support_upsert());
    }

    #[test]
    fn given_brute_index_then_rebuild_from_succeeds_and_does_not_mutate_state() {
        let brute = BruteIndex::new(0);
        let vectors: HashMap<String, Vec<f32>> = vec![("a".to_string(), vec![1.0, 2.0])].into_iter().collect();
        let result = brute.rebuild_from(&vectors);
        assert!(result.is_ok());
    }
}
