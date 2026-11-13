package game

import (
	"fmt"
	"slices"

	"bouquet/lib/hex"
)

// Click ist die eine Aktion des Spielers pro Zug: Ein Klick auf einen Gegner
// greift an, ein Klick auf ein anderes Feld läuft dorthin. War die Aktion
// gültig, sind danach die Gegner dran.
func (s *State) Click(c hex.Coord) {
	if s.Result != Running || !s.Grid.Contains(c) {
		return
	}
	acted := false
	if e := s.EnemyAt(c); e != nil {
		acted = s.AttackEnemy(e)
		if !acted {
			s.say("Zu weit weg: Angreifen geht nur auf ein Nachbarfeld.")
		}
	} else {
		acted = s.MovePlayer(c)
	}
	if acted {
		s.EndTurn()
	}
}

// MovePlayer bewegt den Spieler nach to, wenn das Feld in Laufweite liegt.
// Liegt dort ein Objekt, wirkt es; auf dem Zielfeld ist der Einsatz gewonnen.
// Rückgabe: ob er sich bewegt hat.
func (s *State) MovePlayer(to hex.Coord) bool {
	i := s.Grid.Index(to)
	if i < 0 || s.Reach()[i] < 1 {
		return false
	}
	s.Player = to
	if o := s.ObjectAt(to); o != nil {
		s.enter(o)
	}
	if s.Result == Running && to == s.Goal {
		s.Result = Won
		s.say("Extraktionspunkt erreicht.")
	}
	s.UpdateVision()
	return true
}

// AttackEnemy greift einen Gegner auf einem Nachbarfeld an, mit voller
// Kraft: Schaden = eigene HP. Ein besiegter Gegner senkt den Alarm.
// Rückgabe: ob der Angriff stattgefunden hat.
func (s *State) AttackEnemy(e *Enemy) bool {
	if e.Pos.Dist(s.Player) != 1 {
		return false
	}
	e.HP -= s.PlayerHP
	if e.HP > 0 {
		s.say(fmt.Sprintf("Treffer: %s hat noch %d HP.", e.Def.Name, e.HP))
		return true
	}
	s.Enemies = slices.DeleteFunc(s.Enemies, func(x *Enemy) bool { return x == e })
	s.UpdateVision()
	msg := fmt.Sprintf("%s besiegt.", e.Def.Name)
	if calmed := s.calmAlarm(); calmed > 0 {
		msg += fmt.Sprintf(" Alarm -%d.", calmed)
	}
	s.say(msg)
	return true
}
