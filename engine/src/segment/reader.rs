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
        let header_size = std::mem::size_of::<SegmentHeader>() as u64;
        let start = (header_size + entry.vector_offset) as usize;
        let end = start + (self.meta.dim as usize * std::mem::size_of::<f32>());
        let bytes = &self.vector_mmap[start..end];
        let floats: &[f32] = unsafe { std::slice::from_raw_parts(bytes.as_ptr() as *const f32, self.meta.dim as usize) };

        Ok(floats)
    }

    pub fn get_metadata(&self, index: u64) -> Result<VectorMetadata> {
        let entry = self.read_index_entry(index)?;
        let header_size = std::mem::size_of::<SegmentHeader>() as u64;
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

#[cfg(test)]
mod tests {
    use crate::segment::resource::VectorMetadata;
    use crate::segment::writer::SegmentWriter;
    use crate::shared::hairball::Hairball;
    use std::fs;
    use std::io::Write;
    use std::path::Path;

    use super::*;

    fn make_metadata(id: &str, custom: &str) -> VectorMetadata {
        VectorMetadata {
            id: id.to_string(),
            created_at: 0,
            deleted: false,
            custom: custom.to_string(),
        }
    }

    fn create_test_segment(directory: &Path, dim: u32, vectors: &[(&[f32], VectorMetadata)]) -> SegmentMeta {
        let mut writer = SegmentWriter::new(directory, "test_seg", dim).unwrap();
        for (vec, meta) in vectors {
            writer.append(vec, meta).unwrap();
        }
        writer.finish().unwrap()
    }

    #[test]
    fn given_a_segment_with_no_vectors_then_open_returns_reader_with_zero_length() {
        let dir = std::env::temp_dir().join("neko_test_reader_empty");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let meta = create_test_segment(&dir, 3, &[]);
        let reader = SegmentReader::open(&dir.join("test_seg"), meta).unwrap();

        assert_eq!(reader.length(), 0);
        assert_eq!(reader.iter_vectors().count(), 0);
    }

    #[test]
    fn given_a_segment_with_two_vectors_then_get_vector_returns_correct_data() {
        let dir = std::env::temp_dir().join("neko_test_reader_vectors");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let vectors = [(&[1.0_f32, 2.0, 3.0][..], make_metadata("a", "")), (&[4.0_f32, 5.0, 6.0][..], make_metadata("b", ""))];
        let meta = create_test_segment(&dir, 3, &vectors);
        let reader = SegmentReader::open(&dir.join("test_seg"), meta).unwrap();

        assert_eq!(reader.length(), 2);
        assert_eq!(reader.get_vector(0).unwrap(), &[1.0, 2.0, 3.0]);
        assert_eq!(reader.get_vector(1).unwrap(), &[4.0, 5.0, 6.0]);
    }

    #[test]
    fn given_a_segment_with_custom_metadata_then_get_metadata_returns_full_metadata() {
        let dir = std::env::temp_dir().join("neko_test_reader_meta");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let vectors = [
            (&[1.0_f32, 2.0_f32][..], make_metadata("vec_a", "extra_a")),
            (&[3.0_f32, 4.0_f32][..], make_metadata("vec_b", "extra_b")),
        ];
        let meta = create_test_segment(&dir, 2, &vectors);
        let reader = SegmentReader::open(&dir.join("test_seg"), meta).unwrap();

        let meta0 = reader.get_metadata(0).unwrap();
        assert_eq!(meta0.id, "vec_a");
        assert_eq!(meta0.custom, "extra_a");

        let meta1 = reader.get_metadata(1).unwrap();
        assert_eq!(meta1.id, "vec_b");
        assert_eq!(meta1.custom, "extra_b");
    }

    #[test]
    fn given_a_segment_with_two_vectors_then_iter_vectors_yields_all_in_order() {
        let dir = std::env::temp_dir().join("neko_test_reader_iter");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let vectors = [(&[10.0_f32, 20.0_f32][..], make_metadata("x", "")), (&[30.0_f32, 40.0_f32][..], make_metadata("y", ""))];
        let meta = create_test_segment(&dir, 2, &vectors);
        let reader = SegmentReader::open(&dir.join("test_seg"), meta).unwrap();

        let mut iter = reader.iter_vectors();
        assert_eq!(iter.next().unwrap().unwrap(), &[10.0, 20.0]);
        assert_eq!(iter.next().unwrap().unwrap(), &[30.0, 40.0]);
        assert!(iter.next().is_none());
    }

    #[test]
    fn given_a_segment_with_known_dimension_then_dim_returns_correct_value() {
        let dir = std::env::temp_dir().join("neko_test_reader_dim");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let meta = create_test_segment(&dir, 128, &[]);
        let reader = SegmentReader::open(&dir.join("test_seg"), meta).unwrap();

        assert_eq!(reader.dim(), 128);
    }

    #[test]
    fn given_a_valid_segment_then_open_succeeds() {
        let dir = std::env::temp_dir().join("neko_test_reader_open_valid");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let meta = create_test_segment(&dir, 4, &[]);
        let result = SegmentReader::open(&dir.join("test_seg"), meta);
        assert!(result.is_ok());
    }

    #[test]
    fn given_a_segment_file_with_corrupted_header_then_open_returns_corrupted_segment_error() {
        let dir = std::env::temp_dir().join("neko_test_reader_corrupt");
        let _ = fs::remove_dir_all(&dir);
        fs::create_dir_all(&dir).unwrap();

        let _meta = create_test_segment(&dir, 4, &[]);

        let mut file = std::fs::OpenOptions::new().write(true).open(dir.join("test_seg").join("segment.vec")).unwrap();
        file.write_all(&[0xFF; 8]).unwrap();

        let meta = SegmentMeta {
            directory: dir.join("test_seg"),
            dim: 4,
            count: 0,
        };
        let result = SegmentReader::open(&dir.join("test_seg"), meta);
        assert!(matches!(result, Err(Hairball::CorruptedSegment)));
    }
}
