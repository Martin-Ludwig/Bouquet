// Package grid ist eine rechteckige Karte aus Sechseckfeldern mit Wegsuche
// und Sichtlinie. Was auf einem Feld liegt (Mauer, Wald, Figur), weiß das
// Paket nicht: Das Spiel sagt es ihm über kleine Funktionen.
package grid

import "bouquet/lib/hex"

// Unreachable steht im Distanzfeld für Felder, zu denen kein Weg führt.
const Unreachable = -1

// Grid ist eine Karte mit W Spalten und H Reihen. Jedes Feld hat einen Index
// von 0 bis Len()-1, reihenweise von links oben. Mit dem Index lassen sich
// eigene Daten pro Feld in einem Slice ablegen.
type Grid struct{ W, H int }

// New legt eine Karte mit w Spalten und h Reihen an.
func New(w, h int) Grid { return Grid{W: w, H: h} }

// Len ist die Anzahl der Felder.
func (g Grid) Len() int { return g.W * g.H }

// Contains meldet, ob c auf der Karte liegt.
func (g Grid) Contains(c hex.Coord) bool {
	col, row := c.Offset()
	return col >= 0 && col < g.W && row >= 0 && row < g.H
}

// Index ist der Index des Feldes c. Für Felder außerhalb der Karte: -1.
func (g Grid) Index(c hex.Coord) int {
	if !g.Contains(c) {
		return -1
	}
	col, row := c.Offset()
	return row*g.W + col
}

// Coord ist die Umkehrung von Index.
func (g Grid) Coord(i int) hex.Coord {
	return hex.FromOffset(i%g.W, i/g.W)
}

// Neighbors liefert die Nachbarn von c, die auf der Karte liegen.
func (g Grid) Neighbors(c hex.Coord) []hex.Coord {
	out := make([]hex.Coord, 0, 6)
	for _, n := range c.Neighbors() {
		if g.Contains(n) {
			out = append(out, n)
		}
	}
	return out
}

// DistanceField berechnet für jedes Feld die Weglänge zur nächsten Quelle
// (Breitensuche). Das Ergebnis hat Len() Einträge, abzufragen mit
// field[g.Index(c)]; Unreachable heißt: kein Weg.
//
// passable entscheidet, welche Felder betreten werden dürfen. Die Quellen
// selbst gelten immer als erreicht. maxDepth begrenzt die Suche; ein
// negativer Wert heißt unbegrenzt.
//
// Wer von A nach B will, berechnet das Feld von B aus und geht dann von A
// aus immer zu einem Nachbarn mit kleinerem Wert.
func (g Grid) DistanceField(sources []hex.Coord, passable func(hex.Coord) bool, maxDepth int) []int {
	field := make([]int, g.Len())
	for i := range field {
		field[i] = Unreachable
	}
	queue := make([]hex.Coord, 0, g.Len())
	for _, s := range sources {
		if i := g.Index(s); i >= 0 && field[i] == Unreachable {
			field[i] = 0
			queue = append(queue, s)
		}
	}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		d := field[g.Index(cur)]
		if maxDepth >= 0 && d >= maxDepth {
			continue
		}
		for _, n := range g.Neighbors(cur) {
			i := g.Index(n)
			if field[i] != Unreachable || !passable(n) {
				continue
			}
			field[i] = d + 1
			queue = append(queue, n)
		}
	}
	return field
}

// LineClear meldet, ob zwischen a und b freie Sicht besteht. Nur die Felder
// dazwischen zählen, die beiden Enden nicht. blocks sagt, welche Felder die
// Sicht versperren.
func (g Grid) LineClear(a, b hex.Coord, blocks func(hex.Coord) bool) bool {
	line := hex.Line(a, b)
	for i := 1; i < len(line)-1; i++ {
		if g.Contains(line[i]) && blocks(line[i]) {
			return false
		}
	}
	return true
}
