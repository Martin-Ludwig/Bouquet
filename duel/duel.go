package duel

import (
	"fmt"
	"math/rand/v2"
	"sort"

	"bouquet/game"
	"bouquet/lib/hex"
)

// Duel ist ein laufendes Duell, so wie die Oberfläche es bedient: Arena und
// Stand, dazu was jeder Spieler weiß, die Meldungen und das Ergebnis.
type Duel struct {
	*Arena
	State
	AI      [2]bool       // diese Spieler steuert die KI
	Belief  [2]*hex.Coord // wo Spieler p den Gegner zuletzt gesehen hat; nil: weiß es nicht
	Over    bool
	Winner  int // nach dem Ende: 0 oder 1, -1 bei Unentschieden
	Message string
	Log     []string // die neueste Meldung zuerst

	rng      *rand.Rand
	aiBudget int // so viele Aktionen darf die KI in diesem Zug noch
}

const logLines = 6

// New beginnt ein Duell in der Arena arenaID. Leer heißt: eine zufällige.
// classes sind die Klassen der beiden Helden, ai sagt, wen die KI steuert.
func New(c *game.Content, arenaID string, classes [2]*game.ClassDef, ai [2]bool, seed uint64) (*Duel, error) {
	rng := rand.New(rand.NewPCG(seed, seed^0x51ed270b27b1f00d))
	if arenaID == "" {
		ids := make([]string, 0, len(c.Arenas))
		for id := range c.Arenas {
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("keine Arena in den Inhalten")
		}
		sort.Strings(ids)
		arenaID = ids[rng.IntN(len(ids))]
	}
	a, err := NewArena(c, arenaID, rng)
	if err != nil {
		return nil, err
	}
	d := &Duel{Arena: a, AI: ai, Winner: -1, rng: rng}
	for p, k := range classes {
		d.Heroes[p] = Hero{Class: k, Pos: a.Start[p], HP: k.HP}
	}
	d.Turn, d.Round, d.AP = 0, 1, classes[0].AP
	for p := range d.Belief {
		start := a.Start[1-p]
		d.Belief[p] = &start // den Start des Gegners kennt jeder
	}
	d.say(d.Name(0) + " beginnt. Jeder Zug: mindestens 1 Feld gehen.")
	d.startTurn()
	return d, nil
}

// RandomClass wählt eine Klasse zufällig.
func RandomClass(c *game.Content, rng *rand.Rand) *game.ClassDef {
	ids := make([]string, 0, len(c.Classes))
	for id := range c.Classes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return c.Classes[ids[rng.IntN(len(ids))]]
}

// Name ist die Anzeige für Spieler p, z. B. "Spieler 1" oder "KI".
func (d *Duel) Name(p int) string {
	if d.AI[p] && !d.AI[1-p] {
		return "KI"
	}
	return fmt.Sprintf("Spieler %d", p+1)
}

// Label ist der Name mit Klasse.
func (d *Duel) Label(p int) string { return d.Name(p) + " (" + d.Heroes[p].Class.Name + ")" }

// Viewer ist der Spieler, aus dessen Sicht die Karte gezeigt wird: der
// einzige Mensch, sonst der am Zug.
func (d *Duel) Viewer() int {
	if d.AI[d.Turn] && !d.AI[1-d.Turn] {
		return 1 - d.Turn
	}
	return d.Turn
}

// HumanTurn meldet, ob ein Mensch am Zug ist und das Duell läuft.
func (d *Duel) HumanTurn() bool { return !d.Over && !d.AI[d.Turn] }

func (d *Duel) say(msg string) {
	d.Message = msg
	d.Log = append([]string{msg}, d.Log[:min(len(d.Log), logLines-1)]...)
}

// updateBeliefs hält fest, was die Spieler über den Gegner wissen: Sehen
// sie ihn, kennen sie sein Feld; ist die vermutete Stelle in Sicht und leer,
// ist die Vermutung hinfällig.
func (d *Duel) updateBeliefs() {
	for p := range d.Belief {
		me := d.Heroes[p].Pos
		switch {
		case d.Sees(&d.State, p):
			pos := d.Heroes[1-p].Pos
			d.Belief[p] = &pos
		case d.Belief[p] != nil && d.TileSeen(me, *d.Belief[p]):
			d.Belief[p] = nil
		}
	}
}

// checkOver beendet das Duell, wenn einer keine HP mehr hat.
func (d *Duel) checkOver() bool {
	a, b := d.Heroes[0].HP, d.Heroes[1].HP
	if a > 0 && b > 0 {
		return false
	}
	d.Over = true
	switch {
	case a <= 0 && b <= 0:
		d.say("Unentschieden.")
	case a <= 0:
		d.Winner = 1
		d.say(d.Name(1) + " gewinnt!")
	default:
		d.Winner = 0
		d.say(d.Name(0) + " gewinnt!")
	}
	return true
}

