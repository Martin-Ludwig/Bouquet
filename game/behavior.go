package game

import (
	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Decision ist ein einzelner Schritt, den ein Verhalten für einen Gegner
// vorschlägt.
type Decision struct {
	To     hex.Coord // Zielfeld, nur gültig mit Move
	Move   bool      // false: der Gegner bleibt stehen
	Wander bool      // der Schritt ist zufällig, also kein gezieltes Vorgehen
	Seen   bool      // der Gegner sieht den Spieler gerade
}

// Behavior entscheidet den nächsten Schritt eines Gegners. Es darf den
// Zustand nur lesen. Ein Gegner mit speed 2 wird zweimal gefragt; beim
// zweiten Mal steht er in s schon auf dem Feld des ersten Schritts.
//
// Zufall kommt aus s.Rand(), damit derselbe Seed denselben Verlauf ergibt.
type Behavior func(s *State, e *Enemy) Decision

var behaviors = map[string]Behavior{}

// RegisterBehavior macht ein Verhalten unter name für Gegnertypen verfügbar
// ("behavior" in enemies.json). Ein vorhandener Name wird ersetzt; so kann
// ein Mod auch ein eingebautes Verhalten austauschen. Aufrufen vor dem Laden
// der Inhalte, am einfachsten in einer init-Funktion.
func RegisterBehavior(name string, b Behavior) { behaviors[name] = b }

// BehaviorNames listet die registrierten Verhalten, sortiert.
func BehaviorNames() []string { return sortedKeys(behaviors) }

func init() {
	RegisterBehavior("chase", Chase)
	RegisterBehavior("raid", Raid)
	RegisterBehavior("wander", Wander)
	RegisterBehavior("idle", func(*State, *Enemy) Decision { return Decision{} })
}

// Chase verfolgt den Spieler, solange er in Sicht und höchstens "aggro"
// Felder Weg entfernt ist. Verliert der Gegner ihn, geht er zur letzten
// bekannten Position und läuft danach ziellos umher.
//
// Regler: aggro (Standard 8), wander (siehe Wander).
func Chase(s *State, e *Enemy) Decision {
	if d, ok := pursue(s, e, e.Def.Param("aggro", 8)); ok {
		return d
	}
	return Wander(s, e)
}

// pursue ist der gezielte Teil von Chase: auf den Spieler zu, wenn er in
// Sicht und höchstens aggro Felder Weg entfernt ist, sonst zur letzten
// bekannten Position. ok ist false, wenn es nichts zu verfolgen gibt.
func pursue(s *State, e *Enemy, aggro float64) (d Decision, ok bool) {
	field := s.FieldTo(s.Player)
	dist := field[s.Grid.Index(e.Pos)]
	if dist != grid.Unreachable && float64(dist) <= aggro && s.SightClear(e.Pos, s.Player) {
		to, ok := s.StepToward(e.Pos, field)
		return Decision{To: to, Move: ok, Seen: true}, true
	}
	if e.LastSeen != nil && e.Pos != *e.LastSeen {
		if to, ok := s.StepToward(e.Pos, s.FieldTo(*e.LastSeen)); ok {
			return Decision{To: to, Move: true}, true
		}
	}
	return Decision{}, false
}

// Raid zieht zum nächsten zerstörbaren Objekt, das der Gegner sieht. Kommt
// ihm der Spieler nahe, geht er stattdessen auf ihn los; gibt es nichts mehr
// zu zerstören, jagt er nur noch den Spieler.
//
// Es zählen nur Objekte, die höchstens "raidRange" Felder vom Spieler
// entfernt sind, damit er die Zerstörung miterlebt und verhindern kann.
//
// Regler: aggro (Spieler in der Nähe, Standard 3), huntAggro (ohne Objekte,
// Standard 8), raidRange (Standard 10), wander (siehe Wander).
func Raid(s *State, e *Enemy) Decision {
	targets := s.Razable()
	if len(targets) == 0 {
		if d, ok := pursue(s, e, e.Def.Param("huntAggro", 8)); ok {
			return d
		}
		return Wander(s, e)
	}
	near, chasing := pursue(s, e, e.Def.Param("aggro", 3))
	if chasing && near.Seen {
		return near
	}
	var inSight []hex.Coord
	for _, t := range targets {
		if float64(t.Dist(s.Player)) <= e.Def.Param("raidRange", 10) && s.SightClear(e.Pos, t) {
			inSight = append(inSight, t)
		}
	}
	if len(inSight) > 0 {
		to, ok := s.StepToward(e.Pos, s.FieldToAny(inSight))
		return Decision{To: to, Move: ok}
	}
	if chasing {
		return near
	}
	return Wander(s, e)
}

// Wander geht mit Wahrscheinlichkeit "wander" (Standard 0.3) auf ein
// zufälliges freies Nachbarfeld und bleibt sonst stehen.
func Wander(s *State, e *Enemy) Decision {
	if s.Rand().Float64() >= e.Def.Param("wander", 0.3) {
		return Decision{}
	}
	var opts []hex.Coord
	for _, n := range s.Grid.Neighbors(e.Pos) {
		if s.Free(n) {
			opts = append(opts, n)
		}
	}
	if len(opts) == 0 {
		return Decision{}
	}
	return Decision{To: opts[s.Rand().IntN(len(opts))], Move: true, Wander: true}
}
