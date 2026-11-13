// Package draw zeichnet Sechseckfelder und Spielfiguren mit raylib.
//
// Alle Funktionen bekommen ein hex.Layout und ein Feld; die Umrechnung in
// Pixel passiert hier drin. Aufrufen darf man sie nur zwischen
// rl.BeginDrawing und rl.EndDrawing.
package draw

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/lib/hex"
)

// Fit berechnet ein Layout, bei dem eine Karte mit cols Spalten und rows
// Reihen mittig in das Rechteck (x, y, w, h) passt.
func Fit(cols, rows int, x, y, w, h float64) hex.Layout {
	// Kartengröße in Einheiten des Feldradius
	unitsW := 1.5*float64(cols-1) + 2
	unitsH := math.Sqrt(3) * (float64(rows) + 0.5)
	size := math.Min(w/unitsW, h/unitsH)
	return hex.Layout{
		Size:    size,
		OriginX: x + (w-unitsW*size)/2 + size,
		OriginY: y + (h-unitsH*size)/2 + size*math.Sqrt(3)/2,
	}
}

// Follow berechnet ein Layout für eine Karte mit cols Spalten und rows
// Reihen, von der nur viewRows Reihen auf einmal in das Rechteck (x, y, w,
// h) passen. Das Feld focus liegt möglichst in der Mitte, die Ansicht geht
// aber nicht über den oberen oder unteren Kartenrand hinaus.
func Follow(cols, rows, viewRows int, focus hex.Coord, x, y, w, h float64) hex.Layout {
	l := Fit(cols, viewRows, x, y, w, h)
	half := l.Size * math.Sqrt(3) / 2
	mapH := math.Sqrt(3) * (float64(rows) + 0.5) * l.Size
	_, fy := hex.Layout{Size: l.Size}.Center(focus)
	top := y + half               // Karte oben bündig
	bottom := y + h - mapH + half // Karte unten bündig
	l.OriginY = math.Max(bottom, math.Min(top, y+h/2-fy))
	return l
}

// OnScreen meldet, ob das Feld c ganz oder teilweise im Rechteck (x, y, w,
// h) liegt. Damit lässt sich Zeichnen außerhalb der Ansicht sparen.
func OnScreen(l hex.Layout, c hex.Coord, x, y, w, h float64) bool {
	cx, cy := l.Center(c)
	return cx > x-l.Size && cx < x+w+l.Size && cy > y-l.Size && cy < y+h+l.Size
}

// Tint legt eine Farbe über das Feld c, z. B. Licht und Schatten von
// Hügeln. alpha von 0 bis 1.
func Tint(l hex.Layout, c hex.Coord, col color.RGBA, alpha float64) {
	col.A = uint8(math.Max(0, math.Min(1, alpha)) * 255)
	Hex(l, c, 1, col)
}

func center(l hex.Layout, c hex.Coord) rl.Vector2 {
	x, y := l.Center(c)
	return rl.NewVector2(float32(x), float32(y))
}

// Hex füllt das Feld c. scale 1 füllt es ganz, kleinere Werte lassen einen
// Rand frei.
func Hex(l hex.Layout, c hex.Coord, scale float64, col color.RGBA) {
	rl.DrawPoly(center(l, c), 6, float32(l.Size*scale), 0, col)
}

// Outline zieht den Rand des Feldes c nach.
func Outline(l hex.Layout, c hex.Coord, thick float64, col color.RGBA) {
	rl.DrawPolyLinesEx(center(l, c), 6, float32(l.Size), 0, float32(thick), col)
}

// Hatch schraffiert das Feld c mit schrägen Strichen (für Mauern, Wald,
// Sturm). lines ist die Anzahl der Striche.
func Hatch(l hex.Layout, c hex.Coord, lines int, thick float64, col color.RGBA) {
	x, y := l.Center(c)
	// Die Striche laufen von links unten nach rechts oben und bleiben
	// innerhalb des Inkreises, damit sie nicht über den Feldrand ragen.
	r := l.Size * math.Sqrt(3) / 2 * 0.85
	for i := 0; i < lines; i++ {
		off := (float64(i)+0.5)/float64(lines)*2 - 1 // -1 .. 1 quer zur Strichrichtung
		half := r * math.Sqrt(1-off*off)
		ox, oy := off*r*math.Sqrt2/2, off*r*math.Sqrt2/2
		dx, dy := half*math.Sqrt2/2, -half*math.Sqrt2/2
		rl.DrawLineEx(
			rl.NewVector2(float32(x+ox-dx), float32(y+oy-dy)),
			rl.NewVector2(float32(x+ox+dx), float32(y+oy+dy)),
			float32(thick), col,
		)
	}
}

