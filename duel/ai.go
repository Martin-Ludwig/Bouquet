package duel

import (
	"math/rand/v2"
	"strconv"
)

// Gewichte der KI. Sie bewertet den Stand am Ende ihres Zugs.
const (
	aiLife   = 10.0 // pro HP Vorsprung
	aiThreat = 3.0  // pro HP, die der Gegner im nächsten Zug anrichten könnte
	aiChance = 5.0  // pro HP, die sie selbst danach anrichten könnte
	aiHeight = 0.8  // pro Höhenstufe unter den Füßen
	aiTower  = 1.5  // auf dem Turm
	aiStorm  = 2.0  // wenn der Sturm sie in der nächsten Runde erreicht
	aiCenter = 0.1  // pro Feld Abstand zur Mitte, wohin der Sturm alle treibt
	aiNoise  = 0.4  // Zufall, damit sie nicht immer gleich spielt
	aiWin    = 1000.0
)

// Plan sucht für den Spieler am Zug die beste Folge von Aktionen bis zum
// Zugende. s ist der Stand, wie die KI ihn kennt: Der Gegner steht dort, wo
// sie ihn vermutet. Der Zug ist höchstens ein paar AP lang, also probiert
// sie einfach alle Folgen durch.
func (a *Arena) Plan(s State, rng *rand.Rand) []Act {
	p := s.Turn
	cache := map[string]float64{}
	evaluate := func(s State) float64 {
		end := s.Clone()
		end.Heroes[p].Trail = ageTrail(end.Heroes[p].Trail)
		if a.InStorm(end.Heroes[p].Pos, end.Round) {
			end.Heroes[p].HP -= a.Def.Storm.Damage
		}
		me, foe := end.Heroes[p], end.Heroes[1-p]
		switch {
		case foe.HP <= 0:
			return aiWin
		case me.HP <= 0:
			return -aiWin
		}
		k := key(&end)
		v, ok := cache[k]
		if !ok {
			threat := a.bestDamage(nextTurn(end, 1-p, true))
			chance := a.bestDamage(nextTurn(end, p, false))
			v = float64(me.HP-foe.HP)*aiLife - float64(threat)*aiThreat + float64(chance)*aiChance +
				float64(a.Elevation(me.Pos))*aiHeight - float64(me.Pos.Dist(a.Center))*aiCenter
			if a.IsTower(me.Pos) {
				v += aiTower
			}
			if a.InStorm(me.Pos, end.Round+1) {
				v -= aiStorm
			}
			cache[k] = v
		}
		return v + rng.Float64()*aiNoise
	}

	best, bestScore := []Act(nil), -2*aiWin
	seen := map[string]bool{}
	var seq []Act
	var search func(s State)
	search = func(s State) {
		k := key(&s)
		if seen[k] {
			return
		}
		seen[k] = true
		if a.MayEnd(&s) {
			if v := evaluate(s); v > bestScore {
				best, bestScore = append([]Act(nil), seq...), v
			}
		}
		if s.AP <= 0 {
			return
		}
		for _, act := range a.Acts(&s) {
			next := s.Clone()
			a.Apply(&next, act)
			seq = append(seq, act)
			search(next)
			seq = seq[:len(seq)-1]
		}
	}
	search(s)
	return best
}

// nextTurn ist der Stand zu Beginn des nächsten Zugs von p, mit vollen AP.
// keepShield: Der Schild des anderen wirkt dann noch (er gilt bis zu dessen
// nächstem Zug).
func nextTurn(s State, p int, keepShield bool) State {
	s = s.Clone()
	s.Turn, s.AP = p, s.Heroes[p].Class.AP
	for q := range s.Heroes {
		s.Heroes[q].Attacked, s.Heroes[q].Moved = false, false
		if q == p || !keepShield {
			s.Heroes[q].Shield = false
		}
	}
	return s
}

// bestDamage ist der größte Schaden, den der Spieler am Zug in diesem Zug
// anrichten kann.
func (a *Arena) bestDamage(s State) int {
	p := s.Turn
	start, best := s.Heroes[1-p].HP, 0
	seen := map[string]bool{}
	var search func(s State)
	search = func(s State) {
		best = max(best, start-s.Heroes[1-p].HP)
		if s.AP <= 0 || s.Heroes[p].Attacked {
			return // ein Angriff pro Zug: danach kommt kein Schaden mehr dazu
		}
		me := s.Heroes[p]
		k := strconv.Itoa(me.Pos.Q) + "," + strconv.Itoa(me.Pos.R) + "," + strconv.Itoa(s.AP) + "," + strconv.FormatBool(me.Shield)
		if seen[k] {
			return
		}
		seen[k] = true
		for _, act := range a.Acts(&s) {
			if act.Kind == Shield {
				continue // bringt keinen Schaden
			}
			next := s.Clone()
			a.Apply(&next, act)
			search(next)
		}
	}
	search(s)
	return best
}

// key beschreibt einen Stand eindeutig, zum Wiedererkennen in der Suche.
func key(s *State) string {
	b := make([]byte, 0, 96)
	num := func(v int) { b = strconv.AppendInt(b, int64(v), 10); b = append(b, ',') }
	flag := func(v bool) {
		if v {
			b = append(b, '1')
		} else {
			b = append(b, '0')
		}
	}
	num(s.AP)
	for _, h := range s.Heroes {
		num(h.Pos.Q)
		num(h.Pos.R)
		num(h.HP)
		flag(h.Shield)
		flag(h.Attacked)
		flag(h.Moved)
		for _, m := range h.Trail {
			num(m.Pos.Q)
			num(m.Pos.R)
			num(m.Left)
		}
		b = append(b, '|')
	}
	return string(b)
}
