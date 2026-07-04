pub mod hairball;
pub mod results;

unsafe extern "C" {
    pub fn dot_product(a: *const f32, b: *const f32, dim: u32) -> f32;
    pub fn cosine_distance(a: *const f32, b: *const f32, dim: u32) -> f32;
    pub fn l2_distance(a: *const f32, b: *const f32, dim: u32) -> f32;
}
