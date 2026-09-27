use std::cmp::Ordering;
use std::collections::HashMap;
use std::path::Path;

use serde::{Deserialize, Serialize};

use crate::shared::hairball::Hairball;
use crate::shared::results::Result;
use crate::wal::resource::WalEntry;

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

pub struct InputVector {
    pub id: String,
    pub vector: Vec<f32>,
}

pub trait Index: Send + Sync {
    fn insert(&self, id: &str, vector: &[f32]) -> Result<()>;
    fn insert_batch(&self, items: &[InputVector]) -> Result<()>;
    fn search(&self, vectors: &HashMap<String, Vec<f32>>, query: &[f32], top_k: usize, metric: u8, dim: u32) -> Result<Vec<ScoredVector>>;

    fn prepare_vector_for_insert(&self, vector: &[f32]) -> Result<Vec<f32>>;
    fn is_support_delete(&self) -> bool;
    fn is_support_upsert(&self) -> bool;
    fn rebuild_from(&self, vectors: &HashMap<String, Vec<f32>>) -> Result<()>;
    fn serialise(&self, _path: &Path) -> Result<()> {
        Ok(())
    }
    fn replay_wal(&self, _entries: &[WalEntry]) -> Result<()> {
        Err(Hairball::InternalError)
    }
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize, Deserialize)]
pub enum IndexSpec {
    Brute,
    Hnsw {
        // max connections per node per layer
        // this will caps how many neighbours each HNSW node keeps per layer,
        // trading graph density against memory and insert cost
        max_connections: usize,

        // sets the candidate-list size used while inserting each new node,
        // trading graph-build quality against insert needed
        ef_construction: usize,

        // sets the candidate-list size used while answering search query,
        // trading query recall against query latency
        ef_search: usize,
    },
}

impl IndexSpec {
    pub fn index_type(&self) -> u8 {
        match self {
            IndexSpec::Brute => INDEX_TYPE_BRUTE,
            IndexSpec::Hnsw { .. } => INDEX_TYPE_HNSW,
        }
    }

    pub fn is_hnsw(&self) -> bool {
        matches!(self, IndexSpec::Hnsw { .. })
    }

    pub fn validate(&self) -> Result<()> {
        match self {
            IndexSpec::Brute => Ok(()),
            IndexSpec::Hnsw {
                max_connections,
                ef_construction,
                ef_search,
            } => {
                if *max_connections == 0 {
                    return Err(Hairball::InvalidMetric);
                }
                if *ef_construction == 0 {
                    return Err(Hairball::InvalidMetric);
                }

                if *ef_search == 0 {
                    return Err(Hairball::InvalidMetric);
                }
                Ok(())
            }
        }
    }

    pub fn from_manifest(index_type: u8, hnsw_max_connections: u16, hnsw_ef_construction: u16, hnsw_ef_search: u16) -> IndexSpec {
        match index_type {
            INDEX_TYPE_BRUTE => IndexSpec::Brute,
            _ => IndexSpec::Hnsw {
                max_connections: if hnsw_max_connections == 0 {
                    DEFAULT_MAX_CONNECTIONS
                } else {
                    hnsw_max_connections as usize
                },
                ef_construction: if hnsw_ef_construction == 0 {
                    DEFAULT_EF_CONSTRUCTION
                } else {
                    hnsw_ef_construction as usize
                },
                ef_search: if hnsw_ef_search == 0 { DEFAULT_EF_SEARCH } else { hnsw_ef_search as usize },
            },
        }
    }

    pub fn brute_default() -> Self {
        IndexSpec::Brute
    }

    pub fn hnsw_default() -> Self {
        IndexSpec::Hnsw {
            max_connections: DEFAULT_MAX_CONNECTIONS,
            ef_construction: DEFAULT_EF_CONSTRUCTION,
            ef_search: DEFAULT_EF_SEARCH,
        }
    }
}

pub const INDEX_TYPE_BRUTE: u8 = 0;
pub const INDEX_TYPE_HNSW: u8 = 1;

