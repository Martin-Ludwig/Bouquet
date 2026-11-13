package hex

import "testing"

func TestDist(t *testing.T) {
	cases := []struct {
		a, b Coord
		want int
	}{
		{Coord{0, 0}, Coord{0, 0}, 0},
		{Coord{0, 0}, Coord{1, 0}, 1},
		{Coord{0, 0}, Coord{1, -1}, 1},
		{Coord{0, 0}, Coord{2, -1}, 2},
		{Coord{0, 0}, Coord{3, 3}, 6},
		{Coord{-2, 1}, Coord{2, -1}, 4},
	}
	for _, c := range cases {
		if got := c.a.Dist(c.b); got != c.want {
			t.Errorf("%v.Dist(%v) = %d, erwartet %d", c.a, c.b, got, c.want)
		}
	}
}

func TestNeighborsAreAtDistanceOne(t *testing.T) {
	c := Coord{3, -2}
	for _, n := range c.Neighbors() {
		if c.Dist(n) != 1 {
			t.Errorf("Nachbar %v hat Abstand %d", n, c.Dist(n))
		}
	}
}

func TestOffsetRoundTrip(t *testing.T) {
	for col := -3; col < 8; col++ {
		for row := -3; row < 8; row++ {
			gotCol, gotRow := FromOffset(col, row).Offset()
			if gotCol != col || gotRow != row {
				t.Fatalf("(%d,%d) wurde zu (%d,%d)", col, row, gotCol, gotRow)
			}
		}
	}
}

func TestLine(t *testing.T) {
	a, b := Coord{0, 0}, Coord{4, -2}
	line := Line(a, b)
	if len(line) != a.Dist(b)+1 {
		t.Fatalf("Länge %d, erwartet %d", len(line), a.Dist(b)+1)
	}
	if line[0] != a || line[len(line)-1] != b {
		t.Fatalf("Enden falsch: %v", line)
	}
	for i := 1; i < len(line); i++ {
		if line[i-1].Dist(line[i]) != 1 {
			t.Fatalf("Lücke zwischen %v und %v", line[i-1], line[i])
		}
	}
}

func TestLayoutRoundTrip(t *testing.T) {
	l := Layout{Size: 24, OriginX: 100, OriginY: 60}
	for q := -4; q < 5; q++ {
		for r := -4; r < 5; r++ {
			c := Coord{q, r}
			x, y := l.Center(c)
			if got := l.At(x+3, y-4); got != c {
				t.Fatalf("At(Center(%v)) = %v", c, got)
			}
		}
	}
}
