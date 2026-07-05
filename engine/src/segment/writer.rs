use std::fs::{self, File};
use std::io::{Seek, SeekFrom, Write};
use std::path::{Path, PathBuf};

use crate::segment::resource::{SEGMENT_MAGIC, SEGMENT_VERSION};
use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::resource::{SegmentHeader, SegmentMeta, VectorMetadata};

/*
 * SegmentWriter has responsiblities to:
 * -> Takes vector with .append() then serializes them to binary segment files (.vec, .idx, .meta)
 * -> It will writes the segment header and fsyncs everything and close files via .finish()
 * -> Created once per segment -> Writes are sequential and not a random I/O
 * -> Used by WAL compaction: Replay frozen WAL -> sort by ID -> write through SegmentWriter -> a
 *    new read-optimized segment
 * */
pub struct SegmentWriter {
    directory: PathBuf,
    dim: u32,
    vector_file: File,
    meta_file: File,
    index_file: File,
    vector_offsets: Vec<u64>,
    count: u64,
    metadata_total: u64,
}

impl SegmentWriter {
    pub fn new(directory: &Path, name: &str, dim: u32) -> Result<Self> {
        let segment_directory = directory.join(name);
        fs::create_dir_all(&segment_directory)?;

        let vector_path = segment_directory.join("segment.vec");
        let meta_path = segment_directory.join("segment.meta");
        let index_path = segment_directory.join("segment.idx");

        let mut vector_file = File::create(vector_path)?;
        let mut meta_file = File::create(meta_path)?;
        let mut index_file = File::create(index_path)?;

        let header = SegmentHeader {
            magic: SEGMENT_MAGIC,
            version: SEGMENT_VERSION,
            dim,
            count: 0,
            metadata_length: 0,
        };

        Self::write_header(&mut vector_file, &header)?;
        Self::write_header(&mut meta_file, &header)?;
        Self::write_header(&mut index_file, &header)?;

        Ok(Self {
            directory: segment_directory,
            dim,
            vector_file,
            meta_file,
            index_file,
            vector_offsets: Vec::new(),
            count: 0,
            metadata_total: 0,
        })
    }

    pub fn append(&mut self, vector: &[f32], metadata: &VectorMetadata) -> Result<u64> {
        if vector.len() != self.dim as usize {
            return Err(Hairball::DimMismatch);
        }
        let meta_bytes = serde_json::to_vec(metadata)?;
        let meta_length = meta_bytes.len() as u64;

        let header_size = std::mem::size_of::<SegmentHeader>() as u64;
        let vector_start = self.vector_file.seek(SeekFrom::End(0))? - header_size;
        let vector_data: &[u8] = unsafe { std::slice::from_raw_parts(vector.as_ptr() as *const u8, std::mem::size_of_val(vector)) };
        self.vector_file.write_all(vector_data)?;

        let meta_start = self.meta_file.seek(SeekFrom::End(0))? - header_size;
        self.meta_file.write_all(&meta_bytes)?;

        self.index_file.write_all(&vector_start.to_le_bytes())?;
        self.index_file.write_all(&meta_start.to_le_bytes())?;
        self.index_file.write_all(&meta_length.to_le_bytes())?;

        self.vector_offsets.push(vector_start);
        self.count += 1;
        self.metadata_total += meta_length;
        Ok(self.count - 1)
    }

    pub fn finish(mut self) -> Result<SegmentMeta> {
        let header = SegmentHeader {
            magic: SEGMENT_MAGIC,
            version: SEGMENT_VERSION,
            dim: self.dim,
            count: self.count,
            metadata_length: self.metadata_total,
        };

        Self::update_header(&mut self.vector_file, &header)?;
        Self::update_header(&mut self.meta_file, &header)?;
        Self::update_header(&mut self.index_file, &header)?;

        self.vector_file.flush()?;
        self.meta_file.flush()?;
        self.index_file.flush()?;
        Ok(SegmentMeta {
            directory: self.directory,
            dim: self.dim,
            count: self.count,
        })
    }

    fn write_header(file: &mut File, header: &SegmentHeader) -> Result<()> {
        // converting struct memory to bytes
        // SegmentHeader is #[repr(C)] with plain integer fields, thus all bit patterns are valid
        let bytes: &[u8] = unsafe { std::slice::from_raw_parts(header as *const SegmentHeader as *const u8, std::mem::size_of::<SegmentHeader>()) };

        file.write_all(bytes)?;
        Ok(())
    }

