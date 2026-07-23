.PHONY: build test clean format

build:
	cd simd && make
	cd engine && cargo build --release
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go build -o neko ./cmd/neko

test:
	cd simd && make test
	cd engine && cargo test
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go test ./...

clean:
	cd simd && make clean
	cd engine && cargo clean
	rm -f neko

format:
	cd engine && cargo fmt
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine -Wl,-rpath,$$PWD/engine/target/release" \
	go fmt ./...
