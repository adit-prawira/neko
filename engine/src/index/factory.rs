use std::path::Path;
use std::sync::Arc;

use hnsw_rs::prelude::{DistCosine, DistDot, DistL2};

use crate::shared::hairball::Hairball;
use crate::shared::results::Result;
use crate::wal::resource::{OperationCode, WalEntry};

use super::brute::BruteIndex;
use super::hnsw::HnswIndex;
use super::resource::{Index, IndexSpec};

pub struct IndexFactory;

impl IndexFactory {
    pub fn load_or_build(spec: IndexSpec, dim: Option<u32>, metric: Option<u8>, directory: &Path, wal_entries: &[&WalEntry]) -> Result<Arc<dyn Index>> {
        let Some(metric) = metric else {
            return Err(Hairball::InvalidMetric);
        };

        let Some(dim) = dim else {
            return Err(Hairball::DimMismatch);
        };

        match spec {
            IndexSpec::Brute => IndexFactory::build_brute_index(metric),
            IndexSpec::Hnsw { .. } => {
                let dim_usize = dim as usize;
                let has_delete = wal_entries.iter().any(|entry| matches!(entry.operation_code, OperationCode::Delete));
                let dump_directory = directory.join("graph_dump");
                let is_dump_file_exists = dump_directory.join("graph.hnsw.graph").exists();
                if !has_delete && is_dump_file_exists {
                    return IndexFactory::build_hnsw_index_from_dump(dim_usize, metric, &dump_directory, &spec);
                }

                IndexFactory::build_hnsw_index(dim_usize, metric, &spec)
            }
        }
    }

    fn build_brute_index(metric: u8) -> Result<Arc<dyn Index>> {
        Ok(Arc::new(BruteIndex::new(metric)))
    }

    fn build_hnsw_index(dim: usize, metric: u8, spec: &IndexSpec) -> Result<Arc<dyn Index>> {
        match metric {
            0 => Ok(Arc::new(HnswIndex::<DistL2>::new(dim, DistL2, spec)?)),
            1 => Ok(Arc::new(HnswIndex::<DistCosine>::new(dim, DistCosine, spec)?)),
            2 => Ok(Arc::new(HnswIndex::<DistDot>::new(dim, DistDot, spec)?)),
            _ => Err(Hairball::InvalidMetric),
        }
    }

    fn build_hnsw_index_from_dump(dim: usize, metric: u8, dump_directory: &Path, spec: &IndexSpec) -> Result<Arc<dyn Index>> {
        match metric {
            0 => Ok(Arc::new(HnswIndex::<DistL2>::from_dump(dump_directory, dim, DistL2, spec)?)),
            1 => Ok(Arc::new(HnswIndex::<DistCosine>::from_dump(dump_directory, dim, DistCosine, spec)?)),
            2 => Ok(Arc::new(HnswIndex::<DistDot>::from_dump(dump_directory, dim, DistDot, spec)?)),
            _ => Err(Hairball::InvalidMetric),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::collections::HashMap;

    use hnsw_rs::prelude::DistL2;

    use super::*;
    use crate::segment::resource::VectorMetadata;

    fn temp_dir(name: &str) -> std::path::PathBuf {
        let dir = std::env::temp_dir().join(format!("neko_test_factory_{}", name));
        let _ = std::fs::remove_dir_all(&dir);
        dir
    }

    #[test]
    fn given_factory_build_with_index_type_brute_and_missing_metric_then_returns_invalid_metric() {
        let result = IndexFactory::load_or_build(IndexSpec::Brute, None, None, Path::new("."), &[]);
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }

    #[test]
    fn given_factory_build_with_index_type_brute_and_metric_none_then_returns_invalid_metric() {
        let result = IndexFactory::load_or_build(IndexSpec::Brute, Some(0), None, Path::new("."), &[]);
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }

    #[test]
    fn given_factory_build_with_index_type_brute_and_valid_metric_then_succeeds() {
        let result = IndexFactory::load_or_build(IndexSpec::Brute, Some(0), Some(0), Path::new("."), &[]);
        assert!(result.is_ok());
        let arc = result.unwrap();
        arc.search(&std::collections::HashMap::new(), &[0.0], 0, 0, 1).unwrap();
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_missing_dim_then_returns_dim_mismatch() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), None, Some(0), Path::new("."), &[]);
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_missing_metric_then_returns_invalid_metric() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(384), None, Path::new("."), &[]);
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_l2_then_succeeds() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(384), Some(0), Path::new("."), &[]);
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_cosine_then_succeeds() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(384), Some(1), Path::new("."), &[]);
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_dot_then_succeeds() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(384), Some(2), Path::new("."), &[]);
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_unsupported_metric_then_returns_invalid_metric() {
        let result = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(384), Some(3), Path::new("."), &[]);
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }

    #[test]
    fn given_hnsw_dump_exists_and_wal_has_no_deletes_then_load_or_build_loads_dump() {
        let temp_dir = temp_dir("hnsw_load_dump");
        let index = HnswIndex::<DistL2>::new(2, DistL2, &IndexSpec::hnsw_default()).unwrap();
        index.insert("doc1", &[1.0, 0.0]).unwrap();
        let dump_dir = temp_dir.join("graph_dump");
        index.serialise(&dump_dir).unwrap();

        let loaded = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(2), Some(0), &temp_dir, &[]).unwrap();
        let results = loaded.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();

        assert_eq!(results.len(), 1);
        assert_eq!(results[0].id, "doc1");
    }

    #[test]
    fn given_hnsw_dump_exists_and_wal_has_delete_then_load_or_build_falls_back_to_fresh_index() {
        let temp_dir = temp_dir("hnsw_skip_dump_on_delete");
        let index = HnswIndex::<DistL2>::new(2, DistL2, &IndexSpec::hnsw_default()).unwrap();
        index.insert("doc1", &[1.0, 0.0]).unwrap();
        let dump_dir = temp_dir.join("graph_dump");
        index.serialise(&dump_dir).unwrap();

        let wal_entries = vec![WalEntry {
            operation_code: OperationCode::Delete,
            collection: "col".to_string(),
            id: "doc1".to_string(),
            vector: Vec::new(),
            metadata: VectorMetadata {
                id: "doc1".to_string(),
                created_at: 0,
                deleted: false,
                custom: String::new(),
            },
        }];
        let references: Vec<&WalEntry> = wal_entries.iter().collect();

        let loaded = IndexFactory::load_or_build(IndexSpec::hnsw_default(), Some(2), Some(0), &temp_dir, &references).unwrap();
        let results = loaded.search(&HashMap::new(), &[1.0, 0.0], 1, 0, 2).unwrap();

        assert!(results.is_empty());
    }
}
