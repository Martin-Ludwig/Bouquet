package game

import (
	"fmt"

	"bouquet/lib/hex"
)

// AlarmLevel ist die Alarmstufe: 0, solange der Alarm unter der ersten
// Schwelle liegt, dann eine Stufe pro überschrittener Schwelle.
func (s *State) AlarmLevel() int {
	if s.Alarm == nil {
		return 0
	}
	level := 0
	for _, min := range s.Alarm.Levels {
		if s.AlarmValue >= min {
			level++
		}
	}
	return level
}

// calmAlarm senkt den Alarm, nachdem der Spieler einen Gegner besiegt hat.
// Rückgabe: um wie viel er gesunken ist.
func (s *State) calmAlarm() int {
	if s.Alarm == nil {
		return 0
	}
	before := s.AlarmValue
	s.AlarmValue = max(0, s.AlarmValue-s.Alarm.Kill)
	return before - s.AlarmValue
}

// updateAlarm läuft am Zugende: Der Alarm steigt, und je nach Stufe füllt
// sich der Nachschub-Zähler. Ist er voll, landet ein Gegner.
func (s *State) updateAlarm() {
	a := s.Alarm
	if a == nil {
		return
	}
	s.AlarmValue = min(a.Max, s.AlarmValue+a.PerTurn)
	s.spawnCounter += a.SpawnRate[s.AlarmLevel()]
	// Nur zurücksetzen, wenn wirklich jemand gelandet ist; sonst im nächsten
	// Zug erneut versuchen.
	if s.spawnCounter >= a.SpawnAt && s.spawnReinforcement() {
		s.spawnCounter = 0
	}
}

// spawnReinforcement setzt einen zufälligen Gegner (oder Trupp) aus
// alarm.spawns auf die Karte: bevorzugt im Band nördlich vom Spieler, sonst
// irgendwo mit Abstand außerhalb des Sturms. Rückgabe: ob jemand gelandet
// ist.
func (s *State) spawnReinforcement() bool {
	a := s.Alarm
	if len(a.Spawns) == 0 {
		return false
	}
	def := s.Content.Enemies[a.Spawns[s.rng.IntN(len(a.Spawns))]]
	usable := func(c hex.Coord) bool {
		return s.Free(c) && c != s.Goal && s.ObjectAt(c) == nil && !s.InStorm(c)
	}
	what := def.Name
	if def.Squad > 1 {
		what = "ein Trupp " + def.Name
	}
	_, playerRow := s.Player.Offset()
	north := func(c hex.Coord) bool {
		_, row := c.Offset()
		return usable(c) && row <= playerRow-a.MinRows && (a.MaxRows == 0 || row >= playerRow-a.MaxRows) && c.Dist(s.Player) >= 4
	}
	if pos, ok := s.randomTile(north); ok {
		s.AddSquad(def.ID, pos, north)
		s.say(fmt.Sprintf("Landung: %s erscheint im Norden.", what))
		return true
	}
	anywhere := func(c hex.Coord) bool { return usable(c) && s.AreaOf(c) != -1 && c.Dist(s.Player) >= a.MinRows }
	if pos, ok := s.randomTile(anywhere); ok {
		s.AddSquad(def.ID, pos, anywhere)
		s.say(fmt.Sprintf("Landung: %s erscheint.", what))
		return true
	}
	return false
}
