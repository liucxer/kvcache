# AGENTS.md

## Build & Test

- **All `go` and `make` commands require `CGO_CFLAGS` / `CGO_LDFLAGS` for RocksDB.**
- RocksDB installed at `/usr/local/` (version 8.11.5), gorocksdb v1.8.15 (compatible with 8.x).
- On this Linux host, use these CGO flags:
  ```
  CGO_CFLAGS="-I/usr/local/include" \
  CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy -llz4 -lzstd -Wl,-rpath,/usr/local/lib"
  ```
- The Makefile hardcodes macOS Homebrew paths (`/opt/homebrew/...`) — override them with the flags above.
- Build: `make build` (with CGO flags above) **or** directly: `go build -o kvcache`
- Full project builds and links successfully. All tests pass.
- Test all: `make test` — runs unit tests in `config/`, `service/`, `storage/` and integration tests in `test/api/`
- Test client only: `CGO_ENABLED=0 go test ./client/...`
- Test config only: `CGO_ENABLED=0 go test ./config/...`
- No linter or typecheck targets exist; run `go vet ./...` manually.

## Proto Codegen

- `proto/kv.proto` is the source; `proto/kv.pb.go` and `proto/kv_grpc.pb.go` are generated.
- Regenerate with `protoc --go_out=. --go-grpc_out=. proto/kv.proto`.

## Architecture

- **main.go** auto-discovers port pairs in range **33000–33100** (even=gRPC, odd=HTTP). Do not assume fixed ports 50051/8080.
- Layer order: `storage/` (RocksDB) → `service/` (business logic + metrics) → `api/` (gRPC + HTTP servers)
- `client/` is the SDK — see `docs/sdk-design.md` for full design.
- Multi-instance management scripts: `start-instances.sh`, `stop-instances.sh`, `status-instances.sh`
- Design docs in `docs/`: `docs/sdk-design.md` (SDK), `docs/kvcache-design.md` (server)

## Distributed Mode

- Each kvcache instance now requires `--name`, `--node`, `--tikv-pd` flags on startup.
- Instances register themselves to TiKV at startup and send heartbeats every 1s with capacity data.
- The SDK (`client/`) discovers instances from TiKV and uses capacity-aware local-affinity routing.
- The key→instance mapping is stored in TiKV under `/kvcache/index/` with an LRU in-memory cache (default 1GB).
- **Integration tests that hit TiKV require a running TiKV cluster**; unit tests (route cache, picker) run offline.

## Gotchas

- Tests create `data/` and `value_data/` directories (gitignored). Clean up with `make clean` or manually.
- `config/` holds a `DefaultConfig()` in Go, not a config file — runtime config is in-code.
- Go module is `kvcache` (not a URL path); imports use `kvcache/...`.
- `go get` with `CGO_ENABLED=0` is required for adding Go-only dependencies (TiKV client).
