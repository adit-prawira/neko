.PHONY: build test clean

build:
	cd simd && make
	cd engine && cargo build --release
	CGO_LDFLAGS="-L$$PWD/engine/target/release -lneko_engine" \
	go build -o neko ./cmd/neko

test:
	cd simd && make test
	cd engine && cargo test
	go test ./...

clean:
	cd simd && make clean
	cd engine && cargo clean
	rm -f neko
