// Package duel enthält die Regeln des Duells: zwei Helden auf einer
// gespiegelten Hex-Karte, mit Aktionspunkten, geraden Angriffen, Sicht und
// einem Sturm, der sich zur Mitte zusammenzieht. Die Zahlen kommen aus den
// Inhalten (Klassen und Arenen, siehe bouquet/game). Wie bouquet/game
// importiert es raylib nicht und läuft unter "go test ./duel".
//
// Aufteilung:
//
//	arena.go   die Karte: bauen und abfragen
//	rules.go   Zustand, Aktionen, Schaden, Zugende
//	vision.go  was ein Held sieht
//	duel.go    ein laufendes Duell, wie die Oberfläche es bedient
//	ai.go      die KI
package duel

import (
	"fmt"
	"math/rand/v2"

	"bouquet/game"
	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Arena ist die Karte eines Duells. Sie ändert sich im Spiel nicht.
type Arena struct {
	Def    *game.ArenaDef
	Grid   grid.Grid
	Tiles  []*game.TerrainDef // pro Feld; nil: kein Feld (Lücke am Rand, damit die Spiegelung aufgeht)
	Height []int              // pro Feld, 0 ist flach
	Tower  hex.Coord          // Aussichtsturm, nur gültig mit HasTower
	Start  [2]hex.Coord       // Startfelder von Spieler 1 (unten) und 2 (oben)
	Center hex.Coord
	Radius int // Abstand des äußersten Feldes zur Mitte

	HasTower bool
}

const buildTries = 60 // so oft wird neu gewürfelt, bis beide Starts verbunden sind

// NewArena würfelt eine Arena nach dem Rezept id.
func NewArena(c *game.Content, id string, rng *rand.Rand) (*Arena, error) {
	def, ok := c.Arenas[id]
	if !ok {
		return nil, fmt.Errorf("unbekannte Arena %q", id)
	}
	for try := 0; ; try++ {
		// Notlösung nach vielen Versuchen: ohne Gelände ist alles verbunden.
		withFeatures := try < buildTries
		a := build(c, def, rng, withFeatures)
		if (a.connected() && a.placeTower(rng)) || !withFeatures {
			return a, nil
		}
	}
}

func build(c *game.Content, def *game.ArenaDef, rng *rand.Rand, withFeatures bool) *Arena {
	g := grid.New(def.Width, def.Rows)
	startCol := def.Width / 2 &^ 1 // gerade Spalte, sonst liegt der Start unten in einer Lücke
	a := &Arena{
		Def:    def,
		Grid:   g,
		Tiles:  make([]*game.TerrainDef, g.Len()),
		Height: make([]int, g.Len()),
		Start:  [2]hex.Coord{hex.FromOffset(startCol, def.Rows-1), hex.FromOffset(startCol, 0)},
		Center: hex.FromOffset(def.Width/2, def.Rows/2),
	}
	base := c.Terrain[def.Base]
	for i := range a.Tiles {
		col, row := g.Coord(i).Offset()
		if col&1 == 0 || row < def.Rows-1 {
			a.Tiles[i] = base
		}
	}
	for i := range a.Tiles {
		if c := g.Coord(i); a.OnMap(c) {
			a.Radius = max(a.Radius, c.Dist(a.Center))
		}
	}
	a.raiseHills(rng)
	if withFeatures {
		for _, f := range def.Features {
			a.grow(f, c.Terrain[f.Terrain], base, rng)
		}
	}
	return a
}

// Mirror ist das Gegenstück von c auf der anderen Kartenhälfte. Ungerade
// Spalten liegen eine halbe Reihe tiefer, deshalb ist die Spiegelreihe dort
// eine weiter oben.
func (a *Arena) Mirror(c hex.Coord) hex.Coord {
	col, row := c.Offset()
	if col&1 == 1 {
		return hex.FromOffset(col, a.Grid.H-2-row)
	}
	return hex.FromOffset(col, a.Grid.H-1-row)
}

// farFromStarts: Gelände und Hügel bleiben mindestens 2 Felder von beiden
// Starts weg.
func (a *Arena) farFromStarts(c hex.Coord) bool {
	return c.Dist(a.Start[0]) >= 2 && c.Dist(a.Start[1]) >= 2
}

func (a *Arena) raiseHills(rng *rand.Rand) {
	h := a.Def.Hills
	if h == nil {
		return
	}
	var hills []game.Hill
	for range h.RollCount(rng) {
		hill := h.Roll(rng, a.randomTile(rng))
		mirrored := hill
		mirrored.At = a.Mirror(hill.At)
		hills = append(hills, hill, mirrored)
	}
	heights := game.Heights(a.Grid, hills, h.Max)
	for i := range a.Height {
		c := a.Grid.Coord(i)
		if a.OnMap(c) && c.Dist(a.Start[0]) > h.Flat && c.Dist(a.Start[1]) > h.Flat {
			a.Height[i] = heights[i]
		}
	}
}

func (a *Arena) randomTile(rng *rand.Rand) hex.Coord {
	for {
		if c := a.Grid.Coord(rng.IntN(a.Grid.Len())); a.OnMap(c) {
			return c
		}
	}
}

// grow lässt ein Gelände Count-mal wachsen, immer gespiegelt: als Kette
// (Mauern) oder als Fläche (Wald).
func (a *Arena) grow(f game.FeatureDef, terrain, base *game.TerrainDef, rng *rand.Rand) {
	buildable := func(c hex.Coord) bool {
		return a.OnMap(c) && a.Terrain(c) == base && a.farFromStarts(c) && c != a.Center
	}
	set := func(c hex.Coord) {
		a.Tiles[a.Grid.Index(c)] = terrain
		if m := a.Mirror(c); a.OnMap(m) {
			a.Tiles[a.Grid.Index(m)] = terrain
		}
	}
	for range f.Count {
		start := a.randomTile(rng)
		if !buildable(start) {
			continue
		}
		want := f.Size[0] + rng.IntN(f.Size[1]-f.Size[0]+1)
		cells := []hex.Coord{start}
		set(start)
		for guard := 0; len(cells) < want && guard < 50; guard++ {
			from := cells[len(cells)-1]
			if f.Shape == "blob" {
				from = cells[rng.IntN(len(cells))]
			}
			var opts []hex.Coord
			for _, n := range a.Grid.Neighbors(from) {
				if buildable(n) {
					opts = append(opts, n)
				}
			}
			if len(opts) > 0 {
				next := opts[rng.IntN(len(opts))]
				set(next)
				cells = append(cells, next)
			}
		}
	}
}

// connected meldet, ob ein Weg von Start zu Start führt.
func (a *Arena) connected() bool {
	field := a.Grid.DistanceField([]hex.Coord{a.Start[0]}, a.Passable, -1)
	return field[a.Grid.Index(a.Start[1])] != grid.Unreachable
}

// placeTower stellt den Aussichtsturm auf ein Feld, das von beiden Starts
// genau gleich weit entfernt ist. Rückgabe: ob es eines gab (ohne Turm im
// Rezept immer).
func (a *Arena) placeTower(rng *rand.Rand) bool {
	t := a.Def.Tower
	if t == nil {
		return true
	}
	var spots []hex.Coord
	for i := range a.Tiles {
		c := a.Grid.Coord(i)
		d := c.Dist(a.Start[0])
		if a.Passable(c) && !a.Terrain(c).BlocksSight && d == c.Dist(a.Start[1]) && d >= t.MinDistance {
			spots = append(spots, c)
		}
	}
	if len(spots) == 0 {
		return false
	}
	a.Tower, a.HasTower = spots[rng.IntN(len(spots))], true
	a.Height[a.Grid.Index(a.Tower)] = 0
	return true
}

// ---------- Abfragen ----------

// OnMap meldet, ob c ein Feld der Arena ist.
func (a *Arena) OnMap(c hex.Coord) bool {
	i := a.Grid.Index(c)
	return i >= 0 && a.Tiles[i] != nil
}

// Terrain ist das Gelände von c. c muss auf der Karte liegen.
func (a *Arena) Terrain(c hex.Coord) *game.TerrainDef { return a.Tiles[a.Grid.Index(c)] }

// Passable meldet, ob man c betreten kann (ohne Rücksicht auf Helden).
func (a *Arena) Passable(c hex.Coord) bool { return a.OnMap(c) && a.Terrain(c).Passable }

// IsTower meldet, ob auf c der Aussichtsturm steht.
func (a *Arena) IsTower(c hex.Coord) bool { return a.HasTower && c == a.Tower }

// HeightAt ist die Höhe des Bodens auf c.
func (a *Arena) HeightAt(c hex.Coord) int {
	if !a.OnMap(c) {
		return 0
	}
	return a.Height[a.Grid.Index(c)]
}

// Elevation ist, wie hoch ein Held auf c steht: auf dem Turm über jedem
// Hügel, sonst die Höhe des Bodens.
func (a *Arena) Elevation(c hex.Coord) int {
	if a.IsTower(c) {
		return a.Def.Tower.Height
	}
	return a.HeightAt(c)
}

// SightClear meldet, ob die Sicht von b nach c frei ist. Es versperren
// Lücken, Gelände mit blocksSight und Hügel, die höher sind als beide Enden.
func (a *Arena) SightClear(b, c hex.Coord) bool {
	top := max(a.HeightAt(b), a.HeightAt(c))
	return a.Grid.LineClear(b, c, func(x hex.Coord) bool {
		return !a.OnMap(x) || a.Terrain(x).BlocksSight || a.HeightAt(x) > top
	})
}

// StormRadius ist der Radius um die Mitte, der in Runde round noch frei vom
// Sturm ist. Der Sturm rückt am Ende jeder Runde vor (siehe
// game.StormDef.Advance) und lässt die Mitte mit ihren Nachbarn frei.
func (a *Arena) StormRadius(round int) int {
	st := a.Def.Storm
	if st == nil {
		return a.Radius
	}
	r := a.Radius
	for k := 1; k < round; k++ {
		r -= st.Advance(k)
	}
	return max(1, r)
}

// InStorm meldet, ob c in Runde round im Sturm liegt.
func (a *Arena) InStorm(c hex.Coord, round int) bool {
	return a.Def.Storm != nil && c.Dist(a.Center) > a.StormRadius(round)
}
