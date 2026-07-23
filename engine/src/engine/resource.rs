use std::collections::HashMap;
use std::sync::Mutex;

// Requirement wherer dim <= 4096
pub const MAX_DIM: u32 = 4096;

/*
 * The other name for collection in cat themed terminology
 * */
pub struct Clowder {
    pub name: String,
    pub dim: u32,
    pub metric: u8,
    pub model: Option<String>,
    pub vectors: Mutex<HashMap<String, Vec<f32>>>,
}
