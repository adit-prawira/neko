#include "distance.h"
#include <math.h> 

#ifdef __ARM_NEON
#include <arm_neon.h>
#endif /* ifdef __ARM_NEON */

#ifdef __AVX2__
#include <immintrin.h>
#endif /* ifdef __AVX2__ */

float dot_product(const float *a, const float *b, uint32_t dim) {
  float sum = 0.0f; 
  uint32_t i = 0;

#ifdef __AVX2__
  __m256 acc0 = _mm256_setzero_ps();
  __m256 acc1 = _mm256_setzero_ps();
  for(; i + 16 <= dim; i += 16) {
    __m256 a0 = _mm256_loadu_ps(a + i);
    __m256 b0 = _mm256_loadu_ps(b + i); 
    __m256 a1 = _mm256_loadu_ps(a + i + 8); 
    __m256 b1 = _mm256_loadu_ps(b + i + 8);
    acc0 = _mm256_fmadd_ps(a0, b0, acc0);
    acc1 = _mm256_fmadd_ps(a1, b1, acc1);
  }

  acc0 = _mm256_add_ps(acc0, acc1);
  __m128 hi = _mm256_extractf128_ps(acc0, 1);
  __m128 lo = _mm256_castps256_ps128(acc0);
  __m128 s = _mm_add_ps(lo, hi);
  s = _mm_hadd_ps(s, s);
  s = _mm_hadd_ps(s, s);

  sum += _mm_cvtss_f32(s);
#elif defined (__ARM_NEON)
  float32x4_t acc0 = vdupq_n_f32(0.0f);
  float32x4_t acc1 = vdupq_n_f32(0.0f);
  float32x4_t acc2 = vdupq_n_f32(0.0f);
  float32x4_t acc3 = vdupq_n_f32(0.0f);

  for(; i + 16 <= dim; i+=16) {
    float32x4_t a0 = vld1q_f32(a + i);
    float32x4_t b0 = vld1q_f32(b + i);

    acc0 = vmlaq_f32(acc0, a0, b0);
    
    float32x4_t a1 = vld1q_f32(a + i + 4);
    float32x4_t b1 = vld1q_f32(b + i + 4);
    acc1 = vmlaq_f32(acc1, a1, b1);

    float32x4_t a2 = vld1q_f32(a + i + 8);
    float32x4_t b2 = vld1q_f32(b + i + 8);
    acc2 = vmlaq_f32(acc2, a2, b2);

    float32x4_t a3 = vld1q_f32(a + i + 12);
    float32x4_t b3 = vld1q_f32(b + i + 12);
    acc3 = vmlaq_f32(acc3, a3, b3);
  }

  acc0 = vaddq_f32(acc0, acc1);
  acc2 = vaddq_f32(acc2, acc3);
  acc0 = vaddq_f32(acc0, acc2);
  sum += vaddvq_f32(acc0);
#endif /* ifdef __AVX2__ */

  for(; i < dim; i++) {
    sum += a[i] * b[i];
  }

  return sum;
}

float cosine_distance(const float *a, const float *b, uint32_t dim) {
  float dot = dot_product(a, b, dim);
  float na2 = dot_product(a, a, dim);
  float nb2 = dot_product(b, b, dim);

  if(na2 < 1e-10f || nb2 < 1e-10f) return 1.0f;

  float cos_sim = dot / sqrtf(na2 * nb2);

  if(cos_sim > 1.0f)
    cos_sim = 1.0f; 

  if(cos_sim < -1.0f)
    cos_sim = -1.0f;

  return 1.0f - cos_sim;
}

float l2_distance(const float *a, const float *b, uint32_t dim) {
  float sum = 0.0f; 
  uint32_t i = 0;

#ifdef __AVX2__
  __m256 acc0 = _mm256_setzero_ps(); 
  __m256 acc1 = _mm256_setzero_ps(); 
  for(; i + 16 <= dim; i += 16) {
    __m256 a0 = _mm256_loadu_ps(a + i);
    __m256 b0 = _mm256_loadu_ps(b + i); 

    __m256 a1 = _mm256_loadu_ps(a + i +8); 
    __m256 b1 = _mm256_loadu_ps(b + i + 8); 

    __m256 d0 = _mm256_sub_ps(a0, b0);
    __m256 d1 = _mm256_sub_ps(a1, b1);

    acc0 = _mm256_fmadd_ps(d0, d0, acc0);
    acc1 = _mm256_fmadd_ps(d1, d1, acc1);
  }

  acc0 = _mm256_add_ps(acc0, acc1);
  __m128 hi = _mm256_extractf128_ps(acc0, 1);
  __m128 lo = _mm256_castps256_ps128(acc0);
  __m128 s = _mm_add_ps(lo, hi);

  s = _mm_hadd_ps(s, s);
  s = _mm_hadd_ps(s, s);

  sum += _mm_cvtss_f32(s);
#elif defined (__ARM_NEON)
  float32x4_t acc0 = vdupq_n_f32(0.0f);
  float32x4_t acc1 = vdupq_n_f32(0.0f);
  float32x4_t acc2 = vdupq_n_f32(0.0f);
  float32x4_t acc3 = vdupq_n_f32(0.0f);

  for(; i + 16 <= dim; i += 16) {
    float32x4_t a0 = vld1q_f32(a + i);
    float32x4_t b0 = vld1q_f32(b + i);
    float32x4_t d0 = vsubq_f32(a0, b0);
    acc0 = vmlaq_f32(acc0, d0, d0);

    float32x4_t a1 = vld1q_f32(a + i + 4);
    float32x4_t b1 = vld1q_f32(b + i + 4);
    float32x4_t d1 = vsubq_f32(a1, b1);
    acc1 = vmlaq_f32(acc1, d1, d1);

    float32x4_t a2 = vld1q_f32(a + i + 8);
    float32x4_t b2 = vld1q_f32(b + i + 8);
    float32x4_t d2 = vsubq_f32(a2, b2); 
    acc2 = vmlaq_f32(acc2, d2, d2);

    float32x4_t a3 = vld1q_f32(a + i + 12);
    float32x4_t b3 = vld1q_f32(b + i + 12);
    float32x4_t d3 = vsubq_f32(a3, b3);
    acc3 = vmlaq_f32(acc3, d3, d3);
  }

  acc0 = vaddq_f32(acc0, acc1);
  acc2 = vaddq_f32(acc2, acc3);
  acc0 = vaddq_f32(acc0, acc2);

  sum += vaddvq_f32(acc0);
#endif /* ifdef __AVX_2__ */ 
  for(; i < dim; i++) {
    float d = a[i] - b[i];
    sum += d*d;
  }

  if(sum < 0.0f)
    sum = 0.0f;

  return sqrtf(sum);
}
