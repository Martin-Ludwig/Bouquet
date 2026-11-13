package game

import (
	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Sighting ist, was der Spieler zuletzt auf einem Feld gesehen hat. Def ist
// nil, wenn dort kein Objekt lag.
type Sighting struct {
	Def   *ObjectDef
	Owned bool
}

// UpdateVision berechnet neu, was der Spieler sieht. Er selbst sieht bis
// PlayerSight Felder weit (0: unbegrenzt), aber nur mit freier Sichtlinie.
// Besetzte Objekte mit sight (Sensortürme) sehen in ihrem Umkreis alles, auch
// über Hindernisse hinweg.
//
// Was er einmal gesehen hat, bleibt erkundet; Objekte dort bleiben so in
// Erinnerung, wie er sie zuletzt gesehen hat.
func (s *State) UpdateVision() {
	for i := range s.Visible {
		c := s.Grid.Coord(i)
		inRange := s.PlayerSight <= 0 || c.Dist(s.Player) <= s.PlayerSight
		s.Visible[i] = inRange && s.SightClear(s.Player, c)
	}
	for _, o := range s.Objects {
		if !o.Owned || o.Def.Sight <= 0 {
			continue
		}
		for i := range s.Visible {
			if s.Grid.Coord(i).Dist(o.Pos) <= o.Def.Sight {
				s.Visible[i] = true
			}
		}
	}
	for i, vis := range s.Visible {
		if vis {
			s.Explored[i] = true
			s.Remembered[i] = Sighting{}
		}
	}
	for _, o := range s.Objects {
		if i := s.Grid.Index(o.Pos); s.Visible[i] {
			s.Remembered[i] = Sighting{Def: o.Def, Owned: o.Owned}
		}
	}
}

// CanSee meldet, ob der Spieler das Feld c gerade sieht.
func (s *State) CanSee(c hex.Coord) bool {
	i := s.Grid.Index(c)
	return i >= 0 && s.Visible[i]
}

// JumpTargets liefert die Felder, auf die man von from aus springen kann:
// geradeaus über genau ein unbegehbares Feld auf das begehbare Feld dahinter.
func (s *State) JumpTargets(from hex.Coord) []hex.Coord {
	var out []hex.Coord
	for _, d := range hex.Dirs {
		over, land := from.Add(d), from.Add(d).Add(d)
		if s.Grid.Contains(over) && !s.Passable(over) && s.Passable(land) {
			out = append(out, land)
		}
	}
	return out
}

// Reach ist das Distanzfeld der Spielerbewegung: für jedes Feld die Zahl der
// Schritte vom Spieler aus, höchstens PlayerMoves. 0 ist das Feld des
// Spielers, grid.Unreachable alles außer Reichweite.
//
// Steht der Spieler auf einem Objekt mit jump (Sensorturm), kommen die
// Sprungziele dazu; ein Sprung zählt wie ein Zug mit voller Laufweite.
func (s *State) Reach() []int {
	reach := s.Grid.DistanceField([]hex.Coord{s.Player}, s.Free, s.PlayerMoves)
	if o := s.ObjectAt(s.Player); o != nil && o.Def.Jump {
		for _, land := range s.JumpTargets(s.Player) {
			if i := s.Grid.Index(land); reach[i] == grid.Unreachable && s.Free(land) {
				reach[i] = s.PlayerMoves
			}
		}
	}
	return reach
}
