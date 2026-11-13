package main

import (
	"fmt"
	"math/rand/v2"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/duel"
	"bouquet/game"
	"bouquet/lib/hex"
	"bouquet/view"
)

const (
	aiDelay  = 0.45 // Sekunden zwischen zwei Aktionen der KI, damit man folgen kann
	endDelay = 0.35 // Pause, bevor ein Zug ohne AP von selbst endet
)

// duelGame ist der Duell-Bildschirm: erst die Klassenwahl, dann das Duell.
// Zu zweit an einem Gerät verdeckt vor jedem Zug ein Übergabebildschirm die
// Karte.
type duelGame struct {
	ai      [2]bool
	classes [2]*game.ClassDef
	picking int // wer gerade seine Klasse wählt
	rng     *rand.Rand

	d        *duel.Duel
	handover bool    // Karte verdeckt, bis der nächste Spieler bereit ist
	aiming   bool    // der Spieler zielt auf ein Feld
	rules    bool    // Regelfenster offen
	wait     float64 // Zeitpunkt, ab dem es weitergeht (KI, Zugende)
	turn     int     // wer zuletzt dran war, um Zugwechsel zu bemerken
}

func newDuelGame(ai [2]bool) *duelGame {
	return &duelGame{ai: ai, rng: rand.New(rand.NewPCG(seed(), 1))}
}

// hotseat: zwei Menschen an einem Gerät.
func (g *duelGame) hotseat() bool { return !g.ai[0] && !g.ai[1] }

func (g *duelGame) start(c *game.Content) error {
	d, err := duel.New(c, "", g.classes, g.ai, seed())
	if err != nil {
		return err
	}
	g.d, g.turn, g.aiming, g.wait = d, d.Turn, false, rl.GetTime()+aiDelay
	g.handover = g.hotseat()
	return nil
}

func (g *duelGame) update(a *app, w, h float64) error {
	W, H := int32(w), int32(h)
	if rl.IsKeyPressed(rl.KeyEscape) {
		a.screen = menuScreen
		return nil
	}
	if g.d == nil {
		return g.pickClass(a.content, W, H)
	}
	d := g.d
	if g.handover {
		if view.Menu(d.Label(d.Turn)+" ist am Zug.", []view.MenuItem{{Label: "Bereit", Note: "Gerät übergeben, dann klicken"}}, W, H) == 0 {
			g.handover = false
		}
		return nil
	}

	layout := view.DuelLayout(d, w, h)
	mouse := rl.GetMousePosition()
	hovered := layout.At(float64(mouse.X), float64(mouse.Y))
	a.board.DrawDuel(d, layout, hovered, g.aiming)
	cmd := view.DrawDuelHud(d, g.aiming, W, H)
	if g.rules {
		if view.Panel("Regeln", view.DuelRules(d), W, H) {
			g.rules = false
		}
		return nil
	}

	switch cmd {
	case view.NewCommand:
		return g.start(a.content)
	case view.MenuCommand:
		a.screen = menuScreen
		return nil
	case view.RulesCommand:
		g.rules = true
		return nil
	}
	now := rl.GetTime()
	switch {
	case d.Over:
	case !d.HumanTurn():
		if now >= g.wait {
			d.AIStep()
			g.wait = now + aiDelay
		}
	case cmd != view.NoCommand:
		g.command(cmd)
		g.wait = now + endDelay
	case rl.IsMouseButtonPressed(rl.MouseButtonLeft) && d.OnMap(hovered):
		g.click(hovered)
		g.wait = now + endDelay
	case d.AP <= 0 && now >= g.wait:
		d.EndTurn() // keine AP mehr: der Zug endet von selbst
	}
	if d.Turn != g.turn {
		g.turn, g.aiming = d.Turn, false
		g.handover = g.hotseat() && !d.Over
	}
	return nil
}

// pickClass zeigt die Klassenwahl für den nächsten Menschen. Die KI nimmt
// eine zufällige Klasse.
func (g *duelGame) pickClass(c *game.Content, w, h int32) error {
	for g.picking < 2 && g.ai[g.picking] {
		g.classes[g.picking] = duel.RandomClass(c, g.rng)
		g.picking++
	}
	if g.picking == 2 {
		return g.start(c)
	}
	ids := sortedClassIDs(c)
	items := make([]view.MenuItem, 0, len(ids)+1)
	for _, id := range ids {
		k := c.Classes[id]
		items = append(items, view.MenuItem{Label: k.Name, Note: classInfo(k)})
	}
	items = append(items, view.MenuItem{Label: "Zufall"})
	title := "Deine Klasse"
	if g.hotseat() {
		title = []string{"Spieler 1: Klasse", "Spieler 2: Klasse"}[g.picking]
	}
	switch i := view.Menu(title, items, w, h); {
	case i < 0:
		return nil
	case i == len(ids):
		g.classes[g.picking] = duel.RandomClass(c, g.rng)
	default:
		g.classes[g.picking] = c.Classes[ids[i]]
	}
	g.picking++
	return nil
}

// command führt einen Knopf der Fußzeile aus.
func (g *duelGame) command(cmd view.DuelCommand) {
	d := g.d
	switch cmd {
	case view.AttackCommand:
		switch st := d.Status(); {
		case g.aiming:
			g.aiming = false
		case st.OK:
			d.AttackAt(d.Heroes[1-d.Turn].Pos)
		case !st.Blocked:
			g.aiming = true
			d.Message = "Tippe ein rotes Feld: Dorthin wird geschlagen, auch ins Dunkle."
		}
	case view.ShieldCommand:
		d.RaiseShield()
	case view.EndCommand:
		d.EndTurn()
	}
}

// click: beim Zielen auf das Feld schlagen, auf den sichtbaren Gegner
// angreifen, sonst dorthin gehen.
func (g *duelGame) click(c hex.Coord) {
	d := g.d
	me := d.Heroes[d.Turn]
	switch {
	case g.aiming:
		g.aiming = false
		if !d.AttackAt(c) {
			d.Message = "Zielen abgebrochen."
		}
	case c == d.Heroes[1-d.Turn].Pos && d.Sees(&d.State, d.Turn):
		if st := d.Status(); st.OK {
			d.AttackAt(c)
		} else {
			d.Message = "Kein Treffer möglich: " + st.Reason + "."
		}
	case c != me.Pos:
		d.Move(c)
	}
}

func sortedClassIDs(c *game.Content) []string {
	ids := make([]string, 0, len(c.Classes))
	for id := range c.Classes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// classInfo beschreibt eine Klasse in einer Zeile, für die Auswahl.
func classInfo(k *game.ClassDef) string {
	melee, ranged := "kein Nahangriff", "kein Fernangriff"
	if k.Melee > 0 {
		melee = fmt.Sprintf("Nah %d", k.Melee)
	}
	if k.Ranged > 0 {
		ranged = fmt.Sprintf("Fern %d (2 Felder)", k.Ranged)
		if k.Range > 2 {
			ranged = fmt.Sprintf("Fern %d (2-%d Felder)", k.Ranged, k.Range)
		}
	}
	return fmt.Sprintf("%d HP, %d AP, %s, %s", k.HP, k.AP, melee, ranged)
}
