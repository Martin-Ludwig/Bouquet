package duel

import "bouquet/lib/hex"

// TileSeen meldet, ob ein Held auf from das Feld c sieht: sein eigenes Feld
// und die Nachbarn immer, vom Turm aus alles im Umkreis, sonst mit freier
// Sichtlinie.
func (a *Arena) TileSeen(from, c hex.Coord) bool {
	d := from.Dist(c)
	return d <= 1 || (a.IsTower(from) && d <= a.Def.Tower.Sight) || a.SightClear(from, c)
}

// HeroSeen meldet, ob ein Held auf from einen anderen auf c sieht. Wer auf
// Gelände mit "hides" steht (Wald), ist nur aus der Nähe zu sehen; vom Turm
// aus sieht man ihn trotzdem.
func (a *Arena) HeroSeen(from, c hex.Coord) bool {
	d := from.Dist(c)
	if d <= 1 || (a.IsTower(from) && d <= a.Def.Tower.Sight) {
		return true
	}
	hides := a.Terrain(c).Hides
	return a.TileSeen(from, c) && (hides == 0 || d <= hides)
}

// Sees meldet, ob Spieler p den Gegner gerade sieht.
func (a *Arena) Sees(s *State, p int) bool {
	return a.HeroSeen(s.Heroes[p].Pos, s.Heroes[1-p].Pos)
}
