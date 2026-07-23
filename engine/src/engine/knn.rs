use std::cmp::Ordering;
use std::collections::{BinaryHeap, HashMap};

use crate::segment::resource::Metric;
use crate::shared;
use crate::shared::hairball::Hairball;

#[derive(Clone, Debug)]
pub struct ScoredVector {
    pub id: String,
    pub score: f32,
}

impl PartialEq for ScoredVector {
    fn eq(&self, other: &Self) -> bool {
        self.score.total_cmp(&other.score) == Ordering::Equal
    }
}

impl Eq for ScoredVector {}

impl PartialOrd for ScoredVector {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Ord for ScoredVector {
    fn cmp(&self, other: &Self) -> Ordering {
        self.score.total_cmp(&other.score)
    }
}

#[derive(Clone, Debug)]
pub struct KNNSearchParams<'a> {
    pub query: &'a [f32],
    pub top_k: usize,
    pub metric: u8,
    pub dim: u32,
}

pub struct KNN;

impl KNN {
    pub fn search(vectors: &HashMap<String, Vec<f32>>, params: &KNNSearchParams) -> Result<Vec<ScoredVector>, Hairball> {
        if vectors.is_empty() || params.top_k == 0 {
            return Ok(Vec::new());
        }
        // ensure head_capacity is to be as small as possible
        let heap_capacity = params.top_k.min(vectors.len());

        // add additional slot to allow reallocation of vector on the (capacity'th + 1) push
        let mut heap = BinaryHeap::with_capacity(heap_capacity + 1);
        let metric = Self::resolve_metric(&params.metric)?;

        for (id, vector) in vectors {
            let distance = Self::resolve_distance(&metric, params.query, vector, &params.dim);
            heap.push(ScoredVector {
                id: id.to_string(),
                score: distance,
            });
            if heap.len() > heap_capacity {
                heap.pop();
            }
        }
        let mut scored_vectors: Vec<ScoredVector> = heap.into_vec();
        Self::resolve_sort(&metric, &mut scored_vectors);

        Ok(scored_vectors)
    }

    fn resolve_metric(metric: &u8) -> Result<Metric, Hairball> {
        match *metric {
            0 => Ok(Metric::L2),
            1 => Ok(Metric::Cosine),
            2 => Ok(Metric::Dot),
            _ => Err(Hairball::InvalidMetric),
        }
    }
    fn resolve_sort(metric: &Metric, scored_vectors: &mut Vec<ScoredVector>) {
        if *metric == Metric::Dot {
            for scored_vector in &mut *scored_vectors {
                scored_vector.score = -scored_vector.score;
            }
            scored_vectors.sort_by(|a, b| b.score.total_cmp(&a.score));
        } else {
            scored_vectors.sort_by(|a, b| a.score.total_cmp(&b.score));
        }
    }

