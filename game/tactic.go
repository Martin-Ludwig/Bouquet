package game

import (
	"slices"

	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Tactic plant den Zug eines ganzen Trupps auf einmal, für Gegnertypen, die
// sie als "tactic" nennen. Sie liefert einen Intent pro Mitglied, in der
// Reihenfolge, in der sie ausgeführt werden.
//
// Schritte plant sie mit s.PlanStep: Das setzt das Mitglied in der Planung
// schon auf das neue Feld, damit die übrigen damit rechnen. Danach setzt das
// Spiel alles zurück. Zufall kommt aus s.Rand().
type Tactic func(s *State, squad []*Enemy) []Intent

var tactics = map[string]Tactic{}

// RegisterTactic macht eine Taktik unter name für Gegnertypen verfügbar
// ("tactic" in enemies.json). Ein vorhandener Name wird ersetzt. Aufrufen
// vor dem Laden der Inhalte, am einfachsten in einer init-Funktion.
func RegisterTactic(name string, t Tactic) { tactics[name] = t }

// TacticNames listet die registrierten Taktiken, sortiert.
func TacticNames() []string { return sortedKeys(tactics) }

func init() { RegisterTactic("army", Army) }

// Army führt einen Trupp als Armee. Bemerkt sie den Spieler (ein Mitglied
// sieht ihn und ist höchstens "aggro" Felder Weg entfernt, Standard 8), hält
// sie "hold" Felder Abstand (Standard 2), bis mindestens "strike" Mitglieder
// (Standard 2) im selben Zug neben ihn kommen. Dann stürmen diese heran und
// schlagen gemeinsam zu.
//
// Ohne Spieler in Sicht geht die Armee zur letzten bekannten Position, oder
// das erste Mitglied wandert umher (Regler "wander") und die übrigen folgen.
func Army(s *State, squad []*Enemy) []Intent {
	def := squad[0].Def
	aggro, hold, strike := def.Param("aggro", 8), int(def.Param("hold", 2)), int(def.Param("strike", 2))
	field := s.FieldTo(s.Player)
	away := func(e *Enemy) int { return field[s.Grid.Index(e.Pos)] }

	sees := slices.ContainsFunc(squad, func(m *Enemy) bool {
		d := away(m)
		return d != grid.Unreachable && float64(d) <= aggro && s.SightClear(m.Pos, s.Player)
	})
	plans := map[*Enemy]*Intent{}
	for _, m := range squad {
		plans[m] = &Intent{Enemy: m, Seen: sees}
	}
	var order []*Enemy // wer zuerst geht, geht auch bei der Ausführung zuerst
	step := func(m *Enemy, to hex.Coord, wander bool) {
		if !slices.Contains(order, m) {
			order = append(order, m)
		}
		plans[m].Steps = append(plans[m].Steps, s.PlanStep(m, to, wander))
	}
	approach := func(m *Enemy, f []int) {
		if to, ok := s.StepToward(m.Pos, f); ok {
			step(m, to, false)
		}
	}

	switch {
	case sees:
		strikers := s.strikers(squad, field, hold)
		if len(strikers) >= strike {
			for _, m := range strikers {
				if away(m) > 1 {
					approach(m, field)
				}
				plans[m].Attack = true
			}
			for _, m := range squad {
				if !plans[m].Attack {
					approach(m, field)
				}
			}
			break
		}
		// Abstand halten: von weiter weg herankommen, aus der Nähe zurückweichen.
		for _, m := range squad {
			switch d := away(m); {
			case d > hold:
				approach(m, field)
			case d != grid.Unreachable && d < hold:
				for _, n := range s.Grid.Neighbors(m.Pos) {
					if field[s.Grid.Index(n)] == hold && s.Free(n) {
						step(m, n, false)
						break
					}
				}
			}
		}

	case squadMemory(squad) != nil:
		target := s.FieldTo(*squadMemory(squad))
		for _, m := range squad {
			approach(m, target)
		}

	default:
		leader := squad[0]
		if d := Wander(s, leader); d.Move {
			step(leader, d.To, true)
		}
		follow := s.FieldTo(leader.Pos)
		for _, m := range squad[1:] {
			if m.Pos.Dist(leader.Pos) > 1 {
				approach(m, follow)
			}
		}
	}

	for _, m := range squad {
		if !slices.Contains(order, m) {
			order = append(order, m)
		}
	}
	out := make([]Intent, len(order))
	for i, m := range order {
		out[i] = *plans[m]
	}
	return out
}

// strikers sind die Mitglieder, die in diesem Zug neben den Spieler kommen:
// Sie stehen schon dort oder sind höchstens hold Felder entfernt und finden
// einen freien Schritt auf ein Nachbarfeld des Spielers. Probeweise
// durchgespielt, damit zwei nicht dasselbe Feld nehmen.
func (s *State) strikers(squad []*Enemy, field []int, hold int) []*Enemy {
	restore := s.snapshot()
	defer restore()
	var out []*Enemy
	for _, m := range squad {
		switch d := field[s.Grid.Index(m.Pos)]; {
		case d == 1:
			out = append(out, m)
		case d != grid.Unreachable && d <= hold:
			if to, ok := s.StepToward(m.Pos, field); ok && field[s.Grid.Index(to)] == 1 {
				s.moveEnemy(m, to)
				out = append(out, m)
			}
		}
	}
	return out
}

// squadMemory ist die letzte bekannte Position des Spielers, die ein
// Mitglied kennt, solange noch keines dort steht; sonst nil.
func squadMemory(squad []*Enemy) *hex.Coord {
	for _, m := range squad {
		if m.LastSeen == nil {
			continue
		}
		for _, o := range squad {
			if o.Pos == *m.LastSeen {
				return nil
			}
		}
		return m.LastSeen
	}
	return nil
}
