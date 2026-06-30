#include "distance.h"
#include <float.h>
#include <math.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#define TEST_DIM 384
#define RANDOM_COUNT 10000
#define SYNTHETIC_DIM 16

static int failures = 0;
static const char *fn_type = "";

static int ok(int cond, const char *msg, float got, float expected) {
    if (cond) return 0;
    failures++;
    printf("  FAIL [%s] %s: got=%.8e expected=%.8e\n", fn_type, msg, got, expected);
    return 1;
}

/* --- helpers --- */

static float scalar_dot(const float *a, const float *b, uint32_t dim) {
    double s = 0.0;
    for (uint32_t i = 0; i < dim; i++) s += (double)a[i] * (double)b[i];
    return (float)s;
}

static float scalar_cos(const float *a, const float *b, uint32_t dim) {
    float dot = dot_product(a, b, dim);
    float na2 = dot_product(a, a, dim);
    float nb2 = dot_product(b, b, dim);
    if (na2 < 1e-10f || nb2 < 1e-10f) return 1.0f;
    float cs = dot / sqrtf(na2 * nb2);
    if (cs > 1.0f) cs = 1.0f;
    if (cs < -1.0f) cs = -1.0f;
    return 1.0f - cs;
}

/*
 * Dynamic tolerance: grows with vector size and result magnitude.
 * For a dim-element vector with result R:
 *   expected error ≈ dim × ε × |R|
 * We use dim × 2ε as a safe bound, plus a floor so tiny values aren't over-tested.
 */
static float tolerance(float ref, uint32_t dim) {
    return fmaxf(3e-5f, fabsf(ref) * (float)dim * FLT_EPSILON * 4.0f);
}

/* --- deterministic random generator (simple LCG, no lib deps) --- */

static uint32_t rng_state = 0xDEADBEEF;

static uint32_t rng(void) {
    rng_state = rng_state * 1664525u + 1013904223u;
    return rng_state;
}

static float randf(void) {
    return (float)(rng() & 0xFFFFFF) / (float)0xFFFFFF;  // [0, 1]
}

static void rand_vec(float *v, uint32_t dim) {
    for (uint32_t i = 0; i < dim; i++) v[i] = randf() * 2.0f - 1.0f;  // [-1, 1]
}

/* --- 1. Determinism: same input twice = same output --- */

static void test_determinism(void) {
    fn_type = "determinism";
    float a[SYNTHETIC_DIM], b[SYNTHETIC_DIM];
    rng_state = 0xCAFE;
    rand_vec(a, SYNTHETIC_DIM);
    rand_vec(b, SYNTHETIC_DIM);

    float d1 = dot_product(a, b, SYNTHETIC_DIM);
    float d2 = dot_product(a, b, SYNTHETIC_DIM);
    ok(d1 == d2, "dot_product not deterministic", d1, d2);

    float c1 = cosine_distance(a, b, SYNTHETIC_DIM);
    float c2 = cosine_distance(a, b, SYNTHETIC_DIM);
    ok(c1 == c2, "cosine_distance not deterministic", c1, c2);

    float l1 = l2_distance(a, b, SYNTHETIC_DIM);
    float l2 = l2_distance(a, b, SYNTHETIC_DIM);
    ok(l1 == l2, "l2_distance not deterministic", l1, l2);
}

/* --- 2. Symmetry: f(a,b) == f(b,a) --- */

static void test_symmetry(void) {
    fn_type = "symmetry";
    float a[SYNTHETIC_DIM], b[SYNTHETIC_DIM];
    rng_state = 0xBEEF;

    for (int i = 0; i < 1000; i++) {
        rand_vec(a, SYNTHETIC_DIM);
        rand_vec(b, SYNTHETIC_DIM);

        float d1 = dot_product(a, b, SYNTHETIC_DIM);
        float d2 = dot_product(b, a, SYNTHETIC_DIM);
        ok(fabsf(d1 - d2) < FLT_EPSILON * 2.0f, "dot not symmetric", d1, d2);

        float c1 = cosine_distance(a, b, SYNTHETIC_DIM);
        float c2 = cosine_distance(b, a, SYNTHETIC_DIM);
        ok(fabsf(c1 - c2) < FLT_EPSILON * 2.0f, "cosine not symmetric", c1, c2);

        float l1 = l2_distance(a, b, SYNTHETIC_DIM);
        float l2_ = l2_distance(b, a, SYNTHETIC_DIM);
        ok(fabsf(l1 - l2_) < FLT_EPSILON, "l2 not symmetric", l1, l2_);
    }
}

/* --- 3. Identity: dot(v,v)=|v|^2, cos(v,v)=0, l2(v,v)=0 --- */

