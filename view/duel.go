package view

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/duel"
	"bouquet/lib/draw"
	"bouquet/lib/hex"
)

// DuelFoot ist die Höhe der Fußzeile im Duell: Knöpfe, Meldung, Protokoll.
const DuelFoot = 124.0

var (
	heroColors = [2]rl.Color{player, rival}
	trailGold  = goal
	attackTint = rl.NewColor(200, 50, 40, 255)
	fog        = rl.NewColor(40, 30, 20, 110) // im Duell: außer Sicht
)

// DuelLayout passt die Arena zwischen Kopf- und Fußzeile in ein Fenster der
// Größe w x h ein.
func DuelLayout(d *duel.Duel, w, h float64) hex.Layout {
	return draw.Fit(d.Grid.W, d.Grid.H, 0, HudHeight, w, h-HudHeight-DuelFoot)
}

// DrawDuel zeichnet ein Duell aus der Sicht von d.Viewer(): Was der nicht
// sieht, liegt im Dunkeln, und den Gegner zeigt nur ein "?" an der Stelle,
// wo er ihn zuletzt gesehen hat. Nach dem Ende ist alles offen. aiming: Der
// Spieler zielt gerade. hovered ist das Feld unter der Maus.
func (b *Board) DrawDuel(d *duel.Duel, l hex.Layout, hovered hex.Coord, aiming bool) {
	v := d.Viewer()
	me, foe := d.Heroes[v], d.Heroes[1-v]
	seen := func(c hex.Coord) bool { return d.Over || d.TileSeen(me.Pos, c) }
	foeVisible := d.Over || d.HeroSeen(me.Pos, foe.Pos)
	human := d.HumanTurn()
	var reach map[hex.Coord]int
	if human && !aiming {
		reach = d.Reach()
	}
	height := func(c hex.Coord) int {
		if !d.OnMap(c) {
			return -1
		}
		return d.HeightAt(c)
	}

	for i, t := range d.Tiles {
		c := d.Grid.Coord(i)
		if t == nil {
			continue
		}
		draw.Hex(l, c, 1, b.colors[t.Fill])
		drawRelief(l, c, height)
		if t.Hatch != nil {
			draw.Hatch(l, c, t.Hatch.Lines, t.Hatch.Width, b.colors[t.Hatch.Color])
		}
		if d.IsTower(c) {
			draw.Square(l, c, 0.3, rl.NewColor(160, 130, 80, 255), Ink)
			draw.Triangle(l, c, 0.22, Ink)
		}
		if st := d.Def.Storm; st != nil {
			col := b.colors[st.Color]
			switch {
			case d.InStorm(c, d.Round):
				draw.Hex(l, c, 1, rl.Fade(col, 0.6))
				draw.Hatch(l, c, 4, 1.5, rl.Fade(white, 0.55))
			case d.InStorm(c, d.Round+1):
				draw.Hex(l, c, 1, rl.Fade(col, 0.25))
				draw.Hatch(l, c, 4, 1.5, rl.Fade(white, 0.3))
			}
		}
		draw.Outline(l, c, 1, line)
	}

	// Spuren: die eigene immer, die des Gegners nur, wo man hinsieht. Gold
	// umrandet: Wer von dort angreift, macht mehr Schaden.
	for p, h := range d.Heroes {
		for _, m := range h.Trail {
			if p != v && !seen(m.Pos) {
				continue
			}
			draw.Ring(l, m.Pos, 0.2, 0.12*l.Size*float64(m.Left), rl.Fade(heroColors[p], 0.25+0.12*float32(m.Left)))
			if p != v && !d.Over {
				draw.Outline(l, m.Pos, 2.5, trailGold)
			}
		}
	}

	for c := range reach {
		draw.Hex(l, c, 1, reachTint)
	}
	// Angriffsfelder (rot): gerade Linien, unabhängig von der Sicht. Beim
	// Zielen kräftig und umrandet.
	if human && !me.Attacked && d.AP >= 1 {
		for _, c := range d.Targets(me.Pos, me.Class) {
			alpha := 0.18
			if aiming {
				alpha = 0.4
				draw.Outline(l, c, 2, attackTint)
			}
			draw.Hex(l, c, 0.9, rl.Fade(attackTint, float32(alpha)))
		}
		// Rote Ringe: Von diesem Laufziel aus träfe man den sichtbaren Gegner.
		if foeVisible && !aiming {
			for c, cost := range reach {
				if d.AP-cost >= 1 && d.CanTarget(c, foe.Pos, me.Class) {
					draw.Ring(l, c, 0.5, 2, attackTint)
				}
			}
		}
	}

	for i, t := range d.Tiles {
		if c := d.Grid.Coord(i); t != nil && !seen(c) {
			draw.Hex(l, c, 1, fog)
		}
	}
	for c, cost := range reach {
		draw.Label(l, c, fmt.Sprint(cost), 0.45, rl.Fade(Ink, 0.85))
	}

	for p, h := range d.Heroes {
		if p != v && !foeVisible {
			continue
		}
		if h.Shield {
			draw.Ring(l, h.Pos, 0.78, 3, rl.NewColor(80, 140, 200, 255))
		}
		if !d.Over && p == d.Turn {
			draw.Ring(l, h.Pos, 0.68, 3, goal)
		}
		draw.Token(l, h.Pos, 0.58, heroColors[p], fmt.Sprint(max(0, h.HP)), white)
	}
	if !foeVisible && d.Belief[v] != nil {
		draw.Label(l, *d.Belief[v], "?", 0.8, goal)
	}
	// Vorschau: So viel Schaden macht ein Angriff jetzt.
	if st := d.Status(); human && foeVisible && st.OK {
		draw.Outline(l, foe.Pos, 3, threat)
		x, y := l.Center(foe.Pos)
		label := fmt.Sprintf("-%d", st.Damage)
		px := int32(l.Size * 0.55)
		rl.DrawText(label, int32(x)-rl.MeasureText(label, px)/2, int32(y-l.Size*1.3), px, threat)
	}
	if d.OnMap(hovered) {
		draw.Outline(l, hovered, 3, hover)
	}
}