// ---------- Aktionen des Menschen ----------

// knownState ist der Stand, wie der Spieler am Zug ihn kennt: Sieht er den
// Gegner nicht, steht der für ihn nirgends.
func (d *Duel) knownState() State {
	s := d.State.Clone()
	if !d.Sees(&d.State, d.Turn) {
		s.Heroes[1-d.Turn].Pos = hex.Coord{Q: -1000, R: -1000}
	}
	return s
}

// Reach sagt, welche Felder der Spieler am Zug mit seinen AP erreicht, und
// was der Weg kostet. Einen Gegner, den er nicht sieht, kennt er nicht: Sein
// Feld erscheint begehbar.
func (d *Duel) Reach() map[hex.Coord]int {
	if !d.HumanTurn() {
		return nil
	}
	field, _ := d.paths()
	out := map[hex.Coord]int{}
	for i, steps := range field {
		if steps > 0 {
			out[d.Grid.Coord(i)] = steps
		}
	}
	return out
}

// paths ist die Wegsuche vom Spieler am Zug aus, so weit seine AP reichen:
// Schritte pro Feld und das Feld davor.
func (d *Duel) paths() ([]int, map[hex.Coord]hex.Coord) {
	s := d.knownState()
	from := s.Heroes[s.Turn].Pos
	free := func(c hex.Coord) bool { return d.Passable(c) && c != s.Heroes[1-s.Turn].Pos }
	field := d.Grid.DistanceField([]hex.Coord{from}, free, s.AP)
	prev := map[hex.Coord]hex.Coord{}
	for i, steps := range field {
		c := d.Grid.Coord(i)
		for _, n := range d.Grid.Neighbors(c) {
			if steps > 0 && field[d.Grid.Index(n)] == steps-1 {
				prev[c] = n
				break
			}
		}
	}
	return field, prev
}

// Move geht Feld für Feld nach to. Steht der unsichtbare Gegner im Weg,
// bleibt der Held davor stehen. Rückgabe: ob er gegangen ist.
func (d *Duel) Move(to hex.Coord) bool {
	field, prev := d.paths()
	i := d.Grid.Index(to)
	if !d.HumanTurn() || i < 0 || field[i] < 1 {
		d.say("Dorthin reichen deine AP nicht (oder es ist gesperrt).")
		return false
	}
	var path []hex.Coord
	for c := to; c != d.Heroes[d.Turn].Pos; c = prev[c] {
		path = append([]hex.Coord{c}, path...)
	}
	for _, c := range path {
		if !d.step(c) {
			break
		}
	}
	return true
}

// step geht ein Feld weit. Steht dort der Gegner, stößt der Held auf ihn
// und bleibt stehen. Rückgabe: ob er gegangen ist.
func (d *Duel) step(to hex.Coord) bool {
	if to == d.Heroes[1-d.Turn].Pos {
		d.say(d.Name(d.Turn) + " stößt auf den Gegner!")
		d.updateBeliefs()
		return false
	}
	d.Apply(&d.State, Act{Kind: Step, To: to})
	d.updateBeliefs()
	return true
}

// AttackStatus sagt, ob der Spieler am Zug den Gegner jetzt treffen kann,
// und sonst warum nicht. Blocked: Er kann in diesem Zug gar nicht mehr
// angreifen, auch nicht blind.
type AttackStatus struct {
	OK      bool
	Damage  int
	Reason  string
	Blocked bool
}

// Status ist der AttackStatus des Spielers am Zug.
func (d *Duel) Status() AttackStatus {
	s, p := &d.State, d.Turn
	me, foe := s.Heroes[p], s.Heroes[1-p]
	dist := me.Pos.Dist(foe.Pos)
	switch {
	case me.Attacked:
		return AttackStatus{Reason: "schon angegriffen", Blocked: true}
	case s.AP < attackCost:
		return AttackStatus{Reason: "keine AP", Blocked: true}
	case !d.Sees(s, p):
		return AttackStatus{Reason: "Gegner nicht sichtbar"}
	case dist > MaxRange(me.Class):
		return AttackStatus{Reason: fmt.Sprintf("zu weit (Reichweite %d)", MaxRange(me.Class))}
	case dist < MinRange(me.Class):
		return AttackStatus{Reason: "zu nah (nur Fernangriff)"}
	case !InLine(me.Pos, foe.Pos):
		return AttackStatus{Reason: "nicht in gerader Linie"}
	}
	return AttackStatus{OK: true, Damage: d.Damage(s, p, foe.Pos)}
}

