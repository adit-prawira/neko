#!/usr/bin/env bash
set -euo pipefail
protoc \
  --proto_path=proto \
  --go_out=. \
  --go_opt=module=github.com/adit-prawira/neko \
  --go-grpc_out=. \
  --go-grpc_opt=module=github.com/adit-prawira/neko \
  --doc_out=internal/grpc/docs \
  --doc_opt=html,index.html \
  proto/neko.proto
