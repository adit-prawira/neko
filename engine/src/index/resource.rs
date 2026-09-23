use std::collections::HashMap;

use crate::engine::knn::ScoredVector;
use crate::shared::results::Result;

pub struct InputVector {
    pub id: String,
    pub vector: Vec<f32>,
}

pub trait Index: Send + Sync {
    fn insert(&self, id: &str, vector: &[f32]) -> Result<()>;
    fn insert_batch(&self, items: &[InputVector]) -> Result<()>;
    fn search(&self, vectors: &HashMap<String, Vec<f32>>, query: &[f32], top_k: usize, metric: u8, dim: u32) -> Result<Vec<ScoredVector>>;
}
