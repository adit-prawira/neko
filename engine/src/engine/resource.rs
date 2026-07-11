use std::path::PathBuf;
use std::sync::{OnceLock, RwLock};

pub struct Engine {
    pub data_directory: PathBuf,
}

pub static ENGINE: OnceLock<RwLock<Engine>> = OnceLock::new();
