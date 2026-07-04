// Magic will ensure the authenticity of the file being read
// is related to neko and not just some random file
pub const SEGMENT_MAGIC: u32 = 0x6E656B6F;

// Version ensure that the format of the file is something that
// neko knows how to read
pub const SEGMENT_VERSION: u32 = 1;

#[derive(Clone, Copy, Debug)]
#[repr(C)]
pub struct SegmentHeader {
    pub magic: u32,
    pub version: u32,
    pub dim: u32,
    pub count: u64,
    pub metadata_length: u64,
}

#[derive(Clone, Debug, serde::Serialize, serde::Deserialize)]
pub struct VectorMetadata {
    pub id: String,

    #[serde(default)]
    pub created_at: u64,

    #[serde(default)]
    pub deleted: bool,

    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub custom: String,
}

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum Metric {
    L2 = 0,
    Cosine = 1,
    Dot = 2,
}

#[derive(Clone, Debug)]
pub struct SegmentMeta {
    pub directory: std::path::PathBuf,
    pub dim: u32,
    pub count: u64,
}
