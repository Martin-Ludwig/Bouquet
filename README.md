# Bouquet

A Go + raylib game compiled to WebAssembly, built on
[Raylib-Go-Wasm](https://github.com/BrownNPC/Raylib-Go-Wasm). The `replace`
in `go.mod` redirects the `raylib-go` import to that wasm-only fork, so the
project builds for `GOOS=js GOARCH=wasm` only (no native desktop build).

## Build and run

```sh
GOOS=js GOARCH=wasm go build -o web/main.wasm .
go run ./tools/serve        # serves web/ on http://localhost:8080
```

`air` rebuilds `web/main.wasm` on every save (config in `.air.toml`); keep
`go run ./tools/serve` running next to it and refresh the browser.

## Container

```sh
podman build -t bouquet-dev -f Containerfile .
podman run -it --rm -v .:/workspace:Z -p 8080:8080 bouquet-dev
```

Inside the container, `build-wasm` compiles the game and
`go run ./tools/serve` serves it. The `.devcontainer/` config uses the same
Containerfile.

## Layout

- `main.go` — the game
- `web/` — static page, JS glue between Go and raylib, and the built `main.wasm`
- `web/rl/` — prebuilt raylib wasm + Emscripten loader (vendored, see `web/rl/README.md`)
- `web/wasm_exec.js` — Go's wasm runtime glue; must match the Go toolchain
  (`cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/` after a Go upgrade)
- `tools/serve` — minimal dev file server
