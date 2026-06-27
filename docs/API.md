# Neko — API Reference

Base URL: `http://localhost:3434/v1`

Content-Type: `application/json`

## Collections

### Create Collection

```
POST /collections
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
GET /collections
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
GET /collections/:name
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
DELETE /collections/:name
```

Response `200`:
```json
{
  "deleted": "docs"
}
```

---

## Vectors

### Insert Vector

```
POST /collections/:name/vectors
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
  "id": "doc_42"
}
```

---

### Batch Insert Vectors

```
POST /collections/:name/vectors/batch
```

```json
{
  "vectors": [
    { "id": "doc_1", "vector": [0.12, -0.34, "..."] },
    { "id": "doc_2", "vector": [0.56, 0.78, "..."] }
  ]
}
```

Response `201`:
```json
{
  "inserted": 2
}
```

---

### Upsert Vector

```
PUT /collections/:name/vectors/:id
```

Insert or update by ID. Same body as insert.

Response `200` (updated) or `201` (created).

---

### Search Vectors

```
POST /collections/:name/search
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
DELETE /collections/:name/vectors/:id
```

Response `200`:
```json
{
  "deleted": "doc_42"
}
```

---

## Models

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
- Collections are bound to a model at creation time. Text search uses the collection's model.
- The default model (`all-MiniLM-L6-v2`) is bundled. Other models are downloaded on demand.
