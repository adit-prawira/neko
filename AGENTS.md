# AGENTS.md

## IMPORTANT 
- Talk efficiently. Nice and short. Stop bullshitting
- Everthing must be in concise and simple bullet points
- Only do what I ask you to do. No more no less. 
- Stop assuming, you don't know me. I will tell you what I want. Not what you want.
- When I ask to check my change. Literally only check my implementation code change. Stop worrying about formatting shit because we have formatter to handle that 
- Bug is something that will break the system, in correct logic. Fucking nitpick or formatting is not a bug
## Onboarding
Read `docs/PRD.md`, `docs/ARCHITECTURE.md`, and the latest `.opencode/handoff-*.md` (newest first). Phase doc index lives in `docs/phases/` (`PHASE_0`, `PHASE_0_5`, `PHASE_1`, `PHASE_2`, `PHASE_3`). Handoff files are the source of truth for project state; do not hardcode progress here.

## Build, Test & Format Commands
The root `Makefile` orchestrates all three layers in order:

```
make build    # swag → proto → simd → cargo build --release → go build
make test     # simd → cargo test → go test ./...
make clean    # simd clean → cargo clean → rm neko + rm -rf internal/gen/neko
make format   # cargo fmt → go fmt ./... → swag fmt → clang-format -i proto/*.proto
make swag     # regenerate internal/api/docs/ (OpenAPI spec) from handler.go annotations
make proto    # regenerate internal/gen/neko/v1/*.pb.go from proto/neko.proto
```

`make build` depends on `make swag` and `make proto`, which require the following CLIs:

```
go install github.com/swaggo/swag/cmd/swag@v1.16.3
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Plus `protoc` itself on PATH (apt: protobuf-compiler, brew: protobuf)
```

Without these, `make build` fails at the `swag` or `proto` step. CI installs them before `make build` (see CI note below).

Individual layer commands still work:
```
cd simd && make test              # SIMD kernel correctness (NEON on arm64, AVX2 on x86_64)
cd engine && cargo build          # Rust crate compilation
cd engine && cargo test           # Rust unit + integration tests
cd engine && cargo fmt -- --check # Rust formatting check only (CI. use `cargo fmt` to apply)
go test ./...                     # Go tests (requires pre-built .dylib — run make build first)
```

**Go tests require a compiled engine.** The cgo directive in `internal/ffi/bridge.go` links against `engine/target/release/libneko_engine.dylib`. If you skip `make build` and run `go test ./...` standalone, link errors will occur. Use `make test` to run the full chain, or use the explicit `CGO_LDFLAGS` from the Makefile.

The engine is a library (`crate-type = ["cdylib", "staticlib"]`), not a binary — there is no `main.rs`. Release builds use LTO, `opt-level = "z"`, `codegen-units = 1`, `panic = "abort"`.

## Repo Gotchas
- Go module path: `github.com/adit-prawira/neko` (Go 1.25.0). CLI depends on `github.com/spf13/cobra`. gRPC server uses `google.golang.org/grpc`; transport multiplexing uses `github.com/soheilhy/cmux`.
- `.gitignore` ignores all `*.md` except `README.md`, `docs/BACKGROUND.md`, `docs/ARCHITECTURE.md`, `docs/API.md`. Use `git add -f` for AGENTS.md, handoff files, PRD, and phase docs.
- Rust edition is **2024** (not the more common 2021).
  - **`#[unsafe(no_mangle)]`** is REQUIRED instead of `#[no_mangle]` on all FFI extern functions. IDE autocompletions and older Rust tooling default to `#[no_mangle]` — this will fail to compile.
  - Raw pointer dereferences in extern functions require explicit `unsafe { }` blocks around each operation. This is NOT the case in Rust 2021.
