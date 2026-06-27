# Background: Vector Databases & LLMs

## What Is a Vector?

A **vector** is a list of floating-point numbers that represents the "meaning" of
something — a word, a sentence, an image, a song. This representation is called
an **embedding**.

```
"The cat sat on the mat" → embedding model → [0.12, -0.34, 0.78, ..., 0.05]
                                                   ↑
                                           384 numbers (dimensions)
```

The magic: similar concepts end up **close together** in this number space. Two
sentences about cats will have vectors near each other. A sentence about
rockets will be far away.

```
cat    → [0.8, 0.6, 0.1]
kitten → [0.7, 0.5, 0.2]   ← close (both about cats)
rocket → [0.1, 0.2, 0.9]   ← far (different topic)
```

## Why LLMs Need Vector Databases

Large Language Models (like GPT, Claude, Llama) have a fundamental limitation:
they **don't know your data**. They were trained on public internet data up to a
certain date. They can't answer questions about:

- Your company's internal docs
- Your personal notes
- Recent events after their training cutoff
- Private codebases

### Enter RAG (Retrieval-Augmented Generation)

```
┌──────────────────────────────────────────────────────┐
│                     RAG Pipeline                       │
│                                                       │
│  1. INGEST: Your docs → embed → store in vector DB    │
│                                                       │
│  2. QUERY: User asks a question                       │
│     ↓                                                 │
│  3. RETRIEVE: Find top-5 most relevant docs in DB     │
│     ↓                                                 │
│  4. AUGMENT: "Here are relevant docs + user question" │
│     ↓                                                 │
│  5. GENERATE: LLM answers using your docs as context  │
│                                                       │
└──────────────────────────────────────────────────────┘
```

**Real example:**
- User: "What's our refund policy?"
- Vector DB finds: `refund-policy.txt`, `customer-support.md`
- LLM prompt: "Using these docs: [refund policy text]. Answer: what's our refund policy?"
- LLM response: "Refunds accepted within 30 days with receipt..."

Without the vector DB, the LLM would either hallucinate or say "I don't know."

**The vector database is the LLM's memory.**

## How Vector Search Works

### Step 1: Similarity (distance)

To find "closest" vectors, we measure distance:

| Metric | Formula | Use Case |
|--------|---------|----------|
| **Cosine similarity** | cos(θ) = A·B / (\|A\|·\|B\|) | Measures direction, ignores magnitude. Most common for text. |
| **Dot product** | A·B = Σ(Aᵢ·Bᵢ) | Faster, works when vectors are normalized. |
| **Euclidean (L2)** | √Σ(Aᵢ−Bᵢ)² | Sensitive to magnitude. Used for images. |

### Step 2: The Hard Part — Finding Nearest Neighbors

**Brute force:** Compare the query vector against _every_ vector in the database.
- 1M vectors × 384 dimensions × 4 bytes = ~1.5 GB of computation
- Fast with SIMD, but O(N) — doesn't scale

**ANN (Approximate Nearest Neighbors):** Trade a tiny bit of accuracy for massive speed.

### HNSW (Hierarchical Navigable Small World)

The most popular ANN algorithm. Think of it as a **multi-level skip list for vectors**:

```
Layer 2:  ●───────●           ← sparse, long jumps (highway)
           │       │
Layer 1:  ●──●──●──●──●       ← medium density (local roads)
           │  │  │  │  │
Layer 0:  ●─●●─●●─●●─●●─●     ← dense, all vectors (streets)
```

Search: start at the top layer, greedily move toward the query, drop down a
layer, repeat. Finds top-k in **~1 ms instead of ~50 ms** for 1M vectors.

## The Landscape

| Tool | Type | Pros | Cons |
|------|------|------|------|
| **Pinecone** | Cloud | Managed, zero-ops | $70+/mo, internet required, vendor lock-in |
| **Qdrant** | Self-hosted | Fast, Rust, gRPC API | Requires Docker, heavier setup |
| **Milvus** | Self-hosted | Billion-scale, many index types | Complex, needs etcd + MinIO + Pulsar |
| **FAISS** | Library | Meta-backed, extremely fast | No persistence, no server, C++/Python only |
| **Chroma** | Embedded | Python-native, simple | Python-only, embedding API required |
| **LanceDB** | Embedded | Rust-based, columnar storage | Python/JS bindings, no standalone binary |
| **SQLite-VSS** | Extension | SQLite extension | SQLite-level scale, experimental |
| **Neko** | Embedded | Local-first, single binary, TUI | New project (you're building it!) |

## Key Terms

| Term | Meaning |
|------|---------|
| **Embedding** | A vector representation of data (text, image, audio) |
| **Dimension (dim)** | Number of floats in a vector. 384, 768, 1536 are common. |
| **Top-k** | How many nearest neighbors to return. Usually 5-100. |
| **Recall** | % of true nearest neighbors found by ANN. HNSW typically 95-99%. |
| **Latency** | Time from query to result. Target: < 10 ms for 1M vectors. |
| **Quantization** | Compressing vectors (e.g., 384×f32 → 384×i8) to save memory. Loses ~1% recall. |
| **ef_search** | HNSW parameter controlling accuracy vs speed tradeoff. Higher = more accurate but slower. |
| **WAL** | Write-Ahead Log. Ensures data isn't lost on crash. |

## A Concrete Example

```bash
# 1. Start Neko
neko serve

# 2. Ingest your docs
neko create knowledge --dim 384
neko insert knowledge --id doc1 --text "Refunds are accepted within 30 days"
neko insert knowledge --id doc2 --text "Shipping takes 3-5 business days"
neko insert knowledge --id doc3 --text "Our office is in San Francisco"

# 3. Search
neko search knowledge "Can I get my money back?"
# → doc1: 0.94 "Refunds are accepted within 30 days"
# → doc2: 0.31 "Shipping takes 3-5 business days"
# → doc3: 0.12 "Our office is in San Francisco"

# 4. Use with any LLM
# Take doc1's content, pass to Claude/GPT/Llama:
# "Answer using this context: {doc1}. Question: Can I get my money back?"
```

## Further Reading

- [Pinecone: What is a Vector Database?](https://www.pinecone.io/learn/vector-database/)
- [HNSW Paper (Malkov & Yashunin, 2016)](https://arxiv.org/abs/1603.09320)
- [Weaviate: Vector Search Explained](https://weaviate.io/blog/vector-library)
- [FAISS: A Library for Efficient Similarity Search](https://engineering.fb.com/2017/03/29/data-infrastructure/faiss-a-library-for-efficient-similarity-search/)
