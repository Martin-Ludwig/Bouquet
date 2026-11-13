package grid

import (
	"testing"

	"bouquet/lib/hex"
)

func open(hex.Coord) bool { return true }

func TestIndexRoundTrip(t *testing.T) {
	g := New(7, 5)
	for i := 0; i < g.Len(); i++ {
		if got := g.Index(g.Coord(i)); got != i {
			t.Fatalf("Index(Coord(%d)) = %d", i, got)
		}
	}
	if g.Index(hex.FromOffset(7, 0)) != -1 || g.Index(hex.FromOffset(0, -1)) != -1 {
		t.Fatal("Felder außerhalb müssen -1 liefern")
	}
}

func TestDistanceFieldOpenMapEqualsHexDistance(t *testing.T) {
	g := New(9, 9)
	src := hex.FromOffset(4, 4)
	field := g.DistanceField([]hex.Coord{src}, open, -1)
	for i, d := range field {
		if want := src.Dist(g.Coord(i)); d != want {
			t.Fatalf("Feld %v: %d, erwartet %d", g.Coord(i), d, want)
		}
	}
}

func TestDistanceFieldWallsAndDepth(t *testing.T) {
	g := New(5, 3)
	// Spalte 2 ist komplett Mauer: die rechte Hälfte ist unerreichbar.
	passable := func(c hex.Coord) bool { return c.Q != 2 }
	field := g.DistanceField([]hex.Coord{hex.FromOffset(0, 1)}, passable, -1)
	if field[g.Index(hex.FromOffset(4, 1))] != Unreachable {
		t.Fatal("hinter der Mauer darf nichts erreichbar sein")
	}
	if field[g.Index(hex.FromOffset(1, 1))] != 1 {
		t.Fatal("Nachbar der Quelle muss Abstand 1 haben")
	}

	limited := g.DistanceField([]hex.Coord{hex.FromOffset(0, 0)}, open, 2)
	for i, d := range limited {
		if d > 2 {
			t.Fatalf("Feld %v liegt mit %d über maxDepth", g.Coord(i), d)
		}
	}
	if limited[g.Index(hex.FromOffset(4, 2))] != Unreachable {
		t.Fatal("jenseits von maxDepth muss Unreachable stehen")
	}
}

func TestLineClear(t *testing.T) {
	g := New(7, 3)
	a, b := hex.Coord{Q: 0, R: 1}, hex.Coord{Q: 0, R: 2}
	far := hex.Coord{Q: 4, R: -1}
	wall := hex.Coord{Q: 2, R: 0}
	blocks := func(c hex.Coord) bool { return c == wall }

	if !g.LineClear(a, b, blocks) {
		t.Fatal("Nachbarn sehen sich immer")
	}
	if g.LineClear(hex.Coord{Q: 0, R: 1}, far, blocks) {
		t.Fatal("die Mauer auf der Linie muss die Sicht versperren")
	}
	if !g.LineClear(wall, far, blocks) {
		t.Fatal("die Enden selbst zählen nicht als Hindernis")
	}
}