- `engine/Cargo.lock` is committed (treat as application, not library, for reproducible builds).
- `engine/build.rs` compiles `simd/distance.c` into the Rust `.dylib` via the `cc` crate with arch-specific SIMD flags (`-march=armv8-a+simd` on aarch64, `-mavx2 -mfma` on x86_64).
- `simd/Makefile` uses `cc` (not `gcc`) — this resolves to clang on macOS and gcc on Linux.
- **FFI structs use `#[repr(C)]`** and must match `internal/ffi/bridge.h` byte-for-byte. Rust `engine/src/shared/results.rs` defines `NekoStats` and `NekoSearchResult` with C layout. The Go `bridge.go` mirrors these with cgo types.
- **Metric codes must stay in sync** across three layers: `MetricL2 = 0, MetricCosine = 1, MetricDot = 2` is defined in Go (`internal/ffi/bridge.go`) and Rust (`engine/src/segment/resource.rs` — the `Metric` enum with `#[repr(u8)]`). The proto3 enum (`proto/neko.proto`) reserves `0` for `METRIC_UNSPECIFIED` and shifts the L2 / Cosine / Dot values by +1 (`METRIC_L2=1`, `METRIC_COSINE=2`, `METRIC_DOT=3`); the gRPC shim in `internal/grpc/dto.go` does an explicit switch — never pass `uint8(metric)` through, and never call `ffi.ParseMetric` (string-only) on the enum. Adding a metric requires updates in all three places.
- `Hairball` error enum uses `#[repr(u32)]` — errors cross the FFI boundary as raw `int32`; Go reads integer codes with zero marshaling.
- **Rust** tests are inline — `#[cfg(test)] mod tests { ... }` at the bottom of each `.rs` file. Rust test names follow `given_<scenario>_then_<expectation>`.
- **Go** tests use standard Go convention — `TestXxx` top-level functions with `t.Run("description", ...)` subtests. Go tests live in `*_test.go` files alongside their package, not inline.
- `engine/rustfmt.toml` sets `max_width = 180`.
- Tests create temp directories under `std::env::temp_dir()` with the `neko_test_*` prefix — clean these up if a test run leaves orphans.
- `engine/.rust-analyzer.toml` sets `allTargets = true`.
- `simd/` build artifacts are gitignored via `*.o` and `simd/test_*` patterns. Run `make clean` before committing to keep the workspace clean.
- **CI**: `.github/workflows/ci.yml` runs three jobs on push/PR to `main`. `simd` and `engine` run on both ubuntu (x86_64) and macos (arm64). `cli` runs only on ubuntu-latest. The cli job installs `swag@v1.16.3`, `protoc` (apt: `protobuf-compiler`), `protoc-gen-go`, and `protoc-gen-go-grpc`; exports `$(go env GOPATH)/bin` to `$GITHUB_PATH` so the installed tools are on `PATH`; runs `go mod download` to pre-populate the module cache; then runs `make build` (which depends on `make swag` and `make proto`), then `go test ./...` with explicit `CGO_LDFLAGS`, then a `verify` step that runs `./neko version | grep "neko v0.1.0"` to confirm the binary embeds the right version.
- `internal/api/docs/` is generated by `make swag` and is **committed** to the repo (the spec is embedded in the binary at runtime). Do not hand-edit — regenerate via `make swag` after any handler annotation change.
- `internal/gen/neko/v1/*.pb.go` is generated by `make proto` and is **gitignored** (regenerated on every `make build`). The directory itself is tracked via `internal/gen/.gitkeep`. Do not hand-edit — regenerate via `make proto` after any `proto/neko.proto` change.

- **`.opencode/`** contains a local opencode plugin (`package.json`, `node_modules/`) — this is gitignored and has nothing to do with the neko build. Ignore it.

## Writing Rules [NEVER MODIFY]
- **Only write directly to `.md`, `.gitignore`, `.yaml`, `.yml` and test files.**
- **For implementation code, only recommend diffs — never write directly.** Changes to source files (`.go`, `.rs`, `.c`, `.toml`, `Makefile`, `go.mod`, `Cargo.toml`, etc.) must be proposed as code diffs for review, not applied automatically.
- **When proposing diffs, explain what the code does and why.** The codebase is also a learning resource for low-level systems programming — diffs should include context on algorithms, SIMD patterns, and FFI mechanics.
- **Never use Write or Edit tools on source files under any circumstances.** These tools are restricted to `.md`, `.gitignore`, and test files only. All source changes go through the user.
- Avoid using ambiguous variable names like `e`, `a`, or `i` (single character variable names)
- Code should read like English — prefer explicit, descriptive names
- Never add the `ready-for-agent` label to issues.

## Handoff [NEVER MODIFY]
- When creating handoff documents put it into current directory .opencode/ and follow the file name format for uniqueness (e.g., `handoff-YYYY-MM-DD-session-N.md`).