    fn resolve_distance(metric: &Metric, query: &[f32], vector: &[f32], dim: &u32) -> f32 {
        unsafe {
            match *metric {
                Metric::L2 => shared::l2_distance(query.as_ptr(), vector.as_ptr(), *dim),
                Metric::Cosine => shared::cosine_distance(query.as_ptr(), vector.as_ptr(), *dim),
                Metric::Dot => -shared::dot_product(query.as_ptr(), vector.as_ptr(), *dim),
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn make_vectors(entries: Vec<(&str, Vec<f32>)>) -> HashMap<String, Vec<f32>> {
        entries.into_iter().map(|(id, vec)| (id.to_string(), vec)).collect()
    }

    #[test]
    fn given_empty_vectors_then_search_returns_empty() {
        let vectors: HashMap<String, Vec<f32>> = HashMap::new();
        let params = KNNSearchParams {
            query: &[1.0, 0.0, 0.0],
            top_k: 3,
            metric: 0,
            dim: 3,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert!(results.is_empty());
    }

    #[test]
    fn given_top_k_zero_then_search_returns_empty() {
        let vectors = make_vectors(vec![("a", vec![1.0, 0.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[1.0, 0.0, 0.0],
            top_k: 0,
            metric: 0,
            dim: 3,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert!(results.is_empty());
    }

    #[test]
    fn given_invalid_metric_then_search_returns_error() {
        let vectors = make_vectors(vec![("a", vec![1.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[1.0, 0.0],
            top_k: 1,
            metric: 99,
            dim: 2,
        };
        let result = KNN::search(&vectors, &params);
        assert!(result.is_err());
    }

    #[test]
    fn given_l2_metric_then_closest_vector_has_smallest_score() {
        let vectors = make_vectors(vec![("nearby", vec![2.0, 0.0, 0.0]), ("far", vec![10.0, 0.0, 0.0]), ("farthest", vec![100.0, 0.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[1.0, 0.0, 0.0],
            top_k: 3,
            metric: 0,
            dim: 3,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 3);
        assert!(results[0].score < results[1].score);
        assert!(results[1].score < results[2].score);
    }

    #[test]
    fn given_l2_metric_with_identical_vector_then_score_near_zero() {
        let vectors = make_vectors(vec![("exact", vec![7.0, 8.0, 9.0]), ("other", vec![10.0, 0.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[7.0, 8.0, 9.0],
            top_k: 2,
            metric: 0,
            dim: 3,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 2);
        assert_eq!(results[0].id, "exact");
        assert!(results[0].score < 1e-6);
    }

    #[test]
    fn given_top_k_smaller_than_total_then_returns_exactly_k() {
        let vectors = make_vectors(vec![("a", vec![2.0, 0.0]), ("b", vec![9.0, 0.0]), ("c", vec![5.0, 0.0]), ("d", vec![1.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[0.0, 0.0],
            top_k: 2,
            metric: 0,
            dim: 2,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 2);
        assert!(results[0].score <= results[1].score);
    }

    #[test]
    fn given_top_k_larger_than_total_then_returns_all() {
        let vectors = make_vectors(vec![("x", vec![3.0, 0.0]), ("y", vec![7.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[0.0, 0.0],
            top_k: 10,
            metric: 0,
            dim: 2,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 2);
    }

    #[test]
    fn given_cosine_metric_then_orthogonal_has_larger_score_than_same_direction() {
        let vectors = make_vectors(vec![("same", vec![1.0, 0.0]), ("orthogonal", vec![0.0, 1.0])]);
        let params = KNNSearchParams {
            query: &[1.0, 0.0],
            top_k: 2,
            metric: 1,
            dim: 2,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 2);
        assert_eq!(results[0].id, "same");
        assert!(results[0].score < results[1].score);
    }

    #[test]
    fn given_dot_metric_then_larger_dot_product_appears_first() {
        let vectors = make_vectors(vec![("medium", vec![2.0, 0.0, 0.0]), ("largest", vec![5.0, 0.0, 0.0]), ("zero", vec![0.0, 1.0, 0.0])]);
        let params = KNNSearchParams {
            query: &[1.0, 0.0, 0.0],
            top_k: 3,
            metric: 2,
            dim: 3,
        };
        let results = KNN::search(&vectors, &params).unwrap();
        assert_eq!(results.len(), 3);
        assert_eq!(results[0].id, "largest");
        assert_eq!(results[1].id, "medium");
        assert_eq!(results[2].id, "zero");
        assert!(results[0].score > results[1].score);
        assert!(results[1].score > results[2].score);
    }

    #[test]
    fn given_scored_vector_ordering_then_higher_score_is_greater_for_max_heap() {
        let best = ScoredVector {
            id: "best".to_string(),
            score: 0.5,
        };
        let worst = ScoredVector {
            id: "worst".to_string(),
            score: 9.9,
        };
        assert!(worst > best);
        assert!(best < worst);
    }
}
