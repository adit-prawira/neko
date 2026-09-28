use crate::index::resource::ScoredVector;
use crate::shared;
use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

// Magic will ensure the authenticity of the file being read
// is related to neko and not just some random file
pub const SEGMENT_MAGIC: u32 = 0x6E656B6F;

// Version ensure that the format of the file is something that
// neko knows how to read
pub const SEGMENT_VERSION: u32 = 1;

#[derive(Clone, Copy, Debug)]
#[repr(C)]
pub struct SegmentHeader {
    pub magic: u32,
    pub version: u32,
    pub dim: u32,
    pub count: u64,
    pub metadata_length: u64,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct VectorMetadata {
    pub id: String,

    #[serde(default)]
    pub created_at: u64,

    #[serde(default)]
    pub deleted: bool,

    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub custom: String,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum Metric {
    L2 = 0,
    Cosine = 1,
    Dot = 2,
}

impl Metric {
    pub fn from_u8(metric: &u8) -> Result<Self> {
        match *metric {
            0 => Ok(Metric::L2),
            1 => Ok(Metric::Cosine),
            2 => Ok(Metric::Dot),
            _ => Err(Hairball::InvalidMetric),
        }
    }

    pub fn sort(&self, scored_vectors: &mut Vec<ScoredVector>) {
        if *self == Metric::Dot {
            for scored_vector in &mut *scored_vectors {
                scored_vector.score = -scored_vector.score;
            }
            scored_vectors.sort_by(|a, b| b.score.total_cmp(&a.score));
        } else {
            scored_vectors.sort_by(|a, b| a.score.total_cmp(&b.score));
        }
    }

    pub fn distance(&self, query: &[f32], vector: &[f32], dim: &u32) -> f32 {
        unsafe {
            match *self {
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
    use crate::index::resource::ScoredVector;

    #[test]
    fn given_from_u8_zero_then_returns_l2() {
        assert_eq!(Metric::from_u8(&0).unwrap(), Metric::L2);
    }

    #[test]
    fn given_from_u8_one_then_returns_cosine() {
        assert_eq!(Metric::from_u8(&1).unwrap(), Metric::Cosine);
    }

    #[test]
    fn given_from_u8_two_then_returns_dot() {
        assert_eq!(Metric::from_u8(&2).unwrap(), Metric::Dot);
    }

    #[test]
    fn given_from_u8_unknown_then_returns_invalid_metric() {
        assert_eq!(Metric::from_u8(&99).unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_l2_on_identical_vector_then_distance_is_zero() {
        let query = &[3.0_f32, 4.0, 0.0][..];
        let vector = &[3.0_f32, 4.0, 0.0][..];
        let distance = Metric::L2.distance(query, vector, &3);
        assert!(distance.abs() < 1e-6, "l2 of identical vectors must be ~0, got {}", distance);
    }

    #[test]
    fn given_dot_on_query_vector_then_distance_is_negated_for_min_heap_convention() {
        // Query and vector are identical (dot product = 1 + 4 + 0 = 5).
        // Metric::distance negates so it is stored in a min-heap as -5.
        // Independent check: dot product of [1,2,3] with [1,2,3] is 14.
        // We use [1,2,0] for a clean step value.
        let query = &[1.0_f32, 2.0, 0.0][..];
        let vector = &[1.0_f32, 2.0, 0.0][..];
        let distance = Metric::Dot.distance(query, vector, &3);
        assert!((distance - (-5.0)).abs() < 1e-5, "Dot polarity contract broken: expected -5.0, got {}", distance);
    }

    #[test]
    fn given_l2_sort_then_orders_ascending_by_score() {
        let mut scored = vec![
            ScoredVector { id: "far".into(), score: 9.0 },
            ScoredVector { id: "near".into(), score: 1.0 },
            ScoredVector { id: "mid".into(), score: 4.0 },
        ];
        Metric::L2.sort(&mut scored);
        let ordered_ids: Vec<&str> = scored.iter().map(|vector| vector.id.as_str()).collect();
        assert_eq!(ordered_ids, vec!["near", "mid", "far"]);
    }

    #[test]
    fn given_cosine_sort_then_orders_ascending_by_score() {
        let mut scored = vec![
            ScoredVector { id: "far".into(), score: 9.0 },
            ScoredVector { id: "near".into(), score: 1.0 },
            ScoredVector { id: "mid".into(), score: 4.0 },
        ];
        Metric::Cosine.sort(&mut scored);
        let ordered_ids: Vec<&str> = scored.iter().map(|vector| vector.id.as_str()).collect();
        assert_eq!(ordered_ids, vec!["near", "mid", "far"]);
    }

    #[test]
    fn given_dot_sort_then_flips_sign_and_orders_descending() {
        // Stored as negated dot so min-heap ascends. After sort, the original
        // (positive) score is restored and rows order by largest similarity.
        let mut scored = vec![
            ScoredVector { id: "small".into(), score: -2.0 }, // dot 2
            ScoredVector { id: "large".into(), score: -9.0 }, // dot 9
            ScoredVector { id: "mid".into(), score: -5.0 },   // dot 5
        ];
        Metric::Dot.sort(&mut scored);
        let ordered_ids: Vec<&str> = scored.iter().map(|vector| vector.id.as_str()).collect();
        assert_eq!(ordered_ids, vec!["large", "mid", "small"]);
        let restored: Vec<f32> = scored.iter().map(|vector| vector.score).collect();
        assert_eq!(restored, vec![9.0, 5.0, 2.0]);
    }
}

#[derive(Clone, Debug)]
pub struct SegmentMeta {
    pub directory: std::path::PathBuf,
    pub dim: u32,
    pub count: u64,
}
