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

The game starts in a menu: campaign (world map), "Überleben" (one long
mission), duel against the AI, duel for two. In a mission, Space waits a
turn, R starts a new one and Esc goes back; Esc also leaves the world map
and duels.

## Container

```sh
podman build -t bouquet-dev -f Containerfile .
podman run -it --rm -v .:/workspace:Z -p 8080:8080 bouquet-dev
```

Inside the container, `build-wasm` compiles the game and
`go run ./tools/serve` serves it. The `.devcontainer/` config uses the same
Containerfile.

## Layout

- `main.go` — window, frame loop and main menu; wires the packages together
- `duel_game.go` — the duel screen: class choice, hand-over between two players, AI pacing
- `content/` — the shipped mods as data (`base/` is the base game), embedded at build time
- `mods/` — loads and merges mod folders into `game.Content`
- `game/` — the rules of missions and the world map, pure Go without raylib, so `go test ./game` runs natively
- `duel/` — the rules and AI of the duel, pure Go as well (`go test ./duel`)
- `view/` — draws missions, duels, the world map, buttons and menus with raylib; colours,
  hatching and the background image come from the content
- `save_js.go` — stores the save game in the browser's `localStorage` (key `bouquet-save`)
- `lib/hex`, `lib/grid` — hex coordinates, path distance and line of sight (pure Go)
- `lib/draw` — hex tiles, tokens and arrows drawn with raylib
- `web/` — static page, JS glue between Go and raylib, and the built `main.wasm`
- `web/rl/` — prebuilt raylib wasm + Emscripten loader (vendored, see `web/rl/README.md`)
- `web/wasm_exec.js` — Go's wasm runtime glue; must match the Go toolchain
  (`cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/` after a Go upgrade)
- `tools/serve` — minimal dev file server

## Mods

Content is data. A mod is a folder under `content/` with a `mod.json`
(`id` must equal the folder name) and any of `terrain.json`, `enemies.json`,
`objects.json`, `maps.json`, `places.json`, `world.json`, `classes.json`,
`arenas.json`. Each file maps an ID to an entry; `content/base/` shows every field.

Mods load in order: `base` first, the rest alphabetically. An entry with a new
ID is added. An entry with an existing ID is patched: only the fields it names
change. This raises the hunter's hit points and leaves everything else alone:

```json
{ "hunter": { "hp": 5 } }
```

Unknown field names, unknown references (terrain, enemy type, object,
behaviour, effect) and
bad colours are reported on screen at start instead of being ignored.

New enemy logic is Go code registered by name and then usable from
`enemies.json` as `"behavior": "<name>"`:

```go
func init() { game.RegisterBehavior("flee", func(s *game.State, e *game.Enemy) game.Decision { ... }) }
```

Built in: `chase`, `raid`, `wander`, `idle`.

An enemy type with `"squad": 3` appears as a squad of three on neighbouring
tiles. If it names a `"tactic"`, that tactic plans the whole squad at once;
`behavior` is then not needed. Tactics are registered like behaviours:

```go
func init() { game.RegisterTactic("swarm", func(s *game.State, squad []*game.Enemy) []game.Intent { ... }) }
```

Built in: `army` (the base game's troopers): keep `hold` tiles away from the
player until at least `strike` members can reach him in the same turn, then
strike together.

An enemy type with `"razes": true` destroys objects that name a `razesTo`
object when it steps on them (a colony becomes a ruin), and then turns into
its `becomes` type at full HP (a raider becomes a war mech).

Objects (`objects.json`) lie on a tile and run a list of effects when the
player steps on them; a colony is `heal` followed by `remove`. Effects are
registered the same way with `game.RegisterEffect`. Built in: `heal`,
`damage`, `remove`, `claim`.

The player sees along clear lines of sight up to `player.sight` tiles (leave
it out for unlimited range); terrain with `blocksSight` hides what is behind
it. Unexplored tiles are blank, explored tiles out of sight are dimmed and
show objects as last seen, and enemies and their plans show only in sight.

An object with `sight` and the `claim` effect is a lookout: once the player has
stepped on it, it reveals everything within that range, obstacles or not. An
object with `"jump": true` lets the player jump from it straight over one
impassable tile. The base game's sensor tower is both. `atWall` in a recipe's
object entry is the chance that it is placed where such a jump is possible.

A map recipe places objects (`objects`) and may have a `storm`: a front that
moves north by `wave` half-rows per turn and costs everyone inside `damage` HP.
With `"shape": "ring"` it closes in on the centre instead, `wave` rings per
turn, and leaves the centre and its neighbours free. Leave `storm` out for a
map without one.

A recipe can be long: `areas` stacks that many areas of `rows` rows from
south (start) to north (goal), with a `separator` row between two areas
(`terrain`, number of `gaps` per wall from south to north, gap `width`).
Features, objects and enemies count per area. With `viewRows` the camera
shows only that many rows and follows the player. `hills` raises round hills
per area (`count`, `radius`, `max` height, `flat` radius around the start);
in missions they are only drawn as relief. The base game's `long` recipe
("Expedition") uses all of this.

A recipe's `alarm` rises every turn and drops when the player defeats an
enemy. Above each threshold in `levels` reinforcements from `spawns` land
faster (`spawnRate` per level fills a counter up to `spawnAt`), between
`minRows` and `maxRows` rows north of the player (`maxRows` 0: any distance).
Leave `alarm` out for a map without reinforcements.

The world map is data too. `places.json` lists places with a position on the
map (`x`, `y` from 0 to 1), routes (`links`, valid in both directions) and the
map recipe for missions around them (`map`). `world.json` names the `start`
place, an optional `background` image in the mod folder (JPG or PNG) and the
map recipe that the main menu's "Überleben" plays (`survival`, optional).

On the world map, a click on your own place starts a mission in its
surroundings (repeatable), a click on a known neighbour travels there, and a
click on an unknown neighbour starts the mission that reaches it. Wins, losses
and collected objects are kept per place in the save game.

## Duel

The main menu offers a duel against the AI or for two players on one device.
`classes.json` lists the hero classes (`hp`, `ap` action points per turn,
`melee` damage at 1 tile, `ranged` damage at 2 to `range` tiles; 0 means the
class cannot do it). `arenas.json` lists the duel maps: size, `base`,
`hills` and `features` (both mirrored across the middle, so counts are per
half), a `tower` (`sight`, `height`, `minDistance` from the starts, always
equally far from both), a `storm` (best as `"shape": "ring"`, counted in
rounds) and the rule numbers: `firstAP` (player 2's first turn), `heightBonus`,
`trail` (how many of its owner's turn ends a left tile stays trail),
`trailBonus`, `shieldCost`, `shieldDamage`. A menu duel picks a random arena.

Terrain with `"hides": 2` (the base game's forest) hides a hero standing on it
from anyone further than 2 tiles away. Attacks go in straight lines only and
ignore sight; a player can aim at a tile he cannot see. The AI only knows what
it sees and tries every action sequence of its turn; its weights are
constants at the top of `duel/ai.go`.

## Building in mods

Mods are compiled into the binary for now: a new folder under `content/` must
be added to the `//go:embed` line in `content/content.go`, followed by a rebuild.
