use std::fs::File;
use std::path::Path;

use memmap2::Mmap;

use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::resource::{SEGMENT_MAGIC, SEGMENT_VERSION, SegmentHeader, SegmentMeta, VectorMetadata};

#[repr(C)]
struct IndexEntry {
    vector_offset: u64,
    meta_offset: u64,
    meta_length: u64,
}

pub struct VectorIter<'a> {
    reader: &'a SegmentReader,
    current: u64,
    total: u64,
}

impl<'a> Iterator for VectorIter<'a> {
    type Item = Result<&'a [f32]>;

    fn next(&mut self) -> Option<Self::Item> {
        if self.current >= self.total {
            return None;
        }

        let index = self.current;
        self.current += 1;
        Some(self.reader.get_vector(index))
    }
}

/*
 * SegmentReader has responsibilites to:
 * -> Open an existing segment via mmap and map it to .vec and .idx files director into the process
 *    address space
 * -> Provide an iterator traverse vectors sequentially without copying them off disk
 *    (OS page cache handles what's in RAM)
 * -> Brute-force KNN loop iterates all segments and computes distance via SIMD on the mmap's
 *    vector data
 * */
pub struct SegmentReader {
    meta: SegmentMeta,
    vector_mmap: Mmap,
    index_mmap: Mmap,
}

impl SegmentReader {
    pub fn open(directory: &Path, meta: SegmentMeta) -> Result<Self> {
        let vector_path = directory.join("segment.vec");
        let index_path = directory.join("segment.idx");

        let vector_file = File::open(vector_path)?;
        let index_file = File::open(index_path)?;

        let vector_mmap = unsafe { Mmap::map(&vector_file)? };
        let index_mmap = unsafe { Mmap::map(&index_file)? };

        let reader = Self { meta, vector_mmap, index_mmap };
        reader.validate()?;
        Ok(reader)
    }

    pub fn dim(&self) -> u32 {
        self.meta.dim
    }

    pub fn length(&self) -> u64 {
        self.meta.count
    }

    pub fn get_vector(&self, index: u64) -> Result<&[f32]> {
        let entry = self.read_index_entry(index)?;
        let header_size = std::mem::size_of::<SegmentReader>() as u64;
        let start = (header_size + entry.vector_offset) as usize;
        let end = start + (self.meta.dim as usize * std::mem::size_of::<f32>());
        let bytes = &self.vector_mmap[start..end];
        let floats: &[f32] = unsafe { std::slice::from_raw_parts(bytes.as_ptr() as *const f32, self.meta.dim as usize) };

        Ok(floats)
    }

    pub fn get_metadata(&self, index: u64) -> Result<VectorMetadata> {
        let entry = self.read_index_entry(index)?;
        let header_size = std::mem::size_of::<SegmentReader>() as u64;
        let meta_file = File::open(self.meta.directory.join("segment.meta"))?;
        let meta_mmap = unsafe { Mmap::map(&meta_file)? };
        let start = (header_size + entry.meta_offset) as usize;
        let end = start + entry.meta_length as usize;
        serde_json::from_slice(&meta_mmap[start..end]).map_err(Into::into)
    }

    pub fn iter_vectors(&self) -> VectorIter<'_> {
        VectorIter {
            reader: self,
            current: 0,
            total: self.meta.count,
        }
    }

    fn validate(&self) -> Result<()> {
        let header = unsafe { &*(self.vector_mmap.as_ptr() as *const SegmentHeader) };
        let is_valid_neko_file = header.magic == SEGMENT_MAGIC && header.version == SEGMENT_VERSION;

        if !is_valid_neko_file {
            return Err(Hairball::CorruptedSegment);
        };

        Ok(())
    }

    fn read_index_entry(&self, index: u64) -> Result<IndexEntry> {
        let header_size = std::mem::size_of::<SegmentHeader>();
        let entry_size = std::mem::size_of::<IndexEntry>();
        let offset = header_size + (index as usize * entry_size);
        let bytes = &self.index_mmap[offset..offset + entry_size];
        Ok(IndexEntry {
            vector_offset: u64::from_le_bytes(bytes[0..8].try_into().unwrap()),
            meta_offset: u64::from_le_bytes(bytes[8..16].try_into().unwrap()),
            meta_length: u64::from_le_bytes(bytes[16..24].try_into().unwrap()),
        })
    }
}
