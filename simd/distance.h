/*
 * The idea of these function is to perform search.
 * These functions are comparison engine that search will run 
 *
 * If the request is asking to fin 10 most similar documents => the system will call these
 * functions thousands of times per second to figure out which 10 are closest.
 *
 * Without these functions, searching thorugh 100K vectors would take seconds instead of milliseconds
 * */
#ifndef SIMD_DISTANCE_H
#define SIMD_DISTANCE_H

#include <stdint.h>

/*
 * The method is trying to answer the question of: "How much do they overlap"
 * In simpler term, its trying to count how many matching words two books share.
 *
 * Computes sum(a[i] * b[i]) uses mutiple accumulator registers churning in parallel 
 * to hide instruction latency. 
 *  - NEON: 4 registers x 4 floats = 16/iteration
 *  - AVX2: 2 registers x 8 floats = 16/iteration
 * 
 * The vmlaq_f32/_mm256_fmadd_ps instruction does multiply and then add in on CPU cycle 
 * */
float dot_product(const float *a, const float *b, uint32_t dim);


/*
 * The method is trying to answer the question of: "Do they point to the same direction"
 * In simpler term, its trying to see if two arrows pointing at the same target.
 *
 * It computes 1 - (a·b) / sqrt((a·a) · (b·b)).
 * The function will then clamp the value between [-1, 1] to avoid float rounding giving NaN
 * from the sqrt.
 *
 * Zero vector check prevents division by zero.
 * */
float cosine_distance(const float *a, const float *b, uint32_t dim);

/*
 * The method is trying to answer the question of: "How are apart are they?"
 * In simpler term, its trying to measure the distance between 2 dots on a map
 *
 * It is the same accumulator pattern that computer (a[i] - b[i])² using subtract then 
 * multiply-accumulate.
 *
 * return sqrt(sum).
 *
 * It will then guard against negative rounding before sqrt().
 * */
float l2_distance(const float *a, const float *b, uint32_t dim);

#endif
