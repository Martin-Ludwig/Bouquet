package duel

import (
	"slices"

	"bouquet/game"
	"bouquet/lib/hex"
)

// Hero ist ein Held im Duell.
type Hero struct {
	Class    *game.ClassDef
	Pos      hex.Coord
	HP       int
	Shield   bool   // Schild gehoben, bis zu seinem nächsten Zug
	Attacked bool   // hat in diesem Zug schon angegriffen
	Moved    bool   // hat sich in diesem Zug bewegt
	Trail    []Mark // Felder, die er verlassen hat
}

// Mark ist ein Feld der Spur: Left ist, wie viele Zugenden seines Besitzers
// es noch liegen bleibt.
type Mark struct {
	Pos  hex.Coord
	Left int
}

// State ist der veränderliche Teil eines Duells. Die KI spielt viele
// Varianten durch; deshalb ist er klein und lässt sich mit Clone kopieren.
type State struct {
	Heroes [2]Hero
	Turn   int // wer am Zug ist: 0 oder 1
	Round  int // ab 1; eine Runde ist ein Zug von jedem
	AP     int // übrige Aktionspunkte dessen, der am Zug ist
}

// Clone liefert eine unabhängige Kopie.
func (s State) Clone() State {
	for p := range s.Heroes {
		s.Heroes[p].Trail = slices.Clone(s.Heroes[p].Trail)
	}
	return s
}

// ActKind ist die Art einer Aktion.
type ActKind int

const (
	Step   ActKind = iota // ein Feld weit gehen, 1 AP
	Attack                // auf ein Feld schlagen oder schießen, 1 AP, einmal pro Zug
	Shield                // Schild heben: kostet HP und beendet den Zug
)

// Act ist eine Aktion des Spielers am Zug. To ist das Ziel eines Schritts
// oder Angriffs.
type Act struct {
	Kind ActKind
	To   hex.Coord
}

const attackCost = 1 // AP pro Angriff

// ---------- Angriffe ----------

// MinRange und MaxRange sind die kürzeste und weiteste Entfernung, auf die
// eine Klasse angreifen kann: Nahangriff 1 Feld, Fernangriff 2 bis Range.
func MinRange(k *game.ClassDef) int {
	if k.Melee > 0 {
		return 1
	}
	return 2
}

func MaxRange(k *game.ClassDef) int {
	if k.Ranged > 0 {
		return k.Range
	}
	return 1
}

// InLine meldet, ob a und b auf einer geraden Linie liegen (eine der sechs
// Hex-Richtungen). Nur so gehen Angriffe.
func InLine(a, b hex.Coord) bool {
	dq, dr := a.Q-b.Q, a.R-b.R
	return dq == 0 || dr == 0 || dq+dr == 0
}

// CanTarget meldet, ob ein Held der Klasse k von from auf das Feld at
// zielen kann: gerade Linie, passende Entfernung. Hindernisse und Sicht
// spielen keine Rolle.
func (a *Arena) CanTarget(from, at hex.Coord, k *game.ClassDef) bool {
	d := from.Dist(at)
	return a.Passable(at) && d >= MinRange(k) && d <= MaxRange(k) && InLine(from, at)
}

// Targets sind alle Felder, auf die ein Held der Klasse k von from aus
// zielen kann.
func (a *Arena) Targets(from hex.Coord, k *game.ClassDef) []hex.Coord {
	var out []hex.Coord
	for i := range a.Tiles {
		if c := a.Grid.Coord(i); a.CanTarget(from, c, k) {
			out = append(out, c)
		}
	}
	return out
}

// OnTrail meldet, ob c zur Spur des Helden owner gehört.
func OnTrail(s *State, owner int, c hex.Coord) bool {
	return slices.ContainsFunc(s.Heroes[owner].Trail, func(m Mark) bool { return m.Pos == c })
}

// Damage ist der Schaden, den ein Angriff des Spielers p auf das Feld at
// anrichtet; 0, wenn dort kein Gegner steht oder p es nicht treffen kann.
// Höherer Boden und die Spur des Gegners geben Boni; gegen einen Schild
// zählt immer nur ShieldDamage.
func (a *Arena) Damage(s *State, p int, at hex.Coord) int {
	me, foe := s.Heroes[p], s.Heroes[1-p]
	if at != foe.Pos || !a.CanTarget(me.Pos, at, me.Class) {
		return 0
	}
	if foe.Shield {
		return a.Def.ShieldDamage
	}
	dmg := me.Class.Ranged
	if me.Pos.Dist(at) == 1 {
		dmg = me.Class.Melee
	}
	if a.Elevation(me.Pos) > a.Elevation(at) {
		dmg += a.Def.HeightBonus
	}
	if OnTrail(s, 1-p, me.Pos) {
		dmg += a.Def.TrailBonus
	}
	return max(1, dmg)
}

