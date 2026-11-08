package main

import rl "github.com/gen2brain/raylib-go/raylib"

func main() {
	rl.SetConfigFlags(rl.FlagVsyncHint | rl.FlagWindowResizable)
	rl.InitWindow(800, 450, "Bouquet - hello world")
	rl.SetTargetFPS(60)

	pos := rl.NewVector2(400, 300)
	const speed = 300.0
	const radius = 20.0

	const msg = "Hello, World!"
	tw := rl.MeasureText(msg, 40) // constant text: measure once, not every frame

	update := func() {
		dt := rl.GetFrameTime()
		if rl.IsKeyDown(rl.KeyRight) || rl.IsKeyDown(rl.KeyD) {
			pos.X += speed * dt
		}
		if rl.IsKeyDown(rl.KeyLeft) || rl.IsKeyDown(rl.KeyA) {
			pos.X -= speed * dt
		}
		if rl.IsKeyDown(rl.KeyDown) || rl.IsKeyDown(rl.KeyS) {
			pos.Y += speed * dt
		}
		if rl.IsKeyDown(rl.KeyUp) || rl.IsKeyDown(rl.KeyW) {
			pos.Y -= speed * dt
		}

		w, h := float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
		if w > 0 && h > 0 {
			pos.X = min(max(pos.X, radius), w-radius)
			pos.Y = min(max(pos.Y, radius), h-radius)
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		rl.DrawText(msg, int32(w)/2-tw/2, 60, 40, rl.DarkGray)
		rl.DrawText("Move the ball with arrow keys or WASD", 20, int32(h)-30, 20, rl.Gray)
		rl.DrawCircleV(pos, radius, rl.Maroon)

		rl.EndDrawing()
	}

	// The JS runtime drives update() each animation frame; this never returns.
	// (The binding is wasm-only — see the replace directive in go.mod.)
	rl.SetMain(update)
}
