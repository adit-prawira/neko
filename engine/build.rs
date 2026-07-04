use std::env;

fn main() {
    let mut build = cc::Build::new();
    build.file("../simd/distance.c").opt_level(3).warnings(true).flag("-std=c11");

    let target_arch = env::var("CARGO_CFG_TARGET_ARCH").unwrap();
    match target_arch.as_str() {
        "aarch64" => build.flag("-march=armv8-a+simd"),
        "x86_64" => {
            build.flag("-mavx2");
            build.flag("-mfma")
        }
        _ => panic!("unsupported arch: {}", target_arch),
    };
    build.compile("distance");
}
