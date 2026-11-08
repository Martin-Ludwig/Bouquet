module bouquet

go 1.27

require github.com/gen2brain/raylib-go/raylib v0.0.0-00010101000000-000000000000

require (
	github.com/BrownNPC/Raylib-Go-Wasm/wasm-runtime v0.0.0-20261101111811-ebb3be519e53 // indirect
	github.com/BrownNPC/wasm-ffi-go v1.3.0 // indirect
)

// Redirects the raylib-go import to a wasm-only fork: the project builds for
// GOOS=js GOARCH=wasm only while this replace is in effect.
replace github.com/gen2brain/raylib-go/raylib => github.com/BrownNPC/Raylib-Go-Wasm/raylib v0.0.0-20261101111811-ebb3be519e53
