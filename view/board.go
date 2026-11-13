// Package view zeichnet einen Einsatz. Es liest den Spielzustand nur und
// nimmt Farben und Schraffuren aus den Inhalten; die Regeln stehen in
// bouquet/game.
package view

import (
	"fmt"
	"image/color"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/game"
	"bouquet/lib/draw"
	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Farben der Oberfläche. Gelände und Gegner bringen ihre eigenen mit.
var (
	Paper     = rl.NewColor(214, 196, 160, 255)
	Ink       = rl.NewColor(74, 56, 38, 255)
	line      = rl.NewColor(74, 56, 38, 90)
	reachTint = rl.NewColor(58, 124, 190, 80)
	player    = rl.NewColor(36, 100, 170, 255)
	threat    = rl.NewColor(200, 50, 40, 255)
	wander    = rl.NewColor(120, 110, 100, 255)
	goal      = rl.NewColor(190, 140, 20, 255)
	rival     = rl.NewColor(178, 48, 40, 255) // zweiter Held im Duell
	light     = rl.NewColor(255, 250, 235, 255)
	hover     = rl.NewColor(30, 24, 16, 255)
	white     = rl.NewColor(255, 255, 255, 255)

	unknown     = rl.NewColor(200, 181, 144, 255) // unerkundet: leeres Pergament
	unknownLine = rl.NewColor(74, 56, 38, 28)
	dim         = rl.NewColor(74, 56, 38, 70) // erkundet, aber außer Sicht
)

const (
	HudHeight  = 44.0
	FootHeight = 80.0 // Meldung und zwei ältere Zeilen aus dem Protokoll
)

// Board zeichnet Einsätze mit den Farben aus den Inhalten.
type Board struct {
	colors map[string]color.RGBA
}

// NewBoard liest alle Farben aus den Inhalten. Eine ungültige Farbe ist ein
// Fehler, der die Stelle nennt.
func NewBoard(c *game.Content) (*Board, error) {
	b := &Board{colors: map[string]color.RGBA{}}
	add := func(where, s string) error {
		if s == "" {
			return nil
		}
		col, err := ParseColor(s)
		if err != nil {
			return fmt.Errorf("%s: %w", where, err)
		}
		b.colors[s] = col
		return nil
	}
	for id, t := range c.Terrain {
		if err := add("Gelände "+id+", fill", t.Fill); err != nil {
			return nil, err
		}
		if t.Hatch != nil {
			if err := add("Gelände "+id+", hatch.color", t.Hatch.Color); err != nil {
				return nil, err
			}
		}
	}
	for id, e := range c.Enemies {
		if err := add("Gegner "+id+", color", e.Color); err != nil {
			return nil, err
		}
	}
	for id, o := range c.Objects {
		if err := add("Objekt "+id+", color", o.Color); err != nil {
			return nil, err
		}
		if err := add("Objekt "+id+", ownedColor", o.OwnedColor); err != nil {
			return nil, err
		}
	}
	for id, m := range c.Maps {
		if m.Storm != nil {
			if err := add("Karte "+id+", storm.color", m.Storm.Color); err != nil {
				return nil, err
			}
		}
	}
	for id, a := range c.Arenas {
		if a.Storm != nil {
			if err := add("Arena "+id+", storm.color", a.Storm.Color); err != nil {
				return nil, err
			}
		}
	}
	return b, nil
}

// ParseColor liest "#rrggbb" oder "#rrggbbaa".
func ParseColor(s string) (color.RGBA, error) {
	c := color.RGBA{A: 255}
	var err error
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 9:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	default:
		err = fmt.Errorf("falsche Länge")
	}
	if err != nil {
		return c, fmt.Errorf("Farbe %q muss #rrggbb oder #rrggbbaa sein", s)
	}
	return c, nil
}

// Layout passt die Karte von s zwischen Kopf- und Fußzeile in ein Fenster
// der Größe w x h ein. Ist sie höher als ViewRows, folgt die Kamera dem
// Spieler.
func Layout(s *game.State, w, h float64) hex.Layout {
	if s.ViewRows > 0 && s.ViewRows < s.Grid.H {
		return draw.Follow(s.Grid.W, s.Grid.H, s.ViewRows, s.Player, 0, HudHeight, w, h-HudHeight-FootHeight)
	}
	return draw.Fit(s.Grid.W, s.Grid.H, 0, HudHeight, w, h-HudHeight-FootHeight)
}

// shown meldet, ob c im Fenster zu sehen ist.
func shown(l hex.Layout, c hex.Coord) bool {
	return draw.OnScreen(l, c, 0, 0, float64(rl.GetScreenWidth()), float64(rl.GetScreenHeight()))
}