// DuelCommand ist ein Knopf der Duell-Fußzeile.
type DuelCommand int

const (
	NoCommand DuelCommand = iota
	AttackCommand
	ShieldCommand
	EndCommand
	NewCommand
	MenuCommand
	RulesCommand
)

// DrawDuelHud zeichnet Kopf- und Fußzeile eines Duells in ein Fenster der
// Größe w x h und liefert den Knopf, der angeklickt wurde.
func DrawDuelHud(d *duel.Duel, aiming bool, w, h int32) DuelCommand {
	v := d.Viewer()
	foeHP := "?"
	if d.Over || d.HeroSeen(d.Heroes[v].Pos, d.Heroes[1-v].Pos) {
		foeHP = fmt.Sprint(max(0, d.Heroes[1-v].HP))
	}
	head := fmt.Sprintf("Runde %d    %s    AP %d/%d    HP %d : %s",
		d.Round, d.Label(d.Turn), d.AP, d.Heroes[d.Turn].Class.AP, max(0, d.Heroes[v].HP), foeHP)
	if d.Over {
		head = fmt.Sprintf("Runde %d    Ende    HP %d : %d", d.Round, max(0, d.Heroes[0].HP), max(0, d.Heroes[1].HP))
	}
	rl.DrawText(head, 16, 12, 22, Ink)

	// Knöpfe
	y := float32(h) - DuelFoot + 8
	x := float32(16)
	next := func(width float32) rl.Rectangle {
		r := rl.NewRectangle(x, y, width, 34)
		x += width + 8
		return r
	}
	cmd := NoCommand
	mine := d.HumanTurn()
	st := d.Status()
	attackLabel := "Angriff"
	switch {
	case !mine:
	case st.Blocked:
		attackLabel = "Angriff: " + st.Reason
	case aiming:
		attackLabel = "Zielen abbrechen"
	case st.OK:
		attackLabel = fmt.Sprintf("Angriff -%d", st.Damage)
	default:
		attackLabel = "Zielen (Angriff)"
	}
	attackRect := next(250)
	if mine && st.OK && !aiming {
		if HotButton(attackRect, attackLabel) {
			cmd = AttackCommand
		}
	} else if Button(attackRect, attackLabel, mine && !st.Blocked) {
		cmd = AttackCommand
	}
	shield := fmt.Sprintf("Schild (-%d HP, Zug Ende)", d.Def.ShieldCost)
	if Button(next(250), shield, mine && d.CanShield(&d.State)) {
		cmd = ShieldCommand
	}
	if Button(next(150), "Zug beenden", mine) {
		cmd = EndCommand
	}
	if Button(next(70), "Neu", true) {
		cmd = NewCommand
	}
	if Button(next(80), "Menü", true) {
		cmd = MenuCommand
	}
	if Button(next(90), "Regeln", true) {
		cmd = RulesCommand
	}

	msg := d.Message
	if d.Over {
		msg += " Neu oder Menü?"
	}
	ly := int32(y) + 44
	rl.DrawText(msg, 16, ly, 20, Ink)
	for i := 1; i < min(3, len(d.Log)); i++ {
		rl.DrawText(d.Log[i], 16, ly+6+int32(i)*20, 16, rl.Fade(Ink, 0.6))
	}
	return cmd
}

// DuelRules sind die Regeln in Kurzform, für das Regelfenster.
func DuelRules(d *duel.Duel) []string {
	a := d.Def
	lines := []string{
		"Aktionspunkte (AP): 1 AP pro Feld, ein Angriff pro Zug (1 AP). Übrige AP verfallen.",
		fmt.Sprintf("Spieler 2 hat im allerersten Zug nur %d AP (Ausgleich für den Nachzug).", a.FirstAP),
		"Jeden Zug musst du dich mindestens 1 Feld bewegen.",
		"Angriffe nur in gerader Linie: Nahangriff 1 Feld, Fernangriff 2 bis Reichweite.",
		"Nichts blockiert Angriffe. Mit Zielen schlägst du auch ins Dunkle; daneben ist der Angriff weg.",
		fmt.Sprintf("Höher stehen: +%d Schaden. Aus der gegnerischen Spur (Gold) angreifen: +%d.", a.HeightBonus, a.TrailBonus),
		fmt.Sprintf("Schild: kostet %d HP, beendet den Zug; bis zu deinem nächsten Zug höchstens %d Schaden pro Treffer.", a.ShieldCost, a.ShieldDamage),
		"Sicht: Wald, Mauern und höhere Hügel versperren sie. Im Wald sieht man dich nur aus der Nähe.",
	}
	if a.Tower != nil {
		lines = append(lines, fmt.Sprintf("Turm: von oben siehst du alles im Umkreis %d und stehst höher als jeder Hügel.", a.Tower.Sight))
	}
	if st := a.Storm; st != nil {
		lines = append(lines, fmt.Sprintf("Sturm: ab Runde %d zieht er sich zur Mitte zusammen; wer am Zugende drin steht, verliert %d HP.", st.Delay+2, st.Damage))
	}
	return append(lines, "", "Klicken zum Schließen.")
}