static void test_identity(void) {
    fn_type = "identity";
    float v[SYNTHETIC_DIM];
    rng_state = 0xF001;

    for (int i = 0; i < 100; i++) {
        rand_vec(v, SYNTHETIC_DIM);

        float dp = dot_product(v, v, SYNTHETIC_DIM);
        float ssum = scalar_dot(v, v, SYNTHETIC_DIM);
        ok(fabsf(dp - ssum) <= tolerance(ssum, SYNTHETIC_DIM),
           "dot(v,v) != sum(v[i]^2)", dp, ssum);

        float cd = cosine_distance(v, v, SYNTHETIC_DIM);
        ok(fabsf(cd) < 1e-5f, "cosine(v,v) != 0", cd, 0.0f);

        float l2 = l2_distance(v, v, SYNTHETIC_DIM);
        ok(l2 == 0.0f, "l2(v,v) != 0 (exact)", l2, 0.0f);
    }
}

/* --- 4. Orthogonal: cos([1,0],[0,1]) == 1, dot(...) == 0 --- */

static void test_orthogonal(void) {
    fn_type = "orthogonal";
    float a[4] = {1.0f, 0.0f, 0.0f, 0.0f};
    float b[4] = {0.0f, 1.0f, 0.0f, 0.0f};

    float dp = dot_product(a, b, 4);
    ok(dp == 0.0f, "dot([1,0],[0,1]) != 0 (exact)", dp, 0.0f);

    float cd = cosine_distance(a, b, 4);
    ok(cd == 1.0f, "cosine([1,0],[0,1]) != 1 (exact)", cd, 1.0f);
}

/* --- 5. Zero vector: cosine=1, l2(zero,v)=|v| --- */

static void test_zero(void) {
    fn_type = "zero";
    float z[4] = {0.0f, 0.0f, 0.0f, 0.0f};
    float v[4] = {3.0f, 4.0f, 0.0f, 0.0f};  // |v| = 5

    float cd = cosine_distance(z, v, 4);
    ok(cd == 1.0f, "cosine(zero,v) != 1 (exact)", cd, 1.0f);

    float cd2 = cosine_distance(v, z, 4);
    ok(cd2 == 1.0f, "cosine(v,zero) != 1 (exact)", cd2, 1.0f);

    float l2 = l2_distance(z, v, 4);
    ok(fabsf(l2 - 5.0f) < 1e-5f, "l2(zero,v) != 5", l2, 5.0f);

    // zero vs zero
    float l2z = l2_distance(z, z, 4);
    ok(l2z == 0.0f, "l2(zero,zero) != 0 (exact)", l2z, 0.0f);
}

/* --- 6. Random: SIMD vs double-precision scalar reference --- */

static void test_random_vs_double(void) {
    fn_type = "random";
    float *a = malloc(TEST_DIM * sizeof(float));
    float *b = malloc(TEST_DIM * sizeof(float));

    float max_err_dot = 0, max_err_cos = 0, max_err_l2 = 0;

    rng_state = 0xABCD1234;

    for (int n = 0; n < RANDOM_COUNT; n++) {
        rand_vec(a, TEST_DIM);
        rand_vec(b, TEST_DIM);

        // dot_product
        float simd_dot = dot_product(a, b, TEST_DIM);
        double ref_dot = 0.0;
        for (uint32_t i = 0; i < TEST_DIM; i++) ref_dot += (double)a[i] * (double)b[i];
        float err_dot = fabsf(simd_dot - (float)ref_dot);
        if (err_dot > max_err_dot) max_err_dot = err_dot;
        ok(err_dot <= tolerance((float)ref_dot, TEST_DIM),
           "dot_product vs double ref", simd_dot, (float)ref_dot);

        // cosine_distance
        float simd_cos = cosine_distance(a, b, TEST_DIM);

        // double-precision cosine reference
        double ddot = 0.0, dna2 = 0.0, dnb2 = 0.0;
        for (uint32_t i = 0; i < TEST_DIM; i++) {
            double ai = (double)a[i], bi = (double)b[i];
            ddot += ai * bi;
            dna2 += ai * ai;
            dnb2 += bi * bi;
        }
        float ref_cos;
        if (dna2 < 1e-12 || dnb2 < 1e-12) {
            ref_cos = 1.0f;
        } else {
            double cs = ddot / sqrt(dna2 * dnb2);
            if (cs > 1.0) cs = 1.0;
            if (cs < -1.0) cs = -1.0;
            ref_cos = (float)(1.0 - cs);
        }
        float err_cos = fabsf(simd_cos - ref_cos);
        if (err_cos > max_err_cos) max_err_cos = err_cos;
        ok(err_cos <= tolerance(ref_cos, TEST_DIM),
           "cosine_distance vs double ref", simd_cos, ref_cos);

        // l2_distance
        float simd_l2 = l2_distance(a, b, TEST_DIM);
        double ref_l2_sq = 0.0;
        for (uint32_t i = 0; i < TEST_DIM; i++) {
            double d = (double)a[i] - (double)b[i];
            ref_l2_sq += d * d;
        }
        float ref_l2 = (float)sqrt(fmax(ref_l2_sq, 0.0));
        float err_l2 = fabsf(simd_l2 - ref_l2);
        if (err_l2 > max_err_l2) max_err_l2 = err_l2;
        ok(err_l2 <= tolerance(ref_l2, TEST_DIM),
           "l2_distance vs double ref", simd_l2, ref_l2);
    }

    printf("  max_err  dot=%.2e  cos=%.2e  l2=%.2e\n", max_err_dot, max_err_cos, max_err_l2);

    free(a);
    free(b);
}

