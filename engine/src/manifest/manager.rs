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

#[cfg(test)]
mod tests {
    use std::fs;

    use super::super::resource::Manifest;
    use super::*;

    #[test]
    fn given_no_existing_manifest_then_add_segment_creates_manifest_with_segment() {
        let dir = std::env::temp_dir().join("neko_test_manifest_create");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        ManifestManager::add_segment(&manifest_path, "my_collection", 128, 0, "seg_001").unwrap();

        assert!(manifest_path.exists());
        let content = fs::read_to_string(&manifest_path).unwrap();
        let manifest: Manifest = serde_json::from_str(&content).unwrap();
        assert_eq!(manifest.collection_name, "my_collection");
        assert_eq!(manifest.dim, 128);
        assert_eq!(manifest.metric, 0);
        assert_eq!(manifest.segments, vec!["seg_001".to_string()]);
    }

    #[test]
    fn given_existing_manifest_then_add_segment_appends_and_sorts_segments() {
        let dir = std::env::temp_dir().join("neko_test_manifest_append");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        ManifestManager::add_segment(&manifest_path, "col", 64, 1, "seg_b").unwrap();
        ManifestManager::add_segment(&manifest_path, "col", 64, 1, "seg_a").unwrap();

        let manifest: Manifest = serde_json::from_str(&fs::read_to_string(&manifest_path).unwrap()).unwrap();
        assert_eq!(manifest.segments, vec!["seg_a".to_string(), "seg_b".to_string()]);
    }

    #[test]
    fn given_no_existing_manifest_then_load_segments_returns_empty_list() {
        let dir = std::env::temp_dir().join("neko_test_manifest_load_empty");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let segments = ManifestManager::load_segments(&dir.join("nonexistent.json")).unwrap();
        assert!(segments.is_empty());
    }

    #[test]
    fn given_manifest_with_segments_then_load_segments_returns_paths_joined_with_manifest_base() {
        let dir = std::env::temp_dir().join("neko_test_manifest_load");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        ManifestManager::add_segment(&manifest_path, "col", 384, 1, "seg_x").unwrap();
        ManifestManager::add_segment(&manifest_path, "col", 384, 1, "seg_y").unwrap();

        let segments = ManifestManager::load_segments(&manifest_path).unwrap();
        assert_eq!(segments.len(), 2);
        assert_eq!(segments[0], dir.join("seg_x"));
        assert_eq!(segments[1], dir.join("seg_y"));
    }
}