// Token zeichnet eine Spielfigur: Kreis mit Schatten, hellem Rand und einer
// kurzen Beschriftung in der Farbe text. radius ist ein Anteil der Feldgröße
// (z. B. 0.6).
func Token(l hex.Layout, c hex.Coord, radius float64, col color.RGBA, label string, text color.RGBA) {
	p := center(l, c)
	r := float32(l.Size * radius)
	rl.DrawCircleV(rl.NewVector2(p.X+r*0.15, p.Y+r*0.25), r, rl.NewColor(0, 0, 0, 70))
	rl.DrawCircleV(p, r+2, rl.NewColor(250, 244, 228, 255))
	rl.DrawCircleV(p, r, col)
	Label(l, c, label, radius*0.95, text)
}

// Label schreibt einen kurzen Text mittig auf das Feld c. height ist die
// Schrifthöhe als Anteil der Feldgröße.
func Label(l hex.Layout, c hex.Coord, text string, height float64, col color.RGBA) {
	p := center(l, c)
	px := int32(math.Max(10, l.Size*height))
	w := rl.MeasureText(text, px)
	rl.DrawText(text, int32(p.X)-w/2, int32(p.Y)-px/2, px, col)
}

// Ring zieht einen Kreis um die Mitte des Feldes c (z. B. als Warnung).
func Ring(l hex.Layout, c hex.Coord, radius, thick float64, col color.RGBA) {
	r := float32(l.Size * radius)
	rl.DrawRing(center(l, c), r-float32(thick)/2, r+float32(thick)/2, 0, 360, 36, col)
}

// Arrow zeichnet einen Pfeil von Feld from zu Feld to. Er beginnt und endet
// etwas vor den Feldmitten, damit Figuren sichtbar bleiben.
func Arrow(l hex.Layout, from, to hex.Coord, col color.RGBA) {
	a, b := center(l, from), center(l, to)
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	length := math.Hypot(dx, dy)
	if length == 0 {
		return
	}
	dx, dy = dx/length, dy/length
	s := l.Size
	head := s * 0.35
	startX, startY := float64(a.X)+dx*s*0.6, float64(a.Y)+dy*s*0.6
	tipX, tipY := float64(b.X)-dx*s*0.15, float64(b.Y)-dy*s*0.15
	baseX, baseY := tipX-dx*head, tipY-dy*head
	px, py := -dy*head*0.6, dx*head*0.6

	rl.DrawLineEx(
		rl.NewVector2(float32(startX), float32(startY)),
		rl.NewVector2(float32(baseX), float32(baseY)),
		float32(math.Max(2, s*0.12)), col,
	)
	// raylib zeichnet ein Dreieck nur bei einer der beiden Eckenreihenfolgen.
	// Welche das ist, hängt von der Pfeilrichtung ab, deshalb beide.
	tip := rl.NewVector2(float32(tipX), float32(tipY))
	left := rl.NewVector2(float32(baseX-px), float32(baseY-py))
	right := rl.NewVector2(float32(baseX+px), float32(baseY+py))
	rl.DrawTriangle(tip, left, right, col)
	rl.DrawTriangle(tip, right, left, col)
}

// Square zeichnet ein Quadrat mit hellem Rand mittig auf das Feld c. half
// ist die halbe Kantenlänge als Anteil der Feldgröße.
func Square(l hex.Layout, c hex.Coord, half float64, col, rim color.RGBA) {
	p := center(l, c)
	h := float32(l.Size * half)
	rl.DrawRectangle(int32(p.X-h-2), int32(p.Y-h-2), int32(2*h+4), int32(2*h+4), rim)
	rl.DrawRectangle(int32(p.X-h), int32(p.Y-h), int32(2*h), int32(2*h), col)
}

// Triangle zeichnet ein Dreieck mit der Spitze nach oben mittig auf das Feld
// c. half ist die halbe Höhe als Anteil der Feldgröße.
func Triangle(l hex.Layout, c hex.Coord, half float64, col color.RGBA) {
	p := center(l, c)
	h := float32(l.Size * half)
	top := rl.NewVector2(p.X, p.Y-h)
	left := rl.NewVector2(p.X-h*0.9, p.Y+h*0.8)
	right := rl.NewVector2(p.X+h*0.9, p.Y+h*0.8)
	rl.DrawTriangle(top, left, right, col)
	rl.DrawTriangle(top, right, left, col) // beide Eckenreihenfolgen, siehe Arrow
}

// Cross zeichnet ein schräges Kreuz mittig auf das Feld c. half ist die
// halbe Breite als Anteil der Feldgröße.
func Cross(l hex.Layout, c hex.Coord, half, thick float64, col color.RGBA) {
	p := center(l, c)
	h := float32(l.Size * half)
	rl.DrawLineEx(rl.NewVector2(p.X-h, p.Y-h), rl.NewVector2(p.X+h, p.Y+h), float32(thick), col)
	rl.DrawLineEx(rl.NewVector2(p.X+h, p.Y-h), rl.NewVector2(p.X-h, p.Y+h), float32(thick), col)
}
