use std::fs;
use std::path::{Path, PathBuf};

use crate::shared::results::Result;

use super::resource::Manifest;

pub struct ManifestManager;

impl ManifestManager {
    pub fn add_segment(manifest_path: &Path, name: &str, dim: u32, metric: u8, segment_directory: &str) -> Result<()> {
        let mut manifest = if manifest_path.exists() {
            let json_string = fs::read_to_string(manifest_path)?;
            serde_json::from_str(&json_string)?
        } else {
            Manifest {
                version: 1,
                collection_name: name.to_string(),
                dim,
                metric,
                segments: Vec::new(),
                model: None,
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
            let json_string = fs::read_to_string(manifest_path)?;
            serde_json::from_str(&json_string)?
        } else {
            Manifest {
                version: 1,
                collection_name: String::new(),
                dim: 0,
                metric: 0,
                segments: Vec::new(),
                model: None,
            }
        };

        let base = manifest_path.parent().unwrap_or(Path::new("."));
        let segment_files = manifest.segments.iter().map(|segment| base.join(segment)).collect();
        Ok(segment_files)
    }

    pub fn save_manifest(manifest_path: &Path, manifest: &Manifest) -> Result<()> {
        let tmp = manifest_path.with_extension("tmp");
        let json_string = &serde_json::to_string_pretty(manifest)?;
        fs::write(&tmp, json_string)?;
        fs::rename(&tmp, manifest_path)?;
        Ok(())
    }

    pub fn load_manifest(manifest_path: &Path) -> Result<Manifest> {
        if manifest_path.exists() {
            let json_string = fs::read_to_string(manifest_path)?;
            let parsed_manifest = serde_json::from_str(&json_string)?;
            Ok(parsed_manifest)
        } else {
            Ok(Manifest {
                version: 1,
                collection_name: String::new(),
                dim: 0,
                metric: 0,
                model: None,
                segments: Vec::new(),
            })
        }
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

    #[test]
    fn given_save_manifest_then_load_returns_identical_data() {
        let dir = std::env::temp_dir().join("neko_test_manifest_save_load");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        let manifest = Manifest {
            version: 1,
            collection_name: "docs".to_string(),
            dim: 384,
            metric: 1,
            model: Some("all-MiniLM-L6-v2".to_string()),
            segments: vec!["seg_001".to_string()],
        };

        ManifestManager::save_manifest(&manifest_path, &manifest).unwrap();
        assert!(manifest_path.exists());

        let loaded = ManifestManager::load_manifest(&manifest_path).unwrap();
        assert_eq!(loaded.version, 1);
        assert_eq!(loaded.collection_name, "docs");
        assert_eq!(loaded.dim, 384);
        assert_eq!(loaded.metric, 1);
        assert_eq!(loaded.model.as_deref(), Some("all-MiniLM-L6-v2"));
        assert_eq!(loaded.segments, vec!["seg_001".to_string()]);
    }

    #[test]
    fn given_save_manifest_then_tmp_file_is_cleaned_up() {
        let dir = std::env::temp_dir().join("neko_test_manifest_atomic");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        let tmp_path = dir.join("manifest.tmp");
        let manifest = Manifest {
            version: 1,
            collection_name: "tmp_test".to_string(),
            dim: 128,
            metric: 0,
            model: None,
            segments: vec![],
        };

        ManifestManager::save_manifest(&manifest_path, &manifest).unwrap();
        assert!(manifest_path.exists());
        assert!(!tmp_path.exists(), ".tmp file should be renamed to final path");
    }

    #[test]
    fn given_load_manifest_nonexistent_file_then_returns_default() {
        let dir = std::env::temp_dir().join("neko_test_manifest_load_default");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let loaded = ManifestManager::load_manifest(&dir.join("nonexistent.json")).unwrap();
        assert_eq!(loaded.version, 1);
        assert!(loaded.collection_name.is_empty());
        assert_eq!(loaded.dim, 0);
        assert_eq!(loaded.metric, 0);
        assert!(loaded.model.is_none());
        assert!(loaded.segments.is_empty());
    }

    #[test]
    fn given_save_manifest_without_model_then_load_returns_none() {
        let dir = std::env::temp_dir().join("neko_test_manifest_no_model");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let manifest_path = dir.join("manifest.json");
        let manifest = Manifest {
            version: 1,
            collection_name: "no_model".to_string(),
            dim: 256,
            metric: 2,
            model: None,
            segments: vec![],
        };

        ManifestManager::save_manifest(&manifest_path, &manifest).unwrap();
        let loaded = ManifestManager::load_manifest(&manifest_path).unwrap();
        assert!(loaded.model.is_none());
    }
}
