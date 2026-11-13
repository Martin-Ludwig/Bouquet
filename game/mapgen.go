package game

import (
	"fmt"
	"slices"

	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

const (
	terrainTries = 50  // so oft wird neu gewürfelt, falls das Ziel unerreichbar ist
	placeTries   = 500 // Versuche pro zufälliger Platzierung
)

// New würfelt einen Einsatz nach dem Kartenrezept mapID. Derselbe seed
// ergibt dieselbe Karte und denselben Verlauf.
func New(c *Content, mapID string, seed uint64) (*State, error) {
	def, ok := c.Maps[mapID]
	if !ok {
		return nil, fmt.Errorf("unbekannte Karte %q (bekannt: %v)", mapID, sortedKeys(c.Maps))
	}
	s := NewBlank(c, def.Base, def.Width, def.TotalRows(), seed)
	s.areaRows, s.ViewRows = def.Rows, def.ViewRows
	s.PlayerHP, s.PlayerMoves, s.PlayerSight = def.Player.HP, def.Player.Moves, def.Player.Sight
	// Notlösung nach vielen Versuchen: nur die Trennmauern, deren
	// Durchgänge immer einen Weg lassen.
	for try := 0; ; try++ {
		withFeatures := try < terrainTries
		s.buildTerrain(def, withFeatures)
		if s.goalReachable() || !withFeatures {
			break
		}
	}
	s.raiseHills(def)
	s.Storm, s.Alarm = def.Storm, def.Alarm
	s.StormFront = s.stormStart()

	for area := range def.AreaCount() {
		s.placeObjects(def, area)
		for _, sp := range def.Enemies {
			ok := func(c hex.Coord) bool {
				return s.AreaOf(c) == area && s.Free(c) && c != s.Goal && s.ObjectAt(c) == nil && c.Dist(s.Player) >= sp.MinDistance
			}
			for range sp.Count {
				if pos, found := s.randomTile(ok); found {
					s.AddSquad(sp.Type, pos, ok)
				}
			}
		}
	}
	s.say("Dein Zug: laufen oder einen Nachbarn angreifen.")
	s.planEnemies()
	// Die leere Karte wurde mit unbegrenzter Sicht angelegt: Erkundet ist
	// erst, was der Spieler vom Start aus wirklich sieht.
	clear(s.Explored)
	clear(s.Remembered)
	s.UpdateVision()
	return s, nil
}

func (s *State) goalReachable() bool {
	field := s.Grid.DistanceField([]hex.Coord{s.Player}, s.Passable, -1)
	return field[s.Grid.Index(s.Goal)] != grid.Unreachable
}

// randomTile sucht ein zufälliges Feld, das ok erfüllt.
func (s *State) randomTile(ok func(hex.Coord) bool) (hex.Coord, bool) {
	for try := 0; try < placeTries; try++ {
		c := s.Grid.Coord(s.rng.IntN(s.Grid.Len()))
		if ok(c) {
			return c, true
		}
	}
	return hex.Coord{}, false
}

// nextToSeparator meldet, ob c an eine Reihe zwischen zwei Gebieten grenzt.
func (s *State) nextToSeparator(c hex.Coord) bool {
	for _, n := range s.Grid.Neighbors(c) {
		if s.AreaOf(n) == -1 {
			return true
		}
	}
	return false
}

// buildTerrain füllt die Karte mit dem Grundgelände, zieht die Trennmauern
// und streut, wenn withFeatures, das Gelände des Rezepts in jedes Gebiet.
func (s *State) buildTerrain(def *MapDef, withFeatures bool) {
	base := s.Content.Terrain[def.Base]
	for i := range s.Tiles {
		s.Tiles[i] = base
	}
	s.fields = nil
	if sep := def.Separator; sep != nil {
		for k := range def.AreaCount() - 1 {
			row := s.Grid.H - 1 - ((k+1)*(def.Rows+1) - 1)
			gaps := sep.Gaps[min(k, len(sep.Gaps)-1)]
			starts := s.gapStarts(gaps, sep.Width)
			for col := range s.Grid.W {
				open := slices.ContainsFunc(starts, func(st int) bool { return col >= st && col < st+sep.Width })
				if !open {
					s.Tiles[s.Grid.Index(hex.FromOffset(col, row))] = s.Content.Terrain[sep.Terrain]
				}
			}
		}
	}
	if !withFeatures {
		return
	}
	for area := range def.AreaCount() {
		for _, f := range def.Features {
			s.growFeature(f, area, base)
		}
	}
}

// gapStarts würfelt die Startspalten von n Durchgängen der Breite width,
// mit etwas Abstand zueinander. Passt es nicht, liegen sie gleichmäßig.
func (s *State) gapStarts(n, width int) []int {
	starts := make([]int, n)
	for try := 0; try < 200; try++ {
		for i := range starts {
			starts[i] = s.rng.IntN(max(1, s.Grid.W-width+1))
		}
		slices.Sort(starts)
		spaced := true
		for i := 1; i < n; i++ {
			spaced = spaced && starts[i]-starts[i-1] >= width+2
		}
		if spaced {
			return starts
		}
	}
	for i := range starts {
		starts[i] = (2*i+1)*s.Grid.W/(2*n) - width/2
	}
	return starts
}

// growFeature lässt ein Gelände Count-mal im Gebiet area wachsen: als Kette
// (Mauern) oder als Fläche (Wälder). Gebaut wird nur auf Grundgelände,
// rund um Start und Ziel bleibt es frei, und Unbegehbares kommt nicht vor
// die Durchgänge der Trennmauern.
func (s *State) growFeature(f FeatureDef, area int, base *TerrainDef) {
	terrain := s.Content.Terrain[f.Terrain]
	buildable := func(c hex.Coord) bool {
		return s.Grid.Contains(c) && s.Terrain(c) == base && s.AreaOf(c) == area &&
			c.Dist(s.Player) >= 3 && c.Dist(s.Goal) >= 2 &&
			(terrain.Passable || !s.nextToSeparator(c))
	}
	for range f.Count {
		start, ok := s.randomTile(buildable)
		if !ok {
			return
		}
		want := f.Size[0] + s.rng.IntN(f.Size[1]-f.Size[0]+1)
		cells := []hex.Coord{start}
		s.Tiles[s.Grid.Index(start)] = terrain
		for guard := 0; len(cells) < want && guard < 100; guard++ {
			// Kette: am letzten Feld weiterwachsen. Fläche: an irgendeinem.
			from := cells[len(cells)-1]
			if f.Shape == "blob" {
				from = cells[s.rng.IntN(len(cells))]
			}
			var opts []hex.Coord
			for _, n := range s.Grid.Neighbors(from) {
				if buildable(n) {
					opts = append(opts, n)
				}
			}
			if len(opts) == 0 {
				continue
			}
			next := opts[s.rng.IntN(len(opts))]
			s.Tiles[s.Grid.Index(next)] = terrain
			cells = append(cells, next)
		}
	}
	s.fields = nil
}

// raiseHills würfelt die Hügel jedes Gebiets. Rund um den Start bleibt es
// flach.
func (s *State) raiseHills(def *MapDef) {
	h := def.Hills
	if h == nil {
		return
	}
	for area := range def.AreaCount() {
		inArea := func(c hex.Coord) bool { return s.AreaOf(c) == area }
		var hills []Hill
		for range h.RollCount(s.rng) {
			if at, ok := s.randomTile(inArea); ok {
				hills = append(hills, h.Roll(s.rng, at))
			}
		}
		heights := Heights(s.Grid, hills, h.Max)
		for i := range s.Height {
			if c := s.Grid.Coord(i); inArea(c) && c.Dist(s.Player) > h.Flat {
				s.Height[i] = heights[i]
			}
		}
	}
}

// placeObjects verteilt die Objekte des Rezepts im Gebiet area: auf
// unberührtem Grund, mit Abstand zueinander.
func (s *State) placeObjects(def *MapDef, area int) {
	base := s.Content.Terrain[def.Base]
	for _, sp := range def.Objects {
		var placed []hex.Coord
		fits := func(c hex.Coord) bool {
			if s.AreaOf(c) != area || s.Terrain(c) != base || s.ObjectAt(c) != nil ||
				c.Dist(s.Player) < max(sp.MinDistance, 1) || c.Dist(s.Goal) < 2 {
				return false
			}
			for _, p := range placed {
				if p.Dist(c) < sp.MinSpacing {
					return false
				}
			}
			return true
		}
		for range sp.Count {
			var pos hex.Coord
			ok := false
			if s.rng.Float64() < sp.AtWall {
				pos, ok = s.randomTile(func(c hex.Coord) bool { return fits(c) && len(s.JumpTargets(c)) > 0 })
			}
			if !ok {
				pos, ok = s.randomTile(fits)
			}
			if !ok {
				break
			}
			s.AddObject(sp.Type, pos)
			placed = append(placed, pos)
		}
	}
}
