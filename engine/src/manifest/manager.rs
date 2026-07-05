use std::fs;
use std::path::{Path, PathBuf};

use crate::shared::results::Result;

use super::resource::Manifest;

pub struct ManifestManager;

impl ManifestManager {
    pub fn add_segment(manifest_path: &Path, name: &str, dim: u32, metric: u8, segment_directory: &str) -> Result<()> {
        let mut manifest = if manifest_path.exists() {
            serde_json::from_str(&fs::read_to_string(manifest_path)?)?
        } else {
            Manifest {
                version: 1,
                collection_name: name.to_string(),
                dim,
                metric,
                segments: Vec::new(),
            }
        };

        manifest.segments.push(segment_directory.to_string());
        manifest.segments.sort();

        let tmp = manifest_path.with_extension("tmp");
        fs::write(&tmp, &serde_json::to_string_pretty(&manifest)?)?;
        fs::rename(&tmp, manifest_path)?;

        Ok(())
    }

    pub fn load_segments(manifest_path: &Path) -> Result<Vec<PathBuf>> {
        let manifest = if manifest_path.exists() {
            serde_json::from_str(&fs::read_to_string(manifest_path)?)?
        } else {
            Manifest {
                version: 1,
                collection_name: String::new(),
                dim: 0,
                metric: 0,
                segments: Vec::new(),
            }
        };

        let base = manifest_path.parent().unwrap_or(Path::new("."));
        let segment_files = manifest.segments.iter().map(|segment| base.join(segment)).collect();
        Ok(segment_files)
    }
}