    fn update_header(file: &mut File, header: &SegmentHeader) -> Result<()> {
        file.seek(SeekFrom::Start(0))?;
        Self::write_header(file, header)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::segment::resource::{SEGMENT_MAGIC, SEGMENT_VERSION};
    use crate::shared::hairball::Hairball;
    use std::io::Read;

    fn read_segment_header(file_path: &std::path::Path) -> SegmentHeader {
        let mut file = std::fs::File::open(file_path).unwrap();
        let mut buf = Vec::new();
        file.read_to_end(&mut buf).unwrap();
        unsafe { std::ptr::read_unaligned(buf.as_ptr() as *const SegmentHeader) }
    }

    fn default_metadata(id: &str) -> VectorMetadata {
        VectorMetadata {
            id: id.to_string(),
            created_at: 0,
            deleted: false,
            custom: String::new(),
        }
    }

    #[test]
    fn given_a_new_writer_then_creates_directory_and_three_segment_files_with_valid_initial_headers() {
        let directory = std::env::temp_dir().join("neko_test_writer_new");
        let _ = std::fs::remove_dir_all(&directory);
        std::fs::create_dir_all(&directory).unwrap();

        let writer = SegmentWriter::new(&directory, "seg_test", 4).unwrap();
        drop(writer);

        let segment_path = directory.join("seg_test");
        assert!(segment_path.exists());

        for file_name in &["segment.vec", "segment.meta", "segment.idx"] {
            let path = segment_path.join(file_name);
            assert!(path.exists(), "file missing: {}", file_name);
            let header = read_segment_header(&path);
            assert_eq!(header.magic, SEGMENT_MAGIC);
            assert_eq!(header.version, SEGMENT_VERSION);
            assert_eq!(header.dim, 4);
            assert_eq!(header.count, 0);
            assert_eq!(header.metadata_length, 0);
        }
    }

    #[test]
    fn given_appended_vectors_with_matching_dimension_then_append_returns_sequential_indices() {
        let directory = std::env::temp_dir().join("neko_test_writer_append_indices");
        let _ = std::fs::remove_dir_all(&directory);
        std::fs::create_dir_all(&directory).unwrap();

        let mut writer = SegmentWriter::new(&directory, "seg", 2).unwrap();
        assert_eq!(writer.append(&[1.0, 2.0], &default_metadata("a")).unwrap(), 0);
        assert_eq!(writer.append(&[3.0, 4.0], &default_metadata("b")).unwrap(), 1);
    }

    #[test]
    fn given_appended_vector_with_wrong_dimension_then_append_returns_dim_mismatch_error() {
        let directory = std::env::temp_dir().join("neko_test_writer_dim_err");
        let _ = std::fs::remove_dir_all(&directory);
        std::fs::create_dir_all(&directory).unwrap();

        let mut writer = SegmentWriter::new(&directory, "seg", 3).unwrap();
        let result = writer.append(&[1.0, 2.0], &default_metadata("a"));
        assert!(matches!(result, Err(Hairball::DimMismatch)));
    }

    #[test]
    fn given_two_appended_vectors_then_finish_writes_correct_vector_data_and_updates_headers() {
        let directory = std::env::temp_dir().join("neko_test_writer_data");
        let _ = std::fs::remove_dir_all(&directory);
        std::fs::create_dir_all(&directory).unwrap();

        let mut writer = SegmentWriter::new(&directory, "seg", 2).unwrap();
        writer.append(&[1.0, 2.0], &default_metadata("a")).unwrap();
        writer.append(&[3.0, 4.0], &default_metadata("b")).unwrap();
        let meta = writer.finish().unwrap();

        assert_eq!(meta.dim, 2);
        assert_eq!(meta.count, 2);
        assert_eq!(meta.directory, directory.join("seg"));

        let header_size = std::mem::size_of::<SegmentHeader>();
        let vec_path = directory.join("seg").join("segment.vec");
        let idx_path = directory.join("seg").join("segment.idx");

        // verify vector data at expected offsets
        {
            let mut file = std::fs::File::open(&vec_path).unwrap();
            let mut buf = Vec::new();
            file.read_to_end(&mut buf).unwrap();
            let data = &buf[header_size..];

            assert_eq!(f32::from_le_bytes(data[0..4].try_into().unwrap()), 1.0);
            assert_eq!(f32::from_le_bytes(data[4..8].try_into().unwrap()), 2.0);
            assert_eq!(f32::from_le_bytes(data[8..12].try_into().unwrap()), 3.0);
            assert_eq!(f32::from_le_bytes(data[12..16].try_into().unwrap()), 4.0);
        }

        // verify index contains correct absolute vector offsets
        {
            let mut file = std::fs::File::open(&idx_path).unwrap();
            let mut buf = Vec::new();
            file.read_to_end(&mut buf).unwrap();
            let idx_data = &buf[header_size..];

            let vector_offset_0 = u64::from_le_bytes(idx_data[0..8].try_into().unwrap());
            let vector_offset_1 = u64::from_le_bytes(idx_data[24..32].try_into().unwrap());
            assert_eq!(vector_offset_0, 0);
            assert_eq!(vector_offset_1, 8);
        }

        // verify final headers have updated counts
        {
            let header = read_segment_header(&vec_path);
            assert_eq!(header.count, 2);
            assert_ne!(header.metadata_length, 0);
        }
    }
}
