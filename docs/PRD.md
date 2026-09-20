# Neko — Product Requirements Document

## 1. Purpose

Neko is a **local-first vector database** that developers install on their machine and use immediately. It serves the same role for vector search that SQLite serves for relational data: zero-setup, zero-cost, single-file, embeddable.

## 2. Core Values

| Value | What It Means |
|-------|---------------|
| **Zero cost** | No cloud account, no API keys, no billing. The embedding model runs locally. Storage is your own disk. |
| **Zero friction** | `brew install neko`. One binary. No Docker, no config file required, no cluster to manage. |
| **Blazing fast** | Rust engine with SIMD-accelerated distance math (AVX2, NEON). HNSW indexing for sub-millisecond search on millions of vectors. |
| **Joyful** | Terminal tools should be fun. Cat theme, TUI dashboard, warm CLI. Inspired by the Charmbracelet ecosystem. |
| **Correct** | Rust's ownership model for the storage engine. Crash-safe via write-ahead logging. |
| **Polyglot respect** | Built with the best tool for each job: Go for networking, Rust for engine, C for SIMD. No single-language dogmatism. |

## 3. Target Users

- **AI/ML developers** prototyping RAG (Retrieval-Augmented Generation) apps.
- **CLI tool authors** who want vector search inside a Go/Rust binary.
- **Indie hackers** building search features without a cloud bill.
- **Students & learners** exploring vector search internals.

## 4. Why Not Existing Solutions?

| Alternative | Problem |
|-------------|---------|
| **Pinecone / Weaviate Cloud** | Requires account, API key, internet, payment method. |
| **Qdrant / Milvus** | Excellent, but require Docker or manual native install. Heavy for a laptop. |
| **FAISS (Meta)** | Library, not a database. No persistence, no API, no clustering. |
| **Chroma** | Python-only. No self-contained binary. Embedding API required. |
| **LanceDB** | Rust-based, but still requires Python/JS bindings for typical use. |

**Neko's niche:** A self-contained binary database with a built-in embedding model, REST API, and TUI. No Python. No Docker. No keys.

## 5. Feature Scope

