package game

import (
	"fmt"
	"slices"

	"bouquet/lib/hex"
)

// Step ist ein geplanter Schritt eines Gegners.
type Step struct {
	To     hex.Coord
	Wander bool // zufälliger Schritt
	Razes  bool // zerstört dabei das Objekt auf To
}

// Intent ist der geplante Zug eines Gegners. Er wird zu Beginn des
// Spielerzugs festgelegt, angezeigt und nach der Aktion des Spielers genau so
// ausgeführt: Die Gegner reagieren nicht mehr auf das, was der Spieler tut.
type Intent struct {
	Enemy  *Enemy
	Attack bool   // steht neben dem Spieler und schlägt zu
	Steps  []Step // sonst: bis zu speed Schritte
	Seen   bool   // hat den Spieler beim Planen gesehen
}

// planEnemies legt für jeden Gegner fest, was er nach der nächsten Aktion des
// Spielers tut. Die Gegner werden nacheinander durchgespielt, damit sie sich
// gegenseitig im Weg stehen; danach stehen alle wieder auf ihrem Feld.
// Ein Trupp mit Taktik wird als Ganzes geplant.
func (s *State) planEnemies() {
	s.fields = nil
	s.plannedAt = s.Player
	s.Intents = make([]Intent, 0, len(s.Enemies))

	restore := s.snapshot()
	planned := map[int]bool{} // Trupps, die schon dran waren
	for _, e := range s.Enemies {
		tactic, ok := tactics[e.Def.Tactic]
		switch {
		case !ok:
			s.Intents = append(s.Intents, s.planSingle(e))
		case e.Squad == 0 || !planned[e.Squad]:
			planned[e.Squad] = true
			s.Intents = append(s.Intents, tactic(s, s.SquadOf(e))...)
		}
	}
	restore()
}

// planSingle plant einen Gegner für sich allein: Steht er neben dem Spieler,
// greift er an, sonst geht er bis zu speed Schritte nach seinem Verhalten.
func (s *State) planSingle(e *Enemy) Intent {
	in := Intent{Enemy: e}
	if e.Pos.Dist(s.Player) == 1 {
		in.Attack, in.Seen = true, true
		return in
	}
	// Verhalten und Schrittzahl gelten ab Zugbeginn: Wer sich unterwegs
	// verwandelt, ist erst im nächsten Zug der Neue.
	behave, speed := behaviors[e.Def.Behavior], e.Def.Speed
	for range speed {
		d := behave(s, e)
		in.Seen = in.Seen || d.Seen
		if !d.Move {
			break
		}
		in.Steps = append(in.Steps, s.PlanStep(e, d.To, d.Wander))
	}
	return in
}

// PlanStep plant einen Schritt von e nach to und setzt e in der Planung
// schon dorthin: Ein zerstörtes Objekt ist so für die folgenden Gegner schon
// zerstört. Gedacht für eigene Taktiken (siehe Tactic).
func (s *State) PlanStep(e *Enemy, to hex.Coord, wander bool) Step {
	razed := s.moveEnemy(e, to) != nil
	return Step{To: to, Wander: wander, Razes: razed}
}

// snapshot merkt sich alles, was moveEnemy verändert, und liefert eine
// Funktion, die es zurücksetzt.
func (s *State) snapshot() func() {
	type savedEnemy struct {
		pos hex.Coord
		def *EnemyDef
		hp  int
	}
	enemies := make([]savedEnemy, len(s.Enemies))
	for i, e := range s.Enemies {
		enemies[i] = savedEnemy{e.Pos, e.Def, e.HP}
	}
	objects := make([]*ObjectDef, len(s.Objects))
	for i, o := range s.Objects {
		objects[i] = o.Def
	}
	return func() {
		for i, e := range s.Enemies {
			e.Pos, e.Def, e.HP = enemies[i].pos, enemies[i].def, enemies[i].hp
		}
		for i, o := range s.Objects {
			o.Def = objects[i]
		}
	}
}

// moveEnemy setzt e auf to. Zerstört er dabei ein Objekt, wird es zu dessen
// razesTo, und er selbst zu seinem becomes mit vollen HP. Rückgabe: was das
// Objekt vorher war, oder nil.
func (s *State) moveEnemy(e *Enemy, to hex.Coord) *ObjectDef {
	e.Pos = to
	o := s.ObjectAt(to)
	if !e.Def.Razes || o == nil || o.Def.RazesTo == "" {
		return nil
	}
	before := o.Def
	o.Def = s.Content.Objects[before.RazesTo]
	if next, ok := s.Content.Enemies[e.Def.Becomes]; ok {
		e.Def, e.HP = next, next.HP
	}
	return before
}

// EndTurn führt den angezeigten Gegnerplan aus, lässt den Sturm vorrücken,
// den Alarm steigen und plant den nächsten Zug.
func (s *State) EndTurn() {
	if s.Result != Running {
		return
	}
	defer s.UpdateVision()
	for _, in := range s.Intents {
		e := in.Enemy
		if !slices.Contains(s.Enemies, e) {
			continue // inzwischen vom Spieler besiegt
		}
		// Sieht er den Spieler noch, merkt er sich die neue Position, sonst
		// die vom Planen.
		if in.Seen {
			seen := s.plannedAt
			if s.SightClear(e.Pos, s.Player) {
				seen = s.Player
			}
			e.LastSeen = &seen
		}
		attack := in.Attack
		for _, st := range in.Steps {
			if st.To == s.Player { // der Spieler steht im Weg
				attack = true
				break
			}
			if !s.Free(st.To) {
				break
			}
			was := e.Def
			if razed := s.moveEnemy(e, st.To); razed != nil {
				msg := fmt.Sprintf("%s zerstört %s!", was.Name, razed.Name)
				if e.Def != was {
					msg += fmt.Sprintf(" Wird zu %s.", e.Def.Name)
				}
				s.say(msg)
			}
			if e.LastSeen != nil && e.Pos == *e.LastSeen {
				// Angekommen und niemand da: Das weiß der ganze Trupp.
				for _, m := range s.SquadOf(e) {
					m.LastSeen = nil
				}
			}
		}
		// Angriff mit voller Kraft (Schaden = eigene HP), aber nur, wenn der
		// Spieler noch daneben steht. Ist er ausgewichen, geht er ins Leere.
		if attack && e.Pos.Dist(s.Player) == 1 {
			dmg := e.HP
			if !s.hurtPlayer(dmg) {
				return
			}
			s.say(fmt.Sprintf("%s greift dich an: -%d HP.", e.Def.Name, dmg))
		}
	}
	s.advanceStorm()
	if s.Result != Running {
		return
	}
	s.Turn++
	s.updateAlarm()
	s.planEnemies()
}