// AttackAt schlägt auf das Feld at, ob dort jemand steht oder nicht. Steht
// niemand dort, ist der Angriff trotzdem verbraucht. Rückgabe: ob
// angegriffen wurde.
func (d *Duel) AttackAt(at hex.Coord) bool {
	p := d.Turn
	if d.Over || d.Heroes[p].Attacked || d.AP < attackCost || !d.CanTarget(d.Heroes[p].Pos, at, d.Heroes[p].Class) {
		return false
	}
	melee := d.Heroes[p].Pos.Dist(at) == 1
	dmg := d.Apply(&d.State, Act{Kind: Attack, To: at})
	switch {
	case dmg == 0:
		d.say(d.Name(p) + " schlägt ins Leere.")
		if d.Belief[p] != nil && *d.Belief[p] == at {
			d.Belief[p] = nil // die Vermutung war falsch
		}
	case melee:
		d.say(fmt.Sprintf("%s schlägt zu: -%d HP.", d.Name(p), dmg))
	default:
		d.say(fmt.Sprintf("%s trifft aus der Ferne: -%d HP.", d.Name(p), dmg))
	}
	d.updateBeliefs()
	d.checkOver()
	return true
}

// RaiseShield hebt den Schild und beendet damit den Zug.
func (d *Duel) RaiseShield() bool {
	if d.Over || !d.CanShield(&d.State) {
		return false
	}
	d.Apply(&d.State, Act{Kind: Shield})
	d.say(fmt.Sprintf("%s hebt den Schild (-%d HP).", d.Name(d.Turn), d.Def.ShieldCost))
	if !d.checkOver() {
		d.EndTurn()
	}
	return true
}

// EndTurn beendet den Zug, wenn er enden darf. Rückgabe: ob er beendet ist.
func (d *Duel) EndTurn() bool {
	if d.Over {
		return false
	}
	if !d.MayEnd(&d.State) {
		d.say("Du musst dich bewegen, bevor du den Zug beendest (oder den Schild heben).")
		return false
	}
	p := d.Turn
	if storm := d.Arena.EndTurn(&d.State); storm > 0 {
		d.say(fmt.Sprintf("%s steht im Sturm: -%d HP.", d.Name(p), storm))
	}
	d.updateBeliefs()
	if d.checkOver() {
		return true
	}
	if st := d.Def.Storm; st != nil && d.Turn == 0 && d.Round == st.Delay+2 {
		d.say("Der Sturm zieht auf.")
	}
	d.startTurn()
	return true
}

func (d *Duel) startTurn() {
	d.aiBudget = 8
	if !d.AI[0] && !d.AI[1] {
		d.Log = nil // zu zweit an einem Gerät: Was der andere getan hat, bleibt geheim
	}
	if !d.AI[d.Turn] {
		d.say(d.Label(d.Turn) + " ist am Zug.")
	}
}

// ---------- KI ----------

// AIStep lässt die KI eine Aktion machen. Hat sie nichts mehr vor, beendet
// sie den Zug. Rückgabe: ob sie noch am Zug ist.
func (d *Duel) AIStep() bool {
	if d.Over || !d.AI[d.Turn] {
		return false
	}
	act, ok := d.aiNext()
	if !ok {
		d.EndTurn()
		return false
	}
	switch act.Kind {
	case Step:
		d.step(act.To)
	case Attack:
		d.AttackAt(act.To)
	case Shield:
		d.RaiseShield()
		return false
	}
	return !d.Over && d.AI[d.Turn]
}

// aiNext ist die nächste Aktion der KI. Sie weiß nur, was sie sieht: Sieht
// sie den Gegner nicht, rechnet sie mit der letzten bekannten Stelle, sonst
// mit der Mitte, wohin der Sturm alle treibt. Blind schlägt sie nur auf eine
// Vermutung, die sie nicht selbst einsehen kann.
func (d *Duel) aiNext() (Act, bool) {
	if d.aiBudget <= 0 || d.AP <= 0 {
		return Act{}, false
	}
	d.aiBudget--
	p := d.Turn
	s := d.State.Clone()
	if !d.Sees(&d.State, p) {
		guess := d.Center
		if d.Belief[p] != nil {
			guess = *d.Belief[p]
		}
		if guess == s.Heroes[p].Pos {
			guess = d.Grid.Neighbors(guess)[0]
		}
		s.Heroes[1-p].Pos = guess
		if d.Belief[p] == nil || d.TileSeen(s.Heroes[p].Pos, guess) {
			s.Heroes[p].Attacked = true // nicht ins Blaue schlagen
		}
	}
	plan := d.Plan(s, d.rng)
	if len(plan) == 0 {
		return Act{}, false
	}
	return plan[0], true
}