/* --- 7. Dim != 16 multiples: exercise the scalar tail --- */

static void test_odd_dims(void) {
    fn_type = "odd-dim";
    uint32_t dims[] = {1, 7, 17, 127, 255, 333};
    int ndims = sizeof(dims) / sizeof(dims[0]);

    for (int d = 0; d < ndims; d++) {
        uint32_t dim = dims[d];
        float *a = malloc(dim * sizeof(float));
        float *b = malloc(dim * sizeof(float));
        rand_vec(a, dim);
        rand_vec(b, dim);

        // dot vs double
        float sdot = dot_product(a, b, dim);
        double ref = 0.0;
        for (uint32_t i = 0; i < dim; i++) ref += (double)a[i] * (double)b[i];
        ok(fabsf(sdot - (float)ref) <= tolerance((float)ref, dim),
           "odd-dim dot vs double ref", sdot, (float)ref);

        // l2 vs double
        float sl2 = l2_distance(a, b, dim);
        double ref2 = 0.0;
        for (uint32_t i = 0; i < dim; i++) {
            double d = (double)a[i] - (double)b[i];
            ref2 += d * d;
        }
        float ref_l2 = (float)sqrt(fmax(ref2, 0.0));
        ok(fabsf(sl2 - ref_l2) <= tolerance(ref_l2, dim),
           "odd-dim l2 vs double ref", sl2, ref_l2);

        // cosine vs double
        float scos = cosine_distance(a, b, dim);
        double ddot = 0.0, dna2 = 0.0, dnb2 = 0.0;
        for (uint32_t i = 0; i < dim; i++) {
            double ai = (double)a[i], bi = (double)b[i];
            ddot += ai * bi;
            dna2 += ai * ai;
            dnb2 += bi * bi;
        }
        float ref_cos;
        if (dna2 < 1e-12 || dnb2 < 1e-12) {
            ref_cos = 1.0f;
        } else {
            double cs = ddot / sqrt(dna2 * dnb2);
            if (cs > 1.0) cs = 1.0;
            if (cs < -1.0) cs = -1.0;
            ref_cos = (float)(1.0 - cs);
        }
        ok(fabsf(scos - ref_cos) <= tolerance(ref_cos, dim),
           "odd-dim cosine vs double ref", scos, ref_cos);

        free(a);
        free(b);
    }
}

/* --- main --- */

int main(void) {
    printf("SIMD distance kernel tests\n\n");

    test_determinism();
    printf("  determinism: %s\n", failures == 0 ? "PASS" : "FAIL");
    int saved = failures; failures = 0;

    test_symmetry();
    printf("  symmetry: %s\n", failures == 0 ? "PASS" : "FAIL");
    saved += failures; failures = 0;

    test_identity();
    printf("  identity: %s\n", failures == 0 ? "PASS" : "FAIL");
    saved += failures; failures = 0;

    test_orthogonal();
    printf("  orthogonal: %s\n", failures == 0 ? "PASS" : "FAIL");
    saved += failures; failures = 0;

    test_zero();
    printf("  zero-vector: %s\n", failures == 0 ? "PASS" : "FAIL");
    saved += failures; failures = 0;

    test_random_vs_double();
    printf("  random-vs-double (%d-dim, %d pairs): %s\n",
           TEST_DIM, RANDOM_COUNT, failures == 0 ? "PASS" : "FAIL");
    saved += failures; failures = 0;

    test_odd_dims();
    printf("  odd-dimensions (1,7,17,127,255,333): %s\n", failures == 0 ? "PASS" : "FAIL");
    saved += failures;

    printf("\n");
    if (saved == 0) {
        printf("All tests PASSED.\n");
        return 0;
    } else {
        printf("%d test(s) FAILED.\n", saved);
        return 1;
    }
}
