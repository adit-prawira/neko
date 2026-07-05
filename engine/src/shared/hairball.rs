use std::fmt::Display;

#[derive(Copy, Clone, PartialEq, Eq, Debug)]
#[repr(u32)]
pub enum Hairball {
    Ok = 0,
    NotFound = 1,
    AlreadyExists = 2,
    DimMismatch = 3,
    DimTooLarge = 4,
    InvalidName = 5,
    IoError = 6,
    SerializeError = 7,
    CorruptedSegment = 8,
    InternalError = 9,
}

impl Display for Hairball {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        let code = match self {
            Hairball::Ok => "HAIRBALL_OK",
            Hairball::NotFound => "HAIRBALL_NOT_FOUND",
            Hairball::AlreadyExists => "HAIRBALL_ALREADY_EXISTS",
            Hairball::DimMismatch => "HAIRBALL_DIM_MISMATCH",
            Hairball::DimTooLarge => "HAIRBALL_DIM_TOO_LARGE",
            Hairball::InvalidName => "HAIRBALL_INVALID_NAME",
            Hairball::IoError => "HAIRBALL_IO_ERROR",
            Hairball::SerializeError => "HAIRBALL_SERIALIZE_ERROR",
            Hairball::CorruptedSegment => "HAIRBALL_CORRUPTED_SEGMENT",
            Hairball::InternalError => "HAIRBALL_INTERNAL_ERROR",
        };
        write!(f, "{}", code)
    }
}

impl From<std::io::Error> for Hairball {
    fn from(_: std::io::Error) -> Self {
        Self::IoError
    }
}

impl From<serde_json::Error> for Hairball {
    fn from(_: serde_json::Error) -> Self {
        Self::SerializeError
    }
}
