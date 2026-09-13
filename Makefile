.PHONY: build test clean format swag proto

build: swag proto
	cd simd && make
	cd engine && cargo build --release
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go build -o neko ./cmd/neko

# Regenerate the OpenAPI spec into internal/api/docs/.
# Requires the `swag` CLI: `go install github.com/swaggo/swag/cmd/swag@v1.16.3`.
# The generated `docs.go` embeds swagger.json + swagger.yaml into the binary.
swag:
	swag init -g internal/api/handler.go -o internal/api/docs --parseDependency --parseInternal

# Regenerate Go gRPC stubs from proto/neko.proto into internal/gen/.
# Requires `protoc` on PATH and `protoc-gen-go` + `protoc-gen-go-grpc` in $GOBIN.
proto:
	./proto/gen.sh

test:
	cd simd && make test
	cd engine && cargo test
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go test ./...

clean:
	cd simd && make clean
	cd engine && cargo clean
	rm -f neko
	rm -rf internal/api/docs
	rm -rf internal/gen/neko

format:
	cd engine && cargo fmt
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go fmt ./...
	swag fmt
	clang-format -i proto/*.proto