// drawRelief schattiert das Feld c nach den Höhen ringsum: Hänge zum Licht
// (oben links) werden heller, abgewandte dunkler, höhere Felder etwas
// heller. height liefert die Höhe eines Feldes, -1 außerhalb der Karte.
func drawRelief(l hex.Layout, c hex.Coord, height func(hex.Coord) int) {
	h := height(c)
	at := func(dq, dr int) int {
		if v := height(c.Add(hex.Coord{Q: dq, R: dr})); v >= 0 {
			return v
		}
		return h // am Rand: kein Hang
	}
	// unten rechts höher als oben links: Der Hang schaut zum Licht.
	slope := at(1, 0) + at(0, 1) - at(-1, 0) - at(0, -1)
	v := 0.06*float64(h) + 0.07*float64(slope)
	switch {
	case v > 0.01:
		draw.Tint(l, c, light, v)
	case v < -0.01:
		draw.Tint(l, c, Ink, -v)
	}
}

// Draw zeichnet Karte, Figuren und Gegnerpläne. hovered ist das Feld unter
// der Maus.
//
// Gezeigt wird nur, was der Spieler weiß: Unerkundete Felder sind leeres
// Pergament, erkundete außer Sicht sind abgedunkelt und zeigen Objekte so,
// wie er sie zuletzt gesehen hat. Gegner und ihre Pläne sieht er nur auf
// Feldern in Sicht.
func (b *Board) Draw(s *game.State, l hex.Layout, hovered hex.Coord) {
	inReach := s.Reach()
	height := func(c hex.Coord) int {
		if !s.Grid.Contains(c) {
			return -1
		}
		return s.HeightAt(c)
	}
	for i, t := range s.Tiles {
		c := s.Grid.Coord(i)
		if !shown(l, c) {
			continue
		}
		if !s.Explored[i] {
			draw.Hex(l, c, 1, unknown)
			draw.Outline(l, c, 1, unknownLine)
			continue
		}
		if t.Fill != "" {
			draw.Hex(l, c, 1, b.colors[t.Fill])
		}
		drawRelief(l, c, height)
		if t.Hatch != nil {
			draw.Hatch(l, c, t.Hatch.Lines, t.Hatch.Width, b.colors[t.Hatch.Color])
		}
		if !s.Visible[i] {
			draw.Hex(l, c, 1, dim)
		}
		draw.Outline(l, c, 1, line)
	}

	b.drawStorm(s, l)

	// Laufbereich über Nebel und Sturm, damit er immer zu erkennen ist.
	if s.Result == game.Running {
		for i, d := range inReach {
			if d != grid.Unreachable && d > 0 && shown(l, s.Grid.Coord(i)) {
				draw.Hex(l, s.Grid.Coord(i), 1, reachTint)
			}
		}
	}

	for i, seen := range s.Remembered {
		if seen.Def == nil || !s.Explored[i] {
			continue
		}
		c := s.Grid.Coord(i)
		col, ok := b.colors[seen.Def.Color]
		if owned, has := b.colors[seen.Def.OwnedColor]; seen.Owned && has {
			col, ok = owned, true
		}
		if !ok {
			col = Ink
		}
		if !s.Visible[i] {
			col = rl.Fade(col, 0.55)
		}
		switch seen.Def.Shape {
		case "triangle":
			draw.Triangle(l, c, 0.5, white)
			draw.Triangle(l, c, 0.4, col)
		case "cross":
			draw.Cross(l, c, 0.36, l.Size*0.14, col)
		default:
			draw.Square(l, c, 0.36, col, white)
		}
	}

	draw.Ring(l, s.Goal, 0.55, 3, goal)
	draw.Label(l, s.Goal, "Z", 0.7, goal)

	for _, e := range s.Enemies {
		if !s.CanSee(e.Pos) {
			continue
		}
		col, ok := b.colors[e.Def.Color]
		if !ok {
			col = Ink
		}
		if s.Result == game.Running && e.Pos.Dist(s.Player) == 1 {
			draw.Ring(l, e.Pos, e.Def.Radius+0.15, 3, goal) // angreifbar
		}
		draw.Token(l, e.Pos, e.Def.Radius, col, fmt.Sprintf("%s%d", e.Def.Letter, e.HP), white)
		if e.LastSeen != nil {
			draw.Label(l, *e.LastSeen, "?", 0.6, wander)
		}
	}

	// Gegnerpläne: ein Pfeil pro Schritt, zufällige Schritte grau.
	attacked := false
	for _, in := range s.Intents {
		if !s.CanSee(in.Enemy.Pos) {
			continue
		}
		from := in.Enemy.Pos
		for _, st := range in.Steps {
			col := threat
			if st.Wander {
				col = wander
			}
			draw.Arrow(l, from, st.To, col)
			if st.Razes {
				draw.Outline(l, st.To, 3, threat) // hier wird etwas zerstört
			}
			from = st.To
		}
		attacked = attacked || in.Attack
	}
	// Roter Ring: Ein Gegner schlägt zu, wenn der Spieler stehen bleibt.
	if attacked && s.Result == game.Running {
		draw.Ring(l, s.Player, 0.82, 3, threat)
	}
	draw.Token(l, s.Player, 0.6, player, fmt.Sprint(s.PlayerHP), white)

	if s.Grid.Contains(hovered) {
		draw.Outline(l, hovered, 3, hover)
	}
}