### Phase 0 — Core Storage & Search (v0.1)
- [ ] Rust storage engine (LSM-like segments, mmap reads, WAL) — current state: vectors are held in `HashMap<String, Vec<f32>>` in memory; the WAL writes to `tail.log` and survives restart. LSM segments are not yet built.
- [x] C SIMD distance kernels (dot product, cosine, L2)
- [x] Brute-force KNN search with SIMD
- [x] Go HTTP REST API server with `/v1/` prefix (create/drop collections, insert/search/upsert/delete/batch-insert vectors) — current state: collections (create/list/get/drop), insert, search, get-by-id, upsert, delete, and batch insert (up to 10,000 vectors per call, single FFI dispatch) are all wired (PRs `#27`–`#33`).
- [x] OpenAPI / Swagger UI for the REST surface — generated from `// @`-comment annotations on each handler in `internal/api/handler.go` via [swaggo](https://github.com/swaggo/swag). Spec is regenerated on every `make build`; served at `/swagger/doc.json` with the interactive UI at `/swagger/index.html`. Drift between routes and spec is caught by `internal/api/docs_test.go` at test time.
- [x] Go gRPC API server (same endpoints, protobuf-based) — current state: proto contract in `proto/neko.proto`, codegen via `make proto`, REST + gRPC multiplexed on `:3434` via cmux, gRPC server running with reflection registered (PR `#37`). 10 collection + vector RPCs shipped (PR `#38`). Full `HAIRBALL_*` → gRPC status coverage verified by `internal/grpc/errors_test.go` table-driven test, and the REST/gRPC search parity proven by `internal/grpc/integration_test.go` (PR `#39`).
- [x] CORS support (for browser-based clients)
- [x] Per-collection distance metric selection (cosine, dot, L2)
- [x] Batch insert support (single FFI call for multiple vectors) — REST endpoint `POST /v1/collections/:name/vectors/batch` (PR `#33`), Go bridge `InsertMany`, FFI `neko_insert_many` accepting an array of `NekoInputVector` records. Capped at 10,000 vectors per call at the REST layer.
- [x] Upsert support (insert or update by ID) — REST endpoint (PUT, 201 new / 200 updated), CLI, and FFI all wired (PRs #26, #31).
- [ ] Metadata filtering (pre-filter before ANN, BTree index) — accepted in the search request body but not yet evaluated.
- [x] API server with consistent JSON error format (cat-themed: "hairball" codes)
- [x] Health check endpoint (`GET /health`)
- [x] Collection info endpoints (`GET /collections`, `GET /collections/:name`) with vector count
- [x] Collection naming constraints enforced (alphanumeric + hyphen/underscore, max length, case-sensitive)
- [x] Dimension mismatch rejection on insert (hairball error)
- [x] Maximum dimension limit (default 4096, configurable)
- [x] Config file support (`config.toml`: port, data_dir, log_level, wal_rotate_mb, max_segments, max_dim) — loaded automatically from `<data-dir>/config.toml`; precedence is CLI flag > `NEKO_HOME` env > config file > built-in defaults; missing file falls back to defaults with no error (PR #46, issue #42).
- [x] Go CLI (`serve`, `create`, `insert`, `search`, `delete`, `stats`, `bench`, `version`) — current state: version, create, list, drop, insert, get, search, delete, upsert, serve, stats, and bench are wired (PR #44 stats, PR #50 bench). CLI is split per command into `pkg/cli/<verb>_command.go`; constructors are exported as `New<Verb>Cmd` (PR #51).
- [x] Configurable data directory (`NEKO_HOME` env var or `--data-dir` flag, promoted to a global persistent cobra flag since PR #48)
- [x] Auto-normalization of vectors on insert (configurable)
- [ ] Graceful shutdown (SIGTERM → flush WAL → close mmaps)
- [ ] Single static binary build

### Phase 1 — Indexing (v0.2)
- [ ] HNSW index (layered graph, beam search, pruning)
- [ ] Product quantization (PQ48, PQ24)
- [ ] HNSW index persistence (save graph to disk, reload on restart)
- [ ] Smarter shard-aware search strategy
- [ ] `neko tui` — Bubble Tea interactive dashboard

### Phase 2 — Embeddings (v0.3)
- [ ] ONNX Runtime integration (Rust bindings)
- [ ] Bundled default embedding model (`all-MiniLM-L6-v2`, 90 MB, 45 MB compressed)
- [ ] Multi-model support — model registry (`models.toml`), download on demand
- [ ] `neko models list|pull` — manage installed models
- [ ] Collection → model binding (`--model` flag, dim inferred from model)
- [ ] Supported models: all-MiniLM-L6-v2 (default), bge-small-en-v1.5, gte-small, multilingual-e5-small, all-MiniLM-L12-v2
- [ ] `neko embed` — generate embeddings from text/files
- [ ] Text search without pre-embedded vectors

### Phase 3 — Clustering (v0.4)
- [ ] Consistent hashing for shard placement
- [ ] Raft consensus for metadata
- [ ] Async replication (primary-follower WAL tailing)
- [ ] Multi-node support

### Future
- [ ] IVF index for billion-scale
- [ ] Disk-based HNSW (beyond memory)
- [ ] Memory budget / configurable limits
- [ ] Backup/restore (export/import collections)
- [ ] Prometheus metrics endpoint (`GET /metrics`)
- [ ] Benchmark suite (ANN-Benchmarks integration)
- [ ] Homebrew tap + Linux package repositories
- [ ] Client SDKs (Go, Rust, Python, JS)
- [ ] Helm chart for Kubernetes

## 6. Performance Targets

| Metric | v0.1 Target (brute force) | v0.2 Target (HNSW) |
|--------|--------------------------|---------------------|
| 384-dim vector insert | < 300 μs | < 300 μs |
| 1M vector search (top-10) | < 30 ms | < 1 ms |
| Memory overhead per 1M vectors | ~1.5 GB (raw) | ~2 GB (HNSW graph) |
| Storage per 1M vectors | ~1.5 GB | ~2 GB |

## 7. Non-Goals

- **Not a managed cloud service.** Neko runs on your machine.
- **Not a model training platform.** We bundle a model for embeddings only.
- **Not a general-purpose database.** Only vector search + metadata filtering.
- **Not ACID-compliant** in Phase 0-2. Fsync on WAL but no distributed transactions.

## 8. Design Principles

1. **One binary, no excuses.** Build from any language, ship one artifact.
2. **Defaults matter.** Sensible out-of-the-box. HNSW M=16, ef_construction=200.
3. **CLI first, GUI second.** API and CLI ship together. TUI is a bonus.
4. **Cat names.** Collection → `clowder`. Error → `hairball`. WAL → `tail`.
5. **No telemetry.** Ever.
