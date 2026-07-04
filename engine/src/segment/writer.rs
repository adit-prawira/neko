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
        let _ = fs::create_dir_all(&segment_directory);

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

    pub fn append(&mut self, vector: &[&f32], metadata: &VectorMetadata) -> Result<u64> {
        if vector.len() != self.dim as usize {
            return Err(Hairball::DimMismatch);
        }
        let meta_bytes = serde_json::to_vec(metadata)?;
        let meta_length = meta_bytes.len() as u64;

        let vector_start = self.vector_file.seek(SeekFrom::End(0))?;
        let vector_data: &[u8] = unsafe { std::slice::from_raw_parts(vector.as_ptr() as *const u8, vector.len() * std::mem::size_of::<f32>()) };
        self.vector_file.write_all(vector_data)?;

        let meta_start = self.meta_file.seek(SeekFrom::End(0))?;
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
