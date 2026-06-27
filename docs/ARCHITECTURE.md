# Neko — Architecture

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                       SINGLE STATIC BINARY                       │
│                   brew install neko                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────┐  ┌──────────────────────────────────┐ │
│  │   TUI (Go)           │  │   API Server (Go)                │ │
│  │   Bubble Tea         │  │   • REST (net/http)              │ │
│  │   • Collection view  │  │   • gRPC (protobuf)              │ │
│  │   • Search UI        │  │   • Metadata filtering (BTree)   │ │
│  │   • Live stats       │  │   • Multi-model management       │ │
│  └──────────┬───────────┘  └──────────────┬───────────────────┘ │
│             │                             │                      │
│             └──────────┬──────────────────┘                      │
│                        │ cgo FFI (C ABI)                         │
│                        ▼                                         │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │   Index Engine (Rust)                                        │ │
│  │   • HNSW — layered proximity graph, beam search              │ │
│  │   • IVF — inverted file index (for billion-scale)           │ │
│  │   • Product Quantization (PQ48, PQ24)                       │ │
│  │   • ONNX Runtime bindings → local embedding model            │ │
│  ├─────────────────────────────────────────────────────────────┤ │
│  │   Storage Layer (Rust)                                       │ │
│  │   • LSM-tree segments (immutable, sorted)                    │ │
│  │   • mmap'd vector files (zero-copy reads)                    │ │
│  │   • WAL — write-ahead log for crash safety                   │ │
│  │   • Compaction — background segment merging                  │ │
│  │   • Manifest — segment list + stats                          │ │
│  └──────────────────────────────┬──────────────────────────────┘ │
│                                 │ extern "C"                      │
│                                 ▼                                │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │   SIMD Kernels (C)                                           │ │
│  │   • dot_product     (AVX2 / NEON)                            │ │
│  │   • cosine_distance (AVX2 / NEON)                            │ │
│  │   • l2_distance     (AVX2 / NEON)                            │ │
│  │   • batch_distance   (matrix × matrix, unrolled)             │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Language Boundaries & Build Chain

| Boundary | Method | Why |
|----------|--------|-----|
| Go ↔ Rust | cgo (C ABI) | Go main binary links Rust `cdylib`. Nanosecond FFI overhead. |
| Rust ↔ C SIMD | `extern "C"` + `cc` crate in `build.rs` | C kernels compiled into the Rust `.dylib`. Linked at Rust compile time. |
| Go → Go | Native | API server, CLI, TUI, clustering |

**Build chain:**
```
make
  1. cc simd/distance.c → simd/distance.o
  2. cargo build --release (build.rs links distance.o into libneko_engine.dylib)
  3. go build -o neko ./cmd/neko (cgo links libneko_engine.dylib)
  → Single static binary: ./neko
```

## Disk Layout

```
~/.neko/
  collections/
    docs/
      wal/              ← append-only write-ahead log
      segments/
        s_00001.vec     ← raw f32 vectors (mmap'd)
        s_00001.idx     ← HNSW adjacency lists
        s_00001.meta    ← metadata
      manifest.json     ← active segments + compaction state
      index.meta        ← HNSW params (M, ef_construction, ef_search)
  models/
    models.toml              ← model registry (name → path, dim, language)
    all-MiniLM-L6-v2.onnx    ← ~90 MB, default model, bundled at build time
    bge-small-en-v1.5.onnx   ← downloaded on demand via `neko models pull`
  config.toml
```

## Model Strategy

Neko supports multiple embedding models. Each collection is bound to a specific model
at creation time — the model determines the vector dimension.

**Bundled default:** `all-MiniLM-L6-v2` (384-dim, ~90 MB, ~45 MB gzip). Packaged in the
binary via `go:embed`. Available out of the box — no download needed.

**Download on demand:** Additional models are pulled from HuggingFace via `neko models pull`:

```
neko models pull bge-small-en-v1.5    # 384-dim, best MTEB for small models
neko models pull gte-small            # 384-dim, strong retrieval
neko models pull multilingual-e5-small # 384-dim, 100+ languages
neko models pull all-MiniLM-L12-v2   # 384-dim, better quality
```

**Model registry** (`~/.neko/models/models.toml`):
```toml
[all-MiniLM-L6-v2]
path = "all-MiniLM-L6-v2.onnx"
dim = 384
language = "en"
bundled = true

[bge-small-en-v1.5]
path = "bge-small-en-v1.5.onnx"
dim = 384
language = "en"
bundled = false
```

**Collection binding:**
```bash
neko create docs --model all-MiniLM-L6-v2            # dim inferred (384)
neko create legal --model bge-small-en-v1.5          # same dim, better quality
neko create docs --model multilingual-e5-small --metric cosine
```

`--dim` becomes optional — when `--model` is set, dimension is inferred from the model.
If neither `--model` nor `--dim` is provided, the default model (`all-MiniLM-L6-v2`) is used
with 384 dimensions.

## Search Data Flow

```
 User query: "hello world"
        │
        ▼
 Go API — parse query, validate collection
        │
        ▼  (Phase 2+) ONNX Runtime → embed text → [f32; 384]
        │  (Phase 0)  Raw vector from CLI/API directly
        ▼
  Rust brute-force KNN → C SIMD dot_product() on all vectors
        │       (Phase 1+) HNSW beam search across layered graph
        │       batch eval → C batch_distance() (4x unrolled matrix kernel)
        ▼
 Rust returns top-k (id, score) pairs
        │
        ▼
 Go API — lookup metadata from BTree, format response
        │
        ▼
 HTTP JSON / CLI output / TUI render
```

## Rust FFI API Surface

