# Development container for Bouquet (Go + raylib + WebAssembly).
# See README.md for the full workflow.
#
# Build:
#   podman build -t bouquet-dev -f Containerfile .
#
# Run (project mounted at /workspace, web server on http://localhost:8080):
#   podman run -it --rm -v .:/workspace:Z -p 8080:8080 bouquet-dev
#
# Inside the container:
#   build-wasm               # compiles main package to web/main.wasm
#   go run ./tools/serve     # serves the game on :8080
#   air                      # optional: auto-rebuild on save (uses .air.toml)

FROM docker.io/library/golang:1.27-bookworm

# The WASM build is pure Go (GOOS=js GOARCH=wasm): no C toolchain or X11
# headers needed. The go.mod replace points raylib at a wasm-only fork, so a
# native desktop build would not compile anyway.

# Developer tooling: hot-reload, language server, debugger.
RUN go install github.com/air-verse/air@latest \
    && go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest

# Helper: compile the main package to WASM into web/.
RUN printf '%s\n' \
        '#!/bin/sh -e' \
        'GOOS=js GOARCH=wasm go build -o ./web/main.wasm .' \
        'echo "Built web/main.wasm"' \
        > /usr/local/bin/build-wasm \
    && chmod +x /usr/local/bin/build-wasm

# Module and build caches live at /go/pkg/mod and /root/.cache/go-build (the
# image defaults). Persist them in named volumes for fast rebuilds:
#   podman run ... -v go-mod:/go/pkg/mod -v go-cache:/root/.cache/go-build ...

# Debian's /etc/profile resets PATH for login shells (which is what editor
# terminals such as Zed's open), dropping the Go dirs set by the base image's
# ENV PATH. Re-add them; /etc/profile sources profile.d after its reset.
RUN printf '%s\n' 'export PATH="/go/bin:/usr/local/go/bin:$PATH"' > /etc/profile.d/go.sh

WORKDIR /workspace

# tools/serve listens here.
EXPOSE 8080

CMD ["bash"]