pub const DEFAULT_MAX_CONNECTIONS: usize = 16;
pub const DEFAULT_EF_CONSTRUCTION: usize = 200;
pub const DEFAULT_EF_SEARCH: usize = 100;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn given_brute_spec_then_validate_returns_ok() {
        let spec = IndexSpec::Brute;
        assert!(spec.validate().is_ok());
    }

    #[test]
    fn given_hnsw_spec_with_zero_max_connections_then_validate_returns_invalid_metric() {
        let spec = IndexSpec::Hnsw {
            max_connections: 0,
            ef_construction: DEFAULT_EF_CONSTRUCTION,
            ef_search: DEFAULT_EF_SEARCH,
        };
        assert_eq!(spec.validate().unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_hnsw_spec_with_zero_ef_construction_then_validate_returns_invalid_metric() {
        let spec = IndexSpec::Hnsw {
            max_connections: DEFAULT_MAX_CONNECTIONS,
            ef_construction: 0,
            ef_search: DEFAULT_EF_SEARCH,
        };
        assert_eq!(spec.validate().unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_hnsw_spec_with_zero_ef_search_then_validate_returns_invalid_metric() {
        let spec = IndexSpec::Hnsw {
            max_connections: DEFAULT_MAX_CONNECTIONS,
            ef_construction: DEFAULT_EF_CONSTRUCTION,
            ef_search: 0,
        };
        assert_eq!(spec.validate().unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_hnsw_spec_with_default_values_then_validate_returns_ok() {
        let spec = IndexSpec::hnsw_default();
        assert!(spec.validate().is_ok());
    }

    #[test]
    fn given_hnsw_default_then_returns_expected_constant_values() {
        let spec = IndexSpec::hnsw_default();
        match spec {
            IndexSpec::Hnsw {
                max_connections,
                ef_construction,
                ef_search,
            } => {
                assert_eq!(max_connections, DEFAULT_MAX_CONNECTIONS);
                assert_eq!(ef_construction, DEFAULT_EF_CONSTRUCTION);
                assert_eq!(ef_search, DEFAULT_EF_SEARCH);
            }
            IndexSpec::Brute => panic!("hnsw_default should return Hnsw variant"),
        }
    }

    #[test]
    fn given_brute_default_then_returns_brute_variant() {
        assert_eq!(IndexSpec::brute_default(), IndexSpec::Brute);
    }

    #[test]
    fn given_brute_variant_then_index_type_returns_brute_code() {
        assert_eq!(IndexSpec::Brute.index_type(), INDEX_TYPE_BRUTE);
    }

    #[test]
    fn given_hnsw_variant_then_index_type_returns_hnsw_code() {
        assert_eq!(IndexSpec::hnsw_default().index_type(), INDEX_TYPE_HNSW);
    }

    #[test]
    fn given_brute_variant_then_is_hnsw_returns_false() {
        assert!(!IndexSpec::Brute.is_hnsw());
    }

    #[test]
    fn given_hnsw_variant_then_is_hnsw_returns_true() {
        assert!(IndexSpec::hnsw_default().is_hnsw());
    }

    #[test]
    fn given_from_manifest_with_brute_index_type_then_returns_brute_variant() {
        let spec = IndexSpec::from_manifest(INDEX_TYPE_BRUTE, 0, 0, 0);
        assert_eq!(spec, IndexSpec::Brute);
    }

    #[test]
    fn given_from_manifest_with_hnsw_index_type_and_zero_tuning_fields_then_uses_defaults() {
        let spec = IndexSpec::from_manifest(INDEX_TYPE_HNSW, 0, 0, 0);
        match spec {
            IndexSpec::Hnsw {
                max_connections,
                ef_construction,
                ef_search,
            } => {
                assert_eq!(max_connections, DEFAULT_MAX_CONNECTIONS);
                assert_eq!(ef_construction, DEFAULT_EF_CONSTRUCTION);
                assert_eq!(ef_search, DEFAULT_EF_SEARCH);
            }
            IndexSpec::Brute => panic!("expected Hnsw variant"),
        }
    }

    #[test]
    fn given_from_manifest_with_hnsw_index_type_and_explicit_tuning_fields_then_preserves_values() {
        let spec = IndexSpec::from_manifest(INDEX_TYPE_HNSW, 32, 400, 200);
        match spec {
            IndexSpec::Hnsw {
                max_connections,
                ef_construction,
                ef_search,
            } => {
                assert_eq!(max_connections, 32);
                assert_eq!(ef_construction, 400);
                assert_eq!(ef_search, 200);
            }
            IndexSpec::Brute => panic!("expected Hnsw variant"),
        }
    }

    #[test]
    fn given_from_manifest_with_unknown_index_type_byte_then_falls_through_to_hnsw_default() {
        // Index type bytes > 1 fall through to Hnsw variant. The validator is the
        // gate that rejects invalid persistence; from_manifest is just a converter.
        let spec = IndexSpec::from_manifest(7, 0, 0, 0);
        assert!(spec.is_hnsw());
    }
}
