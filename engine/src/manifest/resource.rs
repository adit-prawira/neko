use serde::{Deserialize, Serialize};

#[derive(Clone, Serialize, Deserialize, Debug)]
pub struct Manifest {
    pub version: u32,
    pub collection_name: String,
    pub dim: u32,
    pub metric: u8,
    pub segments: Vec<String>,
}