## Architecture
- **Go binary**: `cmd/neko/main.go` is a 15-line shim that hands off to `pkg/cli/` (cobra: `version`, `create`, `list`, `drop`, `insert`, `get`, `search`, `delete`, `upsert`, `serve`, `stats`). REST handlers live in `internal/api/` (`handler.go`, `router.go`, `middleware.go`). The gRPC server factory lives in `internal/grpc/` (`server.go` — reflection registered; `RegisterServices` wires the CollectionService and VectorService handlers from PR #38). The shared lifecycle (cmux listener, signal handling, graceful shutdown of REST + gRPC on the same port) lives in `internal/server/` (`core.go`). Go-side cross-cutting helpers live in `internal/shared/` (mirrors Rust `engine/src/shared/`).
- **Proto contract**: `proto/neko.proto` defines `CollectionService` (Create / List / Get / Drop / Search) and `VectorService` (Insert / InsertMany / Get / Upsert / Delete). Generated Go stubs in `internal/gen/neko/v1/` (gitignored).
- **Rust entrypoint**: `engine/src/lib.rs` holds the `#[unsafe(no_mangle)] pub extern "C" fn neko_*` exports. There is no `main.rs` — the crate is `crate-type = ["cdylib", "staticlib"]`.
- **Rust domains** under `engine/src/`: `engine/` (singleton `ENGINE`, knn, validator), `segment/` (vectors + `Metric` enum with `#[repr(u8)]`), `wal/` (write-ahead log), `manifest/` (segment registry), `shared/` (`Hairball` `#[repr(u32)]`, `NekoStats`/`NekoSearchResult`/`NekoMetadata` `#[repr(C)]`).
- **C SIMD kernels**: `simd/distance.c` compiled into the Rust `.dylib` via `engine/build.rs` + `cc` crate.
- **Build chain**: `cc simd/distance.c` → `cargo build --release` (links C into `libneko_engine.dylib`) → `go build -o neko ./cmd/neko` (cgo links the dylib).
- **Domain-Driven Source Structure:**
  - Each domain is a folder under `engine/src/` (e.g., `segment/`, `wal/`, `manifest/`)
  - Each domain folder must have a `mod.rs` that only declares sub-modules
  - Types owned by a domain live in `resource.rs` within that domain folder
  - Cross-domain shared code (errors, FFI decls, constants) lives in the `shared/` domain
  - No shared `types.rs` grab-bag file — every type has an owning domain
- Engine is a global singleton: `pub static ENGINE: OnceLock<RwLock<Engine>>` in `engine/src/engine/engine.rs`. Wire all FFI calls through it; do not create per-call instances.

## Naming Convention (cat theme)
- Collection → **clowder** (in Rust engine structs)
- Error → **hairball** (e.g., `HAIRBALL_NOT_FOUND`, `HAIRBALL_DIM_MISMATCH`)
- WAL → **tail** (e.g., `tail.log`, `tail.NNN.log`)

## Key Defaults
- Port: `3434`, Data dir: `~/.neko/`, API prefix: `/v1/`
- Config: `config.toml` (port, data_dir, log_level, wal_rotate_mb, max_segments) — planned, not yet implemented
- Env: `NEKO_HOME` overrides data dir; CLI: `neko serve --data-dir <path>` overrides both (flag is currently serve-only, not global).

## FFI Surface
Exported from `engine/src/lib.rs` (all use `#[unsafe(no_mangle)] pub extern "C"`, return `i32` Hairball code or 0):

- **Lifecycle:** `neko_version`, `neko_init`, `neko_shutdown`
- **Collections:** `neko_create`, `neko_list`, `neko_drop`, `neko_stats`
- **Vectors:** `neko_insert`, `neko_get`, `neko_insert_many` (takes `*const NekoInputVector` array; `NekoInputVector` is `#[repr(C)]` with `id`, `vector`, `dim`, `metadata` fields), `neko_upsert` (writes `created: u8`), `neko_delete`, `neko_get_vector` (returns `NekoMetadata`), `neko_free_metadata`
- **Search:** `neko_search`, `neko_free_result`
- **Memory:** `neko_free_strings`

Note: `docs/ARCHITECTURE.md` FFI surface still lists `neko_embed*` and `neko_load_model` as planned (Phase 2+) — those are **not in `lib.rs` yet**. The Concurrency Model, Rust Engine State, and Memory Model sections in that doc are now current; see **Known Doc Drift** below for what still drifts. Always read `engine/src/engine/resource.rs` and `engine/src/lib.rs` for the live state.

## REST Surface
Routes in `internal/api/router.go` — all under `/v1/` except `/health`. Live OpenAPI spec served at `/swagger/doc.json`, Swagger UI at `/swagger/index.html`:

| Method | Path | Handler |
|--------|------|---------|
| GET | `/health` | `HandleHealth` |
| POST | `/v1/collections` | `HandleCreateCollection` |
| GET | `/v1/collections` | `HandleGetCollections` |
| GET | `/v1/collections/{name}` | `HandleGetCollection` |
| DELETE | `/v1/collections/{name}` | `HandleDropCollection` |
| POST | `/v1/collections/{name}/search` | `HandleSearchCollection` |
| POST | `/v1/collections/{name}/vectors` | `HandleInsertVector` |
| POST | `/v1/collections/{name}/vectors/batch` | `HandleInsertManyVector` (bare JSON array, capped at 10,000) |
| GET | `/v1/collections/{name}/vectors/{id}` | `HandleGetVector` |
| PUT | `/v1/collections/{name}/vectors/{id}` | `HandleUpsertVector` (201 new / 200 updated) |
| DELETE | `/v1/collections/{name}/vectors/{id}` | `HandleDeleteVector` |
| GET | `/swagger/*` | `httpSwagger.Handler` (UI + spec) |

## Workflow Conventions
Established across PRs #29–#34 (REST endpoints), #37 (gRPC scaffold), and #38 (gRPC handlers):

- **One endpoint per PR.** Branch naming: `feature/<verb>-endpoint` (e.g., `feature/delete-endpoint`).
- **Go 1.22+ `ServeMux` contract:** path segments are non-empty by routing contract. **Don't add `if id == ""` or `if name == ""` guards** to handlers that extract from the path — match `HandleGetVector` / `HandleUpsertVector` style.
- **Vector-CRUD handlers:** `id` comes from the path; the body has only `vector` + optional `metadata`. Do not put `id` in the body.
- **OpenAPI annotations are required for every new endpoint.** Add a swag comment block (`@Summary`, `@Tags`, `@Param`, `@Success`, `@Failure`, `@Router`) above the handler. Use the **full path** in `@Router` (e.g. `/v1/collections/{name}/vectors/{id}`, not `/collections/...`) — there is no `@BasePath`. Then add the `(method, path)` pair to the `expectedRoutes` list in `internal/api/docs_test.go` and run `make swag` before opening the PR. The drift test in that file fails the build if any route is missing from the spec (or vice versa).
- **gRPC proto messages mirror REST DTOs exactly.** The source of truth is `internal/api/handler.go`'s `*HttpDTO` structs. Proto message name = REST DTO name with `HttpDTO` stripped, snake_cased. Type mapping: `string`→`string`, `uint32`→`uint32`, `int`→`int32`, `uint64`→`uint64`, `float32`→`float`, `*uint32`→`optional uint32`, `json.RawMessage`→`bytes`. The only structural changes are transport-forced: path params become message fields, bare arrays become wrapped messages. New proto messages without a matching REST DTO are a design smell.
- **PR description template:** three sections — Context (WHY, 1–3 sentences), Changes (logical groups, not per-file), Tests (categories, not individual names). No "This PR…" opener.
- **Skill order for new endpoint work:** `test-audit` first (established pattern in #29–#32) → `grill-me`/`grilling` for non-trivial interface decisions → `implement` → `commit-message` + `pr-description` at end → `sync-docs` after merge.

## Known Doc Drift
- **`docs/ARCHITECTURE.md`** — the high-level architecture diagram and "Disk Layout" block still depict the **target** shape (HNSW, IVF, PQ, mmap'd segment files, ONNX Runtime). These are all Phase 1+ deliverables. The current v0.1.0 engine is in-memory only — see the "Memory Model" section in that file for the current state, and `engine/src/engine/resource.rs` for the live struct. The "Rust Engine State", "Concurrency Model", "Memory Model", "Build Chain", and "Transport Multiplexing" sections are now correct as of the gRPC scaffold commit (`cf83537`); the diagram and Disk Layout remain forward-looking and need to be marked "(target)" or split out when Phase 1 lands.
- **`engine/Cargo.toml`** declares `memmap2 = "0.9"` and `engine/src/segment/reader.rs` imports it, but the v0.1.0 engine path doesn't actually mmap anything — `Clowder` stores vectors and metadata in plain `HashMap`s. The mmap reader code is in tree but not wired into `Engine::insert/get/search`. Resolves naturally when Phase 0.5 storage hardening (`#8`) lands segments.
