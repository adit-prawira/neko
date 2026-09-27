use crate::shared::hairball::Hairball;
use crate::shared::results::Result;

use super::resource::MAX_DIM;

pub struct EngineValidator;

impl EngineValidator {
    pub fn collection_name(name: &str) -> Result<()> {
        // must not be empty or has more than 64 characters
        if name.is_empty() || name.len() > 64 {
            return Err(Hairball::InvalidName);
        }

        let first = name.chars().next().unwrap();

        // check first character to be alphanumeric characters
        // for early return
        if !first.is_ascii_alphanumeric() {
            return Err(Hairball::InvalidName);
        }

        let is_valid_characters = name.chars().all(|ch| ch.is_ascii_alphanumeric() || ch == '_' || ch == '-');
        if !is_valid_characters {
            return Err(Hairball::InvalidName);
        }

        Ok(())
    }

    pub fn metric(metric: u8) -> Result<()> {
        // metric options only have value of 0 - 2
        // anything beyond it is invalid
        if metric > 2 {
            return Err(Hairball::InvalidMetric);
        }

        Ok(())
    }

    pub fn dim(dim: u32) -> Result<()> {
        if dim == 0 {
            return Err(Hairball::DimTooSmall);
        }
        if dim > MAX_DIM {
            return Err(Hairball::DimTooLarge);
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn given_empty_name_then_returns_invalid_name() {
        assert_eq!(EngineValidator::collection_name("").unwrap_err(), Hairball::InvalidName);
    }

    #[test]
    fn given_name_over_64_chars_then_returns_invalid_name() {
        let name = "a".repeat(65);
        assert_eq!(EngineValidator::collection_name(&name).unwrap_err(), Hairball::InvalidName);
    }

    #[test]
    fn given_name_exactly_64_chars_then_returns_ok() {
        let name = "a".repeat(64);
        assert!(EngineValidator::collection_name(&name).is_ok());
    }

    #[test]
    fn given_name_with_non_alphanumeric_first_char_then_returns_invalid_name() {
        assert_eq!(EngineValidator::collection_name("-abc").unwrap_err(), Hairball::InvalidName);
        assert_eq!(EngineValidator::collection_name("_abc").unwrap_err(), Hairball::InvalidName);
        assert_eq!(EngineValidator::collection_name(".abc").unwrap_err(), Hairball::InvalidName);
    }

    #[test]
    fn given_name_with_special_chars_then_returns_invalid_name() {
        assert_eq!(EngineValidator::collection_name("my collection").unwrap_err(), Hairball::InvalidName);
        assert_eq!(EngineValidator::collection_name("col@name").unwrap_err(), Hairball::InvalidName);
        assert_eq!(EngineValidator::collection_name("col#name").unwrap_err(), Hairball::InvalidName);
    }

    #[test]
    fn given_valid_hyphenated_name_then_returns_ok() {
        assert!(EngineValidator::collection_name("my-collection").is_ok());
    }

    #[test]
    fn given_valid_underscored_name_then_returns_ok() {
        assert!(EngineValidator::collection_name("my_collection").is_ok());
    }

    #[test]
    fn given_valid_alphanumeric_name_then_returns_ok() {
        assert!(EngineValidator::collection_name("docs123").is_ok());
    }

    #[test]
    fn given_metric_0_then_returns_ok() {
        assert!(EngineValidator::metric(0).is_ok());
    }

    #[test]
    fn given_metric_1_then_returns_ok() {
        assert!(EngineValidator::metric(1).is_ok());
    }

    #[test]
    fn given_metric_2_then_returns_ok() {
        assert!(EngineValidator::metric(2).is_ok());
    }

    #[test]
    fn given_metric_greater_than_2_then_returns_invalid_metric() {
        assert_eq!(EngineValidator::metric(3).unwrap_err(), Hairball::InvalidMetric);
        assert_eq!(EngineValidator::metric(255).unwrap_err(), Hairball::InvalidMetric);
    }

    #[test]
    fn given_dim_zero_then_returns_dim_too_small() {
        assert_eq!(EngineValidator::dim(0).unwrap_err(), Hairball::DimTooSmall);
    }

    #[test]
    fn given_dim_greater_than_max_then_returns_dim_too_large() {
        assert_eq!(EngineValidator::dim(4097).unwrap_err(), Hairball::DimTooLarge);
        assert_eq!(EngineValidator::dim(u32::MAX).unwrap_err(), Hairball::DimTooLarge);
    }

    #[test]
    fn given_dim_at_max_then_returns_ok() {
        assert!(EngineValidator::dim(4096).is_ok());
    }

    #[test]
    fn given_valid_dim_then_returns_ok() {
        assert!(EngineValidator::dim(384).is_ok());
        assert!(EngineValidator::dim(1).is_ok());
    }
}
