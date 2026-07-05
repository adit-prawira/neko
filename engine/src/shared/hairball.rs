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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn given_all_hairball_variants_then_each_displays_correct_error_code() {
        let cases = [
            (Hairball::Ok, "HAIRBALL_OK"),
            (Hairball::NotFound, "HAIRBALL_NOT_FOUND"),
            (Hairball::AlreadyExists, "HAIRBALL_ALREADY_EXISTS"),
            (Hairball::DimMismatch, "HAIRBALL_DIM_MISMATCH"),
            (Hairball::DimTooLarge, "HAIRBALL_DIM_TOO_LARGE"),
            (Hairball::InvalidName, "HAIRBALL_INVALID_NAME"),
            (Hairball::IoError, "HAIRBALL_IO_ERROR"),
            (Hairball::SerializeError, "HAIRBALL_SERIALIZE_ERROR"),
            (Hairball::CorruptedSegment, "HAIRBALL_CORRUPTED_SEGMENT"),
            (Hairball::InternalError, "HAIRBALL_INTERNAL_ERROR"),
        ];

        for (variant, expected) in &cases {
            assert_eq!(variant.to_string(), *expected);
        }
    }

    #[test]
    fn given_an_io_error_then_from_converts_to_hairball_io_error() {
        let io_error = std::io::Error::new(std::io::ErrorKind::NotFound, "file not found");
        let hairball: Hairball = io_error.into();
        assert_eq!(hairball, Hairball::IoError);
    }

    #[test]
    fn given_a_serde_json_error_then_from_converts_to_hairball_serialize_error() {
        let json_error = serde_json::from_str::<serde_json::Value>("{invalid}").unwrap_err();
        let hairball: Hairball = json_error.into();
        assert_eq!(hairball, Hairball::SerializeError);
    }

    #[test]
    fn given_all_hairball_variants_then_discriminants_match_ffi_codes() {
        assert_eq!(Hairball::Ok as u32, 0);
        assert_eq!(Hairball::NotFound as u32, 1);
        assert_eq!(Hairball::AlreadyExists as u32, 2);
        assert_eq!(Hairball::DimMismatch as u32, 3);
        assert_eq!(Hairball::DimTooLarge as u32, 4);
        assert_eq!(Hairball::InvalidName as u32, 5);
        assert_eq!(Hairball::IoError as u32, 6);
        assert_eq!(Hairball::SerializeError as u32, 7);
        assert_eq!(Hairball::CorruptedSegment as u32, 8);
        assert_eq!(Hairball::InternalError as u32, 9);
    }
}
