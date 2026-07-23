#include <stdint.h>

typedef struct {
  uint64_t vector_count; 
  uint32_t dim; 
  uint8_t metric;
  uint64_t storage_bytes; 
  uint8_t index_type;
} NekoStats;

int32_t neko_version(void);
int32_t neko_init(const char *data_directory);
int32_t neko_shutdown(void);
int32_t neko_create(const char *name, uint32_t dim, uint8_t metric, const char *model);
int32_t neko_list(char ***names, uint32_t *count);
int32_t neko_drop(const char *name);
int32_t neko_stats(const char *name, NekoStats *stats);
int32_t neko_insert(const char *name, const char *id, const float *vector, uint32_t len, const char *metadata);
int32_t neko_get(const char *name, const char *id, float *vector, uint32_t dim);
void neko_free_strings(char **strings, uint32_t count);
