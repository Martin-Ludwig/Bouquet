package view

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// Einfache Bedienelemente im Stil der Karte: Pergament mit Tintenrand. Sie
// zeichnen sich und melden im selben Aufruf, ob sie angeklickt wurden.

var (
	buttonFill  = rl.NewColor(232, 218, 186, 255)
	buttonHover = rl.NewColor(246, 236, 210, 255)
	buttonHot   = rl.NewColor(200, 70, 55, 255)
	panelFill   = rl.NewColor(222, 205, 170, 250)
)

// Button zeichnet einen Knopf und meldet, ob er in diesem Frame angeklickt
// wurde. Ein gesperrter Knopf ist blass und lässt sich nicht klicken.
func Button(r rl.Rectangle, label string, enabled bool) bool {
	return button(r, label, "", enabled, false)
}

// HotButton ist ein Knopf, der hervorsticht: Jetzt lohnt es sich.
func HotButton(r rl.Rectangle, label string) bool {
	return button(r, label, "", true, true)
}

func button(r rl.Rectangle, label, note string, enabled, hot bool) bool {
	over := enabled && rl.CheckCollisionPointRec(rl.GetMousePosition(), r)
	fill, text := buttonFill, Ink
	switch {
	case hot:
		fill, text = buttonHot, white
	case over:
		fill = buttonHover
	}
	if !enabled {
		fill, text = rl.Fade(fill, 0.5), rl.Fade(Ink, 0.4)
	}
	rl.DrawRectangleRec(r, fill)
	rl.DrawRectangleLinesEx(r, 2, rl.Fade(Ink, 0.8))
	const size, small = 18, 14
	cx := int32(r.X + r.Width/2)
	if note == "" {
		rl.DrawText(label, cx-rl.MeasureText(label, size)/2, int32(r.Y+r.Height/2)-size/2, size, text)
	} else {
		rl.DrawText(label, cx-rl.MeasureText(label, size)/2, int32(r.Y)+8, size, text)
		rl.DrawText(note, cx-rl.MeasureText(note, small)/2, int32(r.Y)+8+size+4, small, rl.Fade(text, 0.75))
	}
	return over && rl.IsMouseButtonPressed(rl.MouseButtonLeft)
}

// MenuItem ist ein Eintrag eines Menüs, mit einer kleinen Zeile darunter.
type MenuItem struct{ Label, Note string }

// Menu zeichnet eine Überschrift und darunter einen Knopf pro Eintrag,
// mittig in einem Fenster der Größe w x h. Rückgabe: der angeklickte
// Eintrag, sonst -1.
func Menu(title string, items []MenuItem, w, h int32) int {
	const bw, bh, gap = 460, 52, 10
	total := int32(40 + len(items)*(bh+gap))
	y := max(16, (h-total)/2)
	rl.DrawText(title, w/2-rl.MeasureText(title, 26)/2, y, 26, Ink)
	y += 44
	clicked := -1
	for i, it := range items {
		r := rl.NewRectangle(float32(w/2-bw/2), float32(y), bw, bh)
		if button(r, it.Label, it.Note, true, false) {
			clicked = i
		}
		y += bh + gap
	}
	return clicked
}

// Panel zeichnet einen Kasten mit Textzeilen, mittig im Fenster (z. B. die
// Regeln). Rückgabe: ob er angeklickt wurde, um ihn zu schließen.
func Panel(title string, lines []string, w, h int32) bool {
	const size, lh, pad = 16, 22, 18
	pw := int32(0)
	for _, l := range lines {
		pw = max(pw, rl.MeasureText(l, size))
	}
	pw = min(w-32, max(pw, rl.MeasureText(title, 22))+2*pad)
	ph := int32(len(lines))*lh + 2*pad + 34
	x, y := (w-pw)/2, max(8, (h-ph)/2)
	rl.DrawRectangle(0, 0, w, h, rl.Fade(Ink, 0.35))
	rl.DrawRectangle(x, y, pw, ph, panelFill)
	rl.DrawRectangleLines(x, y, pw, ph, Ink)
	rl.DrawText(title, x+pad, y+pad, 22, Ink)
	for i, l := range lines {
		rl.DrawText(l, x+pad, y+pad+34+int32(i)*lh, size, Ink)
	}
	return rl.IsMouseButtonPressed(rl.MouseButtonLeft)
}
