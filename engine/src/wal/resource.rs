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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn given_a_valid_insert_byte_then_from_u8_returns_insert_variant() {
        let result = OperationCode::from_u8(0x01);
        assert_eq!(result, Ok(OperationCode::Insert));
    }

    #[test]
    fn given_a_valid_delete_byte_then_from_u8_returns_delete_variant() {
        let result = OperationCode::from_u8(0x02);
        assert_eq!(result, Ok(OperationCode::Delete));
    }

    #[test]
    fn given_an_invalid_byte_then_from_u8_returns_corrupted_segment_error() {
        let result = OperationCode::from_u8(0xFF);
        assert_eq!(result, Err(Hairball::CorruptedSegment));
    }

    #[test]
    fn given_operationcode_variants_then_discriminants_match_wal_binary_format() {
        assert_eq!(OperationCode::Insert as u8, 0x01);
        assert_eq!(OperationCode::Delete as u8, 0x02);
    }
}
