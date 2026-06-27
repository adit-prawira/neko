# neko (猫)

**The local-first vector database that purrs.**

Neko is an open-source, single-binary vector database designed to run on your machine — no cloud, no API keys, no cost. Think SQLite for vectors.

## Why Neko?

Every AI app needs to search vectors. Existing options either lock you into a cloud service ($$$) or require a cluster of Docker containers just to prototype. Neko runs as a single static binary. `brew install neko` and you're done.

- **Zero cost** — local embedding model bundled. No OpenAI bill.
- **Single binary** — Go + Rust + C, fused. No Docker, no deps.
- **Familiar API** — REST + gRPC + CLI. Curl-friendly.
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

# Interactive TUI (Phase 1+)
neko tui
```

## Documentation

- [Background: Vector Databases & LLMs](docs/BACKGROUND.md)
- [Product Requirements](docs/PRD.md)
- [Architecture](docs/ARCHITECTURE.md)
- [API Reference](docs/API.md)
- [Implementation Phases](docs/phases/)

## License

MIT
