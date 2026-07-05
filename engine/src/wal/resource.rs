use crate::segment::resource::VectorMetadata;
use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
#[repr(u8)]
pub enum OperationCode {
    Insert = 0x01,
    Delete = 0x02,
}

impl OperationCode {
    pub fn from_u8(v: u8) -> Result<Self> {
        match v {
            0x01 => Ok(Self::Insert),
            0x02 => Ok(Self::Delete),
            _ => Err(Hairball::CorruptedSegment),
        }
    }
}

pub struct WalEntry {
    pub operation_code: OperationCode,
    pub collection: String,
    pub id: String,
    pub vector: Vec<f32>,
    pub metadata: VectorMetadata,
}
