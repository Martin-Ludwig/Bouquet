// Package hex rechnet mit Sechseckfeldern (Flat-Top, axiale Koordinaten).
//
// Es kennt weder Karte noch Grafik: nur Koordinaten, Abstände, Nachbarn und
// die Umrechnung zwischen Feld und Pixel. Dadurch läuft es auch nativ unter
// "go test".
package hex

import "math"

const sqrt3 = 1.7320508075688772

// Coord ist ein Feld in axialen Koordinaten: Q ist die Spalte, R läuft schräg
// nach unten. Coord ist vergleichbar (==) und taugt als Map-Schlüssel.
type Coord struct{ Q, R int }

// Dirs sind die sechs Richtungen zu den Nachbarfeldern, auf dem Bildschirm gegen
// den Uhrzeigersinn ab "rechts unten".
var Dirs = [6]Coord{{1, 0}, {1, -1}, {0, -1}, {-1, 0}, {-1, 1}, {0, 1}}

// Add verschiebt c um d.
func (c Coord) Add(d Coord) Coord { return Coord{c.Q + d.Q, c.R + d.R} }

// Neighbors liefert alle sechs Nachbarn, auch solche außerhalb einer Karte.
// Für "nur Felder auf der Karte" siehe grid.Grid.Neighbors.
func (c Coord) Neighbors() [6]Coord {
	var out [6]Coord
	for i, d := range Dirs {
		out[i] = c.Add(d)
	}
	return out
}

// Dist ist der Abstand in Feldschritten ohne Rücksicht auf Hindernisse.
func (c Coord) Dist(o Coord) int {
	dq, dr := c.Q-o.Q, c.R-o.R
	return (abs(dq) + abs(dr) + abs(dq+dr)) / 2
}

// FromOffset wandelt Spalte/Reihe eines rechteckigen Rasters in axiale
// Koordinaten um. Ungerade Spalten liegen eine halbe Reihe tiefer.
func FromOffset(col, row int) Coord {
	return Coord{Q: col, R: row - (col-(col&1))/2}
}

// Offset ist die Umkehrung von FromOffset.
func (c Coord) Offset() (col, row int) {
	return c.Q, c.R + (c.Q-(c.Q&1))/2
}

// Round rundet gebrochene axiale Koordinaten auf das nächste Feld.
func Round(fq, fr float64) Coord {
	fs := -fq - fr
	q, r, s := math.Round(fq), math.Round(fr), math.Round(fs)
	dq, dr, ds := math.Abs(q-fq), math.Abs(r-fr), math.Abs(s-fs)
	switch {
	case dq > dr && dq > ds:
		q = -r - s
	case dr > ds:
		r = -q - s
	}
	return Coord{int(q), int(r)}
}

// Line liefert die Felder auf der geraden Linie von a nach b, beide Enden
// eingeschlossen. Eine winzige Verschiebung macht Grenzfälle eindeutig.
func Line(a, b Coord) []Coord {
	n := a.Dist(b)
	out := make([]Coord, 0, n+1)
	for i := 0; i <= n; i++ {
		t := 0.0
		if n > 0 {
			t = float64(i) / float64(n)
		}
		out = append(out, Round(
			float64(a.Q)+float64(b.Q-a.Q)*t+1e-6,
			float64(a.R)+float64(b.R-a.R)*t+2e-6,
		))
	}
	return out
}

// Layout legt fest, wo die Felder auf dem Bildschirm liegen: Size ist der
// Radius eines Feldes (Mitte bis Ecke), OriginX/OriginY die Mitte von Feld
// (0, 0) in Pixeln.
type Layout struct {
	Size             float64
	OriginX, OriginY float64
}

// Center ist die Pixelmitte des Feldes c.
func (l Layout) Center(c Coord) (x, y float64) {
	x = l.OriginX + l.Size*1.5*float64(c.Q)
	y = l.OriginY + l.Size*sqrt3*(float64(c.R)+float64(c.Q)/2)
	return x, y
}

// At ist das Feld unter dem Pixel (x, y).
func (l Layout) At(x, y float64) Coord {
	px, py := (x-l.OriginX)/l.Size, (y-l.OriginY)/l.Size
	return Round(2.0/3.0*px, -px/3+sqrt3/3*py)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