// ---------- Aktionen ----------

// Acts listet, was der Spieler am Zug jetzt tun kann. Ein Angriff steht nur
// drin, wenn er den Gegner trifft (dort, wo s ihn hat).
func (a *Arena) Acts(s *State) []Act {
	me, foe := s.Heroes[s.Turn], s.Heroes[1-s.Turn]
	var acts []Act
	if s.AP >= 1 {
		for _, n := range a.Grid.Neighbors(me.Pos) {
			if a.Passable(n) && n != foe.Pos {
				acts = append(acts, Act{Kind: Step, To: n})
			}
		}
	}
	if !me.Attacked && s.AP >= attackCost && a.Damage(s, s.Turn, foe.Pos) > 0 {
		acts = append(acts, Act{Kind: Attack, To: foe.Pos})
	}
	if a.CanShield(s) {
		acts = append(acts, Act{Kind: Shield})
	}
	return acts
}

// CanShield meldet, ob der Spieler am Zug den Schild heben kann.
func (a *Arena) CanShield(s *State) bool {
	me := s.Heroes[s.Turn]
	return s.AP >= 1 && !me.Shield && me.HP > a.Def.ShieldCost
}

// Apply führt act für den Spieler am Zug aus, ohne zu prüfen, ob sie
// erlaubt ist. Rückgabe: der angerichtete Schaden (bei einem Angriff).
func (a *Arena) Apply(s *State, act Act) (dmg int) {
	me := &s.Heroes[s.Turn]
	switch act.Kind {
	case Step:
		me.Trail = append(slices.DeleteFunc(slices.Clone(me.Trail), func(m Mark) bool { return m.Pos == me.Pos }),
			Mark{Pos: me.Pos, Left: a.Def.Trail})
		me.Pos, me.Moved = act.To, true
		s.AP--
	case Attack:
		dmg = a.Damage(s, s.Turn, act.To)
		s.Heroes[1-s.Turn].HP -= dmg
		me.Attacked = true
		s.AP -= attackCost
	case Shield:
		me.Shield = true
		me.HP -= a.Def.ShieldCost
		s.AP = 0
	}
	return dmg
}

// CanStep meldet, ob der Spieler am Zug noch einen Schritt machen kann.
func (a *Arena) CanStep(s *State) bool {
	return slices.ContainsFunc(a.Acts(s), func(act Act) bool { return act.Kind == Step })
}

// MayEnd meldet, ob der Zug enden darf: Jeder muss sich bewegen, außer er
// hat den Schild gehoben oder kann nicht mehr gehen.
func (a *Arena) MayEnd(s *State) bool {
	me := s.Heroes[s.Turn]
	return me.Moved || me.Shield || !a.CanStep(s)
}

// EndTurn beendet den Zug: Die Spur altert, wer im Sturm steht, verliert
// HP, und der andere ist dran. Rückgabe: der Sturmschaden.
func (a *Arena) EndTurn(s *State) (storm int) {
	me := &s.Heroes[s.Turn]
	me.Trail = ageTrail(me.Trail)
	if a.InStorm(me.Pos, s.Round) {
		storm = a.Def.Storm.Damage
		me.HP -= storm
	}
	s.Turn = 1 - s.Turn
	if s.Turn == 0 {
		s.Round++
	}
	next := &s.Heroes[s.Turn]
	next.Shield, next.Attacked, next.Moved = false, false, false
	s.AP = next.Class.AP
	if s.Round == 1 && s.Turn == 1 && a.Def.FirstAP > 0 {
		s.AP = min(s.AP, a.Def.FirstAP) // Ausgleich für den Nachzug
	}
	return storm
}

func ageTrail(trail []Mark) []Mark {
	var out []Mark
	for _, m := range trail {
		if m.Left > 1 {
			out = append(out, Mark{Pos: m.Pos, Left: m.Left - 1})
		}
	}
	return out
}
