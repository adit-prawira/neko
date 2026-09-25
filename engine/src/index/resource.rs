use std::cmp::Ordering;
use std::collections::HashMap;
use std::path::Path;

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