// drawAlarm zeichnet den Alarm in die Kopfzeile, ab x: Beschriftung, Balken
// mit den Stufenschwellen als Strichen, und die Stufe. Der Balken ist grün
// unter Stufe 1, gelb auf Stufe 1 und rot darüber.
func drawAlarm(s *game.State, a *game.AlarmDef, x int32) {
	const barW, barH, y = 120, 14, 16
	rl.DrawText("Alarm", x, 12, 22, Ink)
	x += rl.MeasureText("Alarm", 22) + 10

	level := s.AlarmLevel()
	col := rl.NewColor(92, 138, 74, 255)
	switch {
	case level == 1:
		col = rl.NewColor(214, 160, 30, 255)
	case level >= 2:
		col = threat
	}
	rl.DrawRectangle(x-1, y-1, barW+2, barH+2, Ink)
	rl.DrawRectangle(x, y, barW, barH, rl.NewColor(232, 218, 186, 255))
	rl.DrawRectangle(x, y, int32(barW*s.AlarmValue/a.Max), barH, col)
	for _, min := range a.Levels {
		tick := x + int32(barW*min/a.Max)
		rl.DrawRectangle(tick, y, 1, barH, Ink)
	}
	rl.DrawText(fmt.Sprintf("Stufe %d", level), x+barW+10, 14, 18, Ink)
}

// drawStorm legt den Sturm über die Karte: kräftig, wo er schon ist, und
// blass, wo er nach diesem Zug sein wird. Die Schraffur macht ihn auch ohne
// Farbe erkennbar.
func (b *Board) drawStorm(s *game.State, l hex.Layout) {
	if s.Storm == nil {
		return
	}
	col, ok := b.colors[s.Storm.Color]
	if !ok {
		col = Ink
	}
	next := s.NextStormFront()
	if s.Result != game.Running {
		next = s.StormFront
	}
	for i := range s.Tiles {
		c := s.Grid.Coord(i)
		if !shown(l, c) {
			continue
		}
		switch depth := s.StormDistance(c); {
		case depth > s.StormFront:
			draw.Hex(l, c, 1, rl.Fade(col, 0.6))
			draw.Hatch(l, c, 4, 1.5, rl.Fade(white, 0.55))
		case depth > next:
			draw.Hex(l, c, 1, rl.Fade(col, 0.25))
			draw.Hatch(l, c, 4, 1.5, rl.Fade(white, 0.3))
		}
	}
}

// DrawHud zeichnet Kopf- und Fußzeile eines Einsatzes in ein Fenster der
// Größe w x h. title ist der Name des Ortes.
func DrawHud(s *game.State, title string, w, h int32) {
	head := fmt.Sprintf("%s    Zug %d    HP %d", title, s.Turn, s.PlayerHP)
	// Eingesammelte Objekte, in fester Reihenfolge
	ids := make([]string, 0, len(s.Collected))
	for id := range s.Collected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		head += fmt.Sprintf("    %s %d", s.Content.Objects[id].Name, s.Collected[id])
	}
	rl.DrawText(head, 16, 12, 22, Ink)
	if a := s.Alarm; a != nil {
		drawAlarm(s, a, 16+rl.MeasureText(head, 22)+28)
	}
	hint := "Leertaste: warten    R: neu    Esc: zurück"
	rl.DrawText(hint, w-rl.MeasureText(hint, 16)-16, h-int32(FootHeight)+12, 16, rl.Fade(Ink, 0.7))

	msg := s.Message
	if s.Result != game.Running {
		msg += " Klicken zum Weitermachen."
	}
	drawLog(msg, s.Log, h)
}

// drawLog schreibt die Meldung und darunter die zwei Meldungen davor in die
// Fußzeile. log ist das Protokoll mit der neuesten Meldung zuerst.
func drawLog(msg string, log []string, h int32) {
	y := h - int32(FootHeight) + 8
	rl.DrawText(msg, 16, y, 20, Ink)
	for i := 1; i < min(3, len(log)); i++ {
		rl.DrawText(log[i], 16, y+6+int32(i)*20, 16, rl.Fade(Ink, 0.6))
	}
}