```rust
// engine/src/lib.rs

// — Lifecycle —
#[no_mangle] pub extern "C" fn neko_init(data_dir: *const c_char) -> i32;
#[no_mangle] pub extern "C" fn neko_shutdown() -> i32;

// — Collections —
// model may be null → uses default model (all-MiniLM-L6-v2)
#[no_mangle] pub extern "C" fn neko_create(name: *const c_char, dim: u32,
                                             metric: u8, model: *const c_char) -> i32;
#[no_mangle] pub extern "C" fn neko_drop(name: *const c_char) -> i32;
#[no_mangle] pub extern "C" fn neko_list_collections(
    names: *mut *mut c_char, count: *mut u32) -> i32;
#[no_mangle] pub extern "C" fn neko_collection_stats(
    name: *const c_char, stats: *mut NekoStats) -> i32;

// — Vectors —
// len = number of f32 elements (must equal collection dim)
#[no_mangle] pub extern "C" fn neko_insert(name: *const c_char, id: *const c_char,
                                             vector: *const f32, len: u32,
                                             metadata: *const c_char) -> i32;
// lens[i] = number of f32 elements in vectors[i] (must equal collection dim)
#[no_mangle] pub extern "C" fn neko_batch_insert(name: *const c_char,
                                                   ids: *const *const c_char,
                                                   vectors: *const f32,
                                                   lens: *const u32,
                                                   metadatas: *const *const c_char,
                                                   count: u32) -> i32;
#[no_mangle] pub extern "C" fn neko_upsert(name: *const c_char, id: *const c_char,
                                             vector: *const f32, len: u32,
                                             metadata: *const c_char) -> i32;
#[no_mangle] pub extern "C" fn neko_delete(name: *const c_char, id: *const c_char) -> i32;

// — Search —
// filter may be null → no metadata filtering
#[no_mangle] pub extern "C" fn neko_search(name: *const c_char, query: *const f32,
                                             dim: u32, top_k: u32,
                                             filter: *const c_char,
                                             results: *mut NekoResult) -> i32;

// — Memory —
#[no_mangle] pub extern "C" fn neko_free_result(results: *mut NekoResult);
#[no_mangle] pub extern "C" fn neko_free_strings(strings: *mut *mut c_char, count: u32);

// — Embeddings (Phase 2+) —
#[no_mangle] pub extern "C" fn neko_embed(model_name: *const c_char,
                                            text: *const c_char,
                                            vector: *mut f32,
                                            dim: *mut u32) -> i32;
#[no_mangle] pub extern "C" fn neko_embed_batch(model_name: *const c_char,
                                                  texts: *const *const c_char,
                                                  count: u32,
                                                  vectors: *mut f32) -> i32;

// — Model management (Phase 2+) —
#[no_mangle] pub extern "C" fn neko_load_model(name: *const c_char,
                                                 path: *const c_char) -> i32;
#[no_mangle] pub extern "C" fn neko_unload_model(name: *const c_char) -> i32;
```

## Concurrency Model

- **Rust:** Single-threaded index per collection (RWLock). Search is read-parallel via `rayon`.
- **Go:** Standard `net/http` goroutine-per-request model. API calls serialize through Rust FFI (mutex-guarded).
- **C:** Stateless SIMD kernels. No threading concerns.

## Rust Engine State

The Rust engine maintains a global singleton with interior mutability:

```rust
// engine/src/engine.rs
struct Clowder {
    name: String,
    dim: u32,
    metric: Metric,
    model: Option<String>,       // bound model name (Phase 2+)
    segments: Vec<Segment>,      // mmap'd, immutable
    wal: Mutex<WAL>,             // append-only, single-writer
    manifest: RwLock<Manifest>,  // segment list + compaction state
}

struct Engine {
    clowders: HashMap<String, Arc<Clowder>>,
    data_dir: PathBuf,
}

// Global singleton — initialized by neko_init
static ENGINE: OnceLock<RwLock<Engine>> = OnceLock::new();
```

Locking strategy:
- `Engine.clowders`: `RwLock` — read for most operations, write for create/drop
- `Clowder.wal`: `Mutex` — serializes writes (only one thread appends to WAL)
- `Clowder.manifest`: `RwLock` — read for search, write for segment rotation

All FFI functions acquire the engine lock first, then operate on the clowder.
Insert path: `engine.read() → clowder.wal.lock() → append → release`
Search path: `engine.read() → clowder.segments (immutable, no lock) → SIMD scan → release`

## WAL Rotation & Compaction

WAL files are append-only for write durability, but they grow unbounded.
Neko uses threshold-based rotation:

1. Active WAL writes to `wal/tail.log` — all inserts/deletes append here.
2. When WAL exceeds **64 MB** (configurable via `config.toml`: `wal_rotate_mb`):
   a. Freeze current WAL, rename to `wal/tail.NNN.log`
   b. Create a new, empty `wal/tail.log`
   c. Background compaction: replay frozen WAL → sort by ID → write immutable segment
   d. After segment is written + fsynced, delete the frozen WAL file
   e. Update `manifest.json` to include the new segment
3. On crash/restart:
   a. Replay `wal/tail.log` (active WAL) into memory
   b. Also replay any frozen WALs not yet compacted
   c. Load all segments listed in `manifest.json`
4. Compaction merges multiple small segments into fewer larger ones:
   a. Triggered when segment count exceeds `max_segments` (default 32)
   b. Uses size-tiered strategy: merge N segments of similar size
   c. Merged segment replaces source segments in manifest atomically

This keeps WAL bounded, segments optimized for mmap reads, and crash safety intact.

## Memory Model

- Vectors live in mmap'd files. OS page cache handles read caching.
- HNSW graph lives in Rust heap (Arc<RwLock<>>).
- WAL is append-only, flushed on insert. Replayed on startup for crash recovery.
- Metadata lives in Go-side BTree for fast string matching.
