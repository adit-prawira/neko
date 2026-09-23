use std::sync::Arc;

use hnsw_rs::prelude::{DistCosine, DistDot, DistL2};

use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::brute::BruteIndex;
use super::hnsw::HnswIndex;
use super::resource::Index;

pub const INDEX_TYPE_BRUTE: u8 = 0;
pub const INDEX_TYPE_HNSW: u8 = 1;

pub struct IndexFactory;

impl IndexFactory {
    pub fn build(index_type: u8, dim: Option<u32>, metric: Option<u8>) -> Result<Arc<dyn Index>> {
        if index_type == INDEX_TYPE_BRUTE {
            return IndexFactory::build_brute_index();
        };

        let Some(dim) = dim else {
            return Err(Hairball::DimMismatch);
        };
        let Some(metric) = metric else {
            return Err(Hairball::InvalidMetric);
        };
        IndexFactory::build_hnsw_index(dim, metric)
    }

    fn build_brute_index() -> Result<Arc<dyn Index>> {
        Ok(Arc::new(BruteIndex))
    }

    fn build_hnsw_index(dim: u32, metric: u8) -> Result<Arc<dyn Index>> {
        let dim_usize = dim as usize;
        match metric {
            0 => Ok(Arc::new(HnswIndex::<DistL2>::new(dim_usize, DistL2)?)),
            1 => Ok(Arc::new(HnswIndex::<DistCosine>::new(dim_usize, DistCosine)?)),
            2 => Ok(Arc::new(HnswIndex::<DistDot>::new(dim_usize, DistDot)?)),
            _ => Err(Hairball::InvalidMetric),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::index::resource::Index;

    #[test]
    fn given_factory_build_with_index_type_brute_then_succeeds_without_dim_or_metric() {
        let result = IndexFactory::build(INDEX_TYPE_BRUTE, None, None);
        assert!(result.is_ok());
        let arc = result.unwrap();
        arc.search(&std::collections::HashMap::new(), &[0.0], 0, 0, 1).unwrap();
    }

    #[test]
    fn given_factory_build_with_index_type_brute_then_ignores_dim_and_metric() {
        let result = IndexFactory::build(INDEX_TYPE_BRUTE, Some(0), None);
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_missing_dim_then_returns_dim_mismatch() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, None, Some(0));
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_missing_metric_then_returns_invalid_metric() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, Some(384), None);
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_l2_then_succeeds() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, Some(384), Some(0));
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_cosine_then_succeeds() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, Some(384), Some(1));
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_metric_dot_then_succeeds() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, Some(384), Some(2));
        assert!(result.is_ok());
    }

    #[test]
    fn given_factory_build_with_index_type_hnsw_and_unsupported_metric_then_returns_invalid_metric() {
        let result = IndexFactory::build(INDEX_TYPE_HNSW, Some(384), Some(3));
        assert!(matches!(result, Err(Hairball::InvalidMetric)));
    }
}
