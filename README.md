# neko (猫)

**The local-first vector database that purrs.**

Neko is an open-source, single-binary vector database designed to run on your machine — no cloud, no API keys, no cost. Think SQLite for vectors.

## Why Neko?

Every AI app needs to search vectors. Existing options either lock you into a cloud service ($$$) or require a cluster of Docker containers just to prototype. Neko runs as a single static binary. `brew install neko` and you're done.

- **Zero cost** — no cloud account, no API keys, no billing. Storage is your own disk.
- **Single binary** — Go + Rust + C, fused. No Docker, no deps.
- **Familiar API** — REST today, gRPC collection + vector handlers on the same port (PR #38). Curl-friendly.
- **Cat-themed** — because terminal tools should bring joy.

## Quickstart

```bash
brew install neko

# Start the server
neko serve

# Create a collection, insert a vector, and search
neko create docs --dim 384
neko insert docs --id doc1 --file query.f32
neko search docs --file query.f32 --k 10

# Benchmark insert + search throughput
neko bench --vectors 100000 --dim 384 --k 10

# Or use the REST API
curl -X POST localhost:3434/v1/collections/docs/search \
  -H 'Content-Type: application/json' \
  -d '{"vector": [0.12, -0.34, 0.78], "top_k": 10}'

# Browse the API in Swagger UI
open http://localhost:3434/swagger/index.html
```

> **gRPC** — A second transport runs on the same port (3434) via cmux HTTP/2 multiplexing. The proto contract is in [`proto/neko.proto`](proto/neko.proto); generated Go stubs land in `internal/gen/neko/v1/`. PR #37 ships the scaffold (proto, codegen, multiplexed listener, reflection); PR #38 ships the 10 collection + vector RPCs. PR #39 ships full status-code coverage (`internal/grpc/errors_test.go`) and the REST/gRPC parity integration test (`internal/grpc/integration_test.go`). See the [gRPC section](#grpc) below for the developer workflow.

## Status (v0.1.0 — Phase 0)

| Surface | State |
|---|---|
| Brute-force KNN search (cosine / dot / L2) | Shipped |
| Collections (create / list / get / drop) | Shipped |
| Vectors (insert / get / upsert / delete / batch insert) | Shipped |
| WAL-backed durability + crash recovery | Shipped |
| SIMD distance kernels (NEON / AVX2) | Shipped |
| REST API (11 endpoints) | Shipped |
| OpenAPI spec + Swagger UI (`/swagger/`) | Shipped |
| gRPC API | 10 collection + vector RPCs shipped (PR #38); full status mapping + parity test (PR #39) |
| HNSW index, TUI | Phase 1 |
| Bundled embedding model, `neko embed` | Phase 2 |
| Clustering, replication | Phase 3 |

## OpenAPI / Swagger

The REST surface is documented via [swaggo](https://github.com/swaggo/swag). When `neko serve` is running:

- **Swagger UI** — http://localhost:3434/swagger/index.html
- **Raw OpenAPI 2.0 spec (JSON)** — http://localhost:3434/swagger/doc.json
- **YAML spec** — http://localhost:3434/swagger/doc.json → `?format=yaml` (via the UI's "Download" button, or read `internal/api/docs/swagger.yaml` from the source tree)

The spec is generated from `// @`-comment annotations on each handler in `internal/api/handler.go`. The full request/response schemas are derived automatically from the Go DTO structs.

### Regenerating the spec

The spec is regenerated on every `make build` (the `swag` target runs first). To regenerate manually:

```bash
# One-time: install the swag CLI
go install github.com/swaggo/swag/cmd/swag@v1.16.3

# Regenerate internal/api/docs/ from handler.go annotations
make swag
# or:
swag init -g internal/api/handler.go -o internal/api/docs --parseDependency --parseInternal
```

### Adding a new endpoint

1. Register the route in `internal/api/router.go`.
2. Add a swag annotation block above the handler in `internal/api/handler.go` — see existing handlers for the shape. Use the **full path** in `@Router` (e.g. `/v1/collections/{name}/vectors/{id}`), not a basePath-relative one.
3. Add the `(method, path)` pair to the `expectedRoutes` list in `internal/api/docs_test.go`.
4. Run `make swag` then `go test ./...`. The drift test in `internal/api/docs_test.go` fails the build if the spec and the router disagree.

A working annotation block looks like:

```go
// HandleCreateCollection godoc
// @Summary      Create a collection
// @Description  Creates a new named collection with a fixed dimension and distance metric.
// @Tags         collections
// @Accept       json
// @Produce      json
// @Param        body body CreateCollectionRequestHttpDTO true "Collection config"
// @Success      201 {object} CreateCollectionResponseHttpDTO
// @Failure      400 {object} APIError "HAIRBALL_INVALID_NAME / HAIRBALL_INVALID_METRIC"
// @Failure      409 {object} APIError "HAIRBALL_ALREADY_EXISTS"
// @Router       /v1/collections [post]
func (s *Server) HandleCreateCollection(rw http.ResponseWriter, r *http.Request) {
    // ...
}
```

## gRPC

PR #37 ships the gRPC scaffold; PR #38 ships the 10 collection + vector RPCs. When `neko serve` is running on the default port, both REST (HTTP/1.1) and gRPC (HTTP/2) accept connections on the same socket — cmux demuxes by protocol.

### Browsing the gRPC API

When `neko serve` is running:

- **gRPC reference docs** — http://localhost:3434/grpc-doc/ (single-file HTML generated by `protoc-gen-doc` from `proto/neko.proto`, embedded into the binary)
- **Live introspection** — `grpcurl -plaintext localhost:3434 list` (gRPC reflection is on by default)

The HTML is regenerated on every `make build` (the `proto` target now also runs `protoc-gen-doc`) and committed to the repo under `internal/grpc/docs/index.html`.

- **Proto contract** — [`proto/neko.proto`](proto/neko.proto). Request and response messages mirror the REST `*HttpDTO` shapes field-for-field. Two services: `CollectionService` (Create / List / Get / Drop / Search) and `VectorService` (Insert / InsertMany / Get / Upsert / Delete).
- **Generated stubs** — `internal/gen/neko/v1/*.pb.go` (gitignored; regenerated by `make proto`).
- **Reflection** — `grpcurl` works out of the box:
  ```bash
  grpcurl -plaintext localhost:3434 list            # list services
  grpcurl -plaintext localhost:3434 neko.v1.CollectionService/Create  # live since PR #38
  ```

### Regenerating the stubs

```bash
# One-time: install protoc + the Go gen plugins
brew install protobuf          # macOS
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Regenerate internal/gen/neko/v1/ from proto/neko.proto
make proto
# or:
./proto/gen.sh
```

`make build` runs `make swag` then `make proto` before the Go binary is compiled, so the stubs are always in sync with the proto on a clean build.

### Adding a new RPC

1. Add the request/response messages and the RPC declaration to `proto/neko.proto`. **Mirror the REST `*HttpDTO` in `internal/api/handler.go` field-for-field** — proto message name = REST DTO name with `HttpDTO` stripped, snake_cased. Path params become message fields. Bare-array bodies become wrapped messages. No new fields without a matching REST DTO.
2. Run `make proto` to regenerate stubs.
3. Implement the handler in `internal/grpc/<service>.go` — a thin adapter that calls into `internal/ffi/` and maps `HAIRBALL_*` codes to gRPC status.
4. Register the service on the `*grpc.Server` in `internal/server/core.go`.

## Documentation

- [Background: Vector Databases & LLMs](docs/BACKGROUND.md)
- [Product Requirements](docs/PRD.md)
- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Implementation Phases](docs/phases/)

## License

MIT
