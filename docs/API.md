# Neko — API Reference

Base URL: `http://localhost:3434/v1`

Content-Type: `application/json`

> **Live OpenAPI spec** — A machine-readable spec for every endpoint in this document is generated from the `// @`-comment annotations on each handler in `internal/api/handler.go` via [swaggo](https://github.com/swaggo/swag). When `neko serve` is running, it's available at:
> - **JSON spec**: `http://localhost:3434/swagger/doc.json`
> - **YAML spec**: `internal/api/docs/swagger.yaml` in the source tree
> - **Interactive UI**: `http://localhost:3434/swagger/index.html`
>
> The spec is regenerated on every `make build`. Drift between the routes and the spec is caught by `internal/api/docs_test.go` at test time — adding a new route without an annotation (or vice versa) fails the build.

## Collections

### Create Collection

```
POST /v1/collections
```

```json
{
  "name": "docs",
  "dim": 384,
  "model": "all-MiniLM-L6-v2",
  "metric": "cosine"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | yes | — | Collection name. Alphanumeric + hyphens/underscores, 1-64 chars. |
| `dim` | u32 | no* | — | Vector dimension. Optional if `model` is set. Must be ≤ 4096. |
| `model` | string | no | `"all-MiniLM-L6-v2"` | Embedding model name. `dim` is inferred from the model. |
| `metric` | string | no | `"cosine"` | Distance metric: `"cosine"`, `"dot"`, `"l2"`. |

Response `201`:
```json
{
  "name": "docs",
  "dim": 384,
  "metric": "cosine",
  "model": "all-MiniLM-L6-v2",
  "vector_count": 0
}
```

---

### List Collections

```
GET /v1/collections
```

Response `200`:
```json
{
  "collections": [
    {
      "name": "docs",
      "dim": 384,
      "metric": "cosine",
      "model": "all-MiniLM-L6-v2",
      "vector_count": 1234
    }
  ]
}
```

---

### Get Collection Info

```
GET /v1/collections/:name
```

Response `200`:
```json
{
  "name": "docs",
  "dim": 384,
  "metric": "cosine",
  "model": "all-MiniLM-L6-v2",
  "vector_count": 1234,
  "storage_bytes": 1572864,
  "index": "brute"
}
```

---

### Drop Collection

```
DELETE /v1/collections/:name
```

Response `204` (no body).

---

## Vectors

### Insert Vector

```
POST /v1/collections/:name/vectors
```

```json
{
  "id": "doc_42",
  "vector": [0.12, -0.34, 0.78, "..."]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique vector ID. 1-256 chars. |
| `vector` | [f32] | yes | Floating-point vector. Must match collection dim. |
| `metadata` | object | no | Arbitrary key-value pairs for filtering. |

Response `201`:
```json
{
  "id": "doc_42",
  "dim": 384
}
```

---

### Get Vector

```
GET /v1/collections/:name/vectors/:id
```

Response `200`:
```json
{
  "id": "doc_42",
  "vector": [0.12, -0.34, 0.78, "..."],
  "metadata": "{\"author\":\"alice\"}"
}
```

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Echoes the vector ID from the path. |
| `vector` | [f32] | The stored vector. For cosine collections, this is the unit-normalized version of what was inserted. |
| `metadata` | string | The raw JSON string that was sent at insert time. Absent (not `null`) if no metadata was attached. |

Response `404` (`HAIRBALL_NOT_FOUND`) if the collection or vector id does not exist.

---

### Batch Insert Vectors

```
POST /v1/collections/:name/vectors/batch
```

Insert up to 10,000 vectors in one call. The body is a bare JSON array
of vector objects (same shape as the single-insert body). The whole batch
is dispatched through a single FFI call (`neko_insert_many`).

```json
[
  { "id": "doc_1", "vector": [0.12, -0.34, 0.78, "..."] },
  { "id": "doc_2", "vector": [0.56, 0.78, 0.90, "..."], "metadata": "{\"author\":\"alice\"}" }
]
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Unique vector ID. 1-256 chars. |
| `vector` | [f32] | yes | Floating-point vector. Must match the collection's dim. |
| `metadata` | string | no | Raw JSON string echoed back by GET. |

Response `201`:
```json
{
  "inserted": 2,
  "ids": ["doc_1", "doc_2"],
  "dim": 384
}
```

| Status | Code | When |
|--------|------|------|
| `201` | — | All vectors inserted. |
| `400` | `HAIRBALL_INVALID_NAME` | Body is not valid JSON, the array is empty, an item is missing `id`, or the batch exceeds 10,000 vectors. |
| `400` | `HAIRBALL_DIM_MISMATCH` | Any item's `vector` length does not match the collection's dim. |
| `404` | `HAIRBALL_NOT_FOUND` | Collection does not exist. |
| `500` | `HAIRBALL_INTERNAL` | FFI failure (e.g., null pointer from a malformed request). |

All-or-nothing on dim and id validation: the first invalid item returns
the error and no vectors are written. Within a valid batch, existing IDs
are silently overwritten (use `PUT /vectors/:id` for explicit upsert).

---

### Upsert Vector

```
PUT /v1/collections/:name/vectors/:id
```

Insert or update by ID. The `:id` is the vector ID from the path; the body contains only the vector and optional metadata.

```json
{
  "vector": [0.12, -0.34, 0.78, "..."],
  "metadata": "{\"author\":\"alice\"}"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `vector` | [f32] | yes | Floating-point vector. Must match collection dim. |
| `metadata` | string | no | Raw JSON string echoed back by GET. |

Response `201` (created) or `200` (updated):
```json
{ "id": "doc1", "dim": 384 }
```

| Status | Code | When |
|--------|------|------|
| `200` | — | Vector existed and was replaced. |
| `201` | — | Vector did not exist; new vector stored. |
| `400` | `HAIRBALL_INVALID_NAME` | Body is not valid JSON. |
| `400` | `HAIRBALL_DIM_TOO_SMALL` | Vector is empty. |
| `400` | `HAIRBALL_DIM_MISMATCH` | Vector length does not match the collection's dim. |
| `404` | `HAIRBALL_NOT_FOUND` | Collection does not exist. |

---

### Search Vectors

```
POST /v1/collections/:name/search
```

```json
{
  "vector": [0.12, -0.34, 0.78, "..."],
  "top_k": 10,
  "filter": "category = 'docs'"
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `vector` | [f32] | — | Raw query vector. Must match collection dim. |
| `top_k` | u32 | 10 | Number of results to return. |
| `filter` | string | — | Metadata filter expression (see below). |

Response `200`:
```json
{
  "results": [
    {
      "id": "doc_42",
      "score": 0.9412,
      "metadata": { "title": "Refund Policy" }
    },
    {
      "id": "doc_17",
      "score": 0.8734,
      "metadata": { "title": "Shipping FAQ" }
    }
  ],
  "latency_us": 420
}
```

- `score`: [0..1] for cosine/dot (higher = closer), unbounded positive for L2 (lower = closer).
- `latency_us`: Search time in microseconds.

---

### Delete Vector

```
DELETE /v1/collections/:name/vectors/:id
```

Response `204` (no body) on success. Response `404` (`HAIRBALL_NOT_FOUND`) if the collection or vector id does not exist.

---

## Models

*Planned — Phase 2 (embeddings). Listed here so the full API surface is visible; none of these endpoints exist in v0.1.*

### List Models

```
GET /v1/models
```

Response `200`:
```json
{
  "models": [
    {
      "name": "all-MiniLM-L6-v2",
      "dim": 384,
      "size_bytes": 94371840,
      "loaded": true,
      "bundled": true,
      "language": "en"
    },
    {
      "name": "bge-small-en-v1.5",
      "dim": 384,
      "size_bytes": 136314880,
      "loaded": true,
      "bundled": false,
      "language": "en"
    }
  ]
}
```

---

### Pull Model

```
POST /v1/models/pull
```

```json
{
  "name": "bge-small-en-v1.5"
}
```

Downloads the model ONNX file from HuggingFace Hub, validates integrity,
registers in `models.toml`. May take 10-30 seconds depending on network.

Response `200`:
```json
{
  "name": "bge-small-en-v1.5",
  "dim": 384,
  "size_bytes": 136314880,
  "loaded": true
}
```

Response `404` if model name is not recognized.

---

## Health

### Health Check

```
GET /health
```

Response `200`:
```json
{
  "status": "ok",
  "uptime_s": 3600
}
```

---

## Metadata Filter Syntax

Neko uses a simple expression language for metadata filters. All metadata
values are automatically typed from their JSON representation.

### Operators

| Operator | Description | Example |
|----------|-------------|---------|
| `=` | Equality | `category = 'docs'` |
| `!=` | Not equal | `status != 'archived'` |
| `>` | Greater than | `priority > 5` |
| `<` | Less than | `score < 0.5` |
| `>=` | Greater or equal | `count >= 10` |
| `<=` | Less or equal | `age <= 30` |
| `IN` | Value in set | `status IN ('open', 'pending')` |
| `BETWEEN` | Range (inclusive) | `age BETWEEN 18 AND 65` |

### Logical Combinators (in precedence order)

| Combinator | Description | Example |
|------------|-------------|---------|
| `NOT` | Unary negation | `NOT deleted = true` |
| `AND` | Conjunction | `category = 'docs' AND priority > 5` |
| `OR` | Disjunction | `status = 'open' OR status = 'pending'` |

### Nesting

Use parentheses `()` to group expressions:

```
(category = 'docs' AND priority > 5) OR category = 'wiki'
```

### Value Types

| Type | Format | Example |
|------|--------|---------|
| String | Single-quoted | `'hello world'` |
| Number (int) | Bare integer | `42`, `-7` |
| Number (float) | Bare decimal | `3.14`, `-0.5` |
| Boolean | Literal | `true`, `false` |
| Null | Literal | `null` |

### Filter Execution

Filters are applied as a pre-filter step during search: Neko scans vectors
and skips any whose metadata does not match the filter expression. The top-k
results are selected from the filtered set.

If no `filter` is provided, all vectors are scanned.

---

## Errors

All errors follow the same format:

```json
{
  "error": {
    "code": "HAIRBALL_NOT_FOUND",
    "message": "clowder 'docs' does not exist"
  }
}
```

| HTTP Status | Typical Codes |
|-------------|---------------|
| `400` | `HAIRBALL_INVALID_NAME`, `HAIRBALL_DIM_MISMATCH`, `HAIRBALL_DIM_TOO_LARGE`, `HAIRBALL_INVALID_METRIC`, `HAIRBALL_INVALID_MODEL` |
| `404` | `HAIRBALL_NOT_FOUND` (collection or vector), `HAIRBALL_MODEL_NOT_FOUND` (model name unknown) |
| `409` | `HAIRBALL_ALREADY_EXISTS` (collection, vector ID) |
| `500` | `HAIRBALL_INTERNAL` |

Additional model-specific codes:
| `HAIRBALL_MODEL_NOT_LOADED` | The model is registered but not loaded into memory |
| `HAIRBALL_MODEL_DOWNLOAD_FAILED` | Model download from HuggingFace failed |
| `HAIRBALL_MODEL_DIM_MISMATCH` | The model's output dimension doesn't match the collection |

---

## gRPC

*Planned — not yet implemented. The service definition below shows the intended shape; no `proto/neko.proto` or `internal/api/grpc.go` exists in v0.1.*

The gRPC API mirrors the REST API exactly. Service definition:

```protobuf
service Neko {
  rpc CreateCollection(CreateCollectionRequest) returns (Collection);
  rpc DropCollection(DropCollectionRequest) returns (DropCollectionResponse);
  rpc ListCollections(ListCollectionsRequest) returns (ListCollectionsResponse);
  rpc GetCollection(GetCollectionRequest) returns (Collection);

  rpc InsertVector(InsertVectorRequest) returns (InsertVectorResponse);
  rpc BatchInsertVectors(BatchInsertRequest) returns (BatchInsertResponse);
  rpc UpsertVector(UpsertVectorRequest) returns (UpsertVectorResponse);
  rpc SearchVectors(SearchRequest) returns (SearchResponse);
  rpc DeleteVector(DeleteVectorRequest) returns (DeleteVectorResponse);

  rpc HealthCheck(HealthCheckRequest) returns (HealthCheckResponse);

  rpc ListModels(ListModelsRequest) returns (ListModelsResponse);
  rpc PullModel(PullModelRequest) returns (PullModelResponse);
}
```

---

## Notes

- Vectors are auto-normalized on insert when the collection metric is `cosine`.
- The default data directory is `~/.neko/`. Override with `NEKO_HOME` env var or `--data-dir`.
- CORS headers are included on all responses (supports browser-based clients).
- Search results are sorted by score descending for cosine/dot, ascending for L2.
- Collections are bound to a model at creation time. Text search uses the collection's model. (Planned for Phase 2 — the `model` field on collection create is accepted today but not yet enforced.)
