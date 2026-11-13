package game

import (
	"fmt"
	"slices"

	"bouquet/lib/hex"
)

// StormDepth misst, wie weit südlich ein Feld liegt, in halben Feldreihen.
// Das passt zur Zeichnung: Ungerade Spalten liegen eine halbe Reihe tiefer.
func StormDepth(c hex.Coord) int { return 2*c.R + c.Q }

// Center ist das Feld in der Mitte der Karte.
func (s *State) Center() hex.Coord { return hex.FromOffset(s.Grid.W/2, s.Grid.H/2) }

// StormDistance ist der Abstand von c, den der Sturm mit der Front
// vergleicht: bei einer Front die Tiefe nach Süden, bei einem Ring der
// Abstand zur Kartenmitte. Felder mit einem Abstand über der Front liegen im
// Sturm.
func (s *State) StormDistance(c hex.Coord) int {
	if s.Storm != nil && s.Storm.Ring() {
		return c.Dist(s.Center())
	}
	return StormDepth(c)
}

// stormStart ist die Front zu Beginn: am äußersten Feld, also liegt noch
// nichts im Sturm.
func (s *State) stormStart() int {
	far := 0
	for i := range s.Tiles {
		far = max(far, s.StormDistance(s.Grid.Coord(i)))
	}
	return far
}

// InStorm meldet, ob c gerade im Sturm liegt.
func (s *State) InStorm(c hex.Coord) bool {
	return s.Storm != nil && s.StormDistance(c) > s.StormFront
}

// NextStormFront ist die Front nach dem Ende des laufenden Zugs (für die
// Vorschau). Ein Ring lässt die Mitte und ihre Nachbarn frei.
func (s *State) NextStormFront() int {
	if s.Storm == nil {
		return s.StormFront
	}
	next := s.StormFront - s.Storm.Advance(s.Turn)
	if s.Storm.Ring() {
		next = max(1, next)
	}
	return next
}

// advanceStorm rückt die Front vor; wer danach im Sturm steht, verliert HP.
func (s *State) advanceStorm() {
	if s.Storm == nil {
		return
	}
	s.StormFront = s.NextStormFront()
	s.Enemies = slices.DeleteFunc(s.Enemies, func(e *Enemy) bool {
		if !s.InStorm(e.Pos) {
			return false
		}
		e.HP -= s.Storm.Damage
		if e.HP > 0 {
			return false
		}
		if s.CanSee(e.Pos) {
			s.say(fmt.Sprintf("%s wird vom %s zerstört.", e.Def.Name, s.Storm.Name))
		}
		return true
	})
	if s.InStorm(s.Player) {
		if s.hurtPlayer(s.Storm.Damage) {
			s.say(fmt.Sprintf("Der %s trifft dich: -%d HP.", s.Storm.Name, s.Storm.Damage))
		} else {
			s.say(fmt.Sprintf("Der %s hat dich erwischt.", s.Storm.Name))
		}
	}
}
