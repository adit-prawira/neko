use std::ffi::c_int;

pub mod engine;
pub mod manifest;
pub mod segment;
pub mod shared;
pub mod wal;

#[unsafe(no_mangle)]
pub extern "C" fn neko_version() -> c_int {
    0
}
