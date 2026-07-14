use super::hairball::Hairball;

pub type Result<T> = std::result::Result<T, Hairball>;

#[derive(Debug)]
#[repr(C)]
pub struct NekoStats {
    pub vector_count: u64,
    pub dim: u32,
    pub metric: u8,
    pub storage_bytes: u64,
    pub index_type: u8,
}
