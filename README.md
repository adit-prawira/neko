# neko (猫)

**The local-first vector database that purrs.**

Neko is an open-source, single-binary vector database designed to run on your machine — no cloud, no API keys, no cost. Think SQLite for vectors.

## Why Neko?

Every AI app needs to search vectors. Existing options either lock you into a cloud service ($$$) or require a cluster of Docker containers just to prototype. Neko runs as a single static binary. `brew install neko` and you're done.

- **Zero cost** — no cloud account, no API keys, no billing. Storage is your own disk.
- **Single binary** — Go + Rust + C, fused. No Docker, no deps.
- **Familiar API** — REST + CLI today, gRPC coming in Phase 7. Curl-friendly.
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

# Or use the REST API
curl -X POST localhost:3434/v1/collections/docs/search \
  -H 'Content-Type: application/json' \
  -d '{"vector": [0.12, -0.34, 0.78], "top_k": 10}'

# Browse the API in Swagger UI
open http://localhost:3434/swagger/index.html
```

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
| gRPC API | Phase 7 |
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

## Documentation

- [Background: Vector Databases & LLMs](docs/BACKGROUND.md)
- [Product Requirements](docs/PRD.md)
- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Implementation Phases](docs/phases/)

## License

MIT
