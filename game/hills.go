package game

import (
	"math"
	"math/rand/v2"

	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Hill ist eine runde Kuppe: Mitte, Radius in Feldern und Höhe in der Mitte.
type Hill struct {
	At             hex.Coord
	Radius, Height float64
}

// Roll würfelt einen Hügel mit der Mitte at nach dem Rezept.
func (d *HillsDef) Roll(rng *rand.Rand, at hex.Coord) Hill {
	return Hill{
		At:     at,
		Radius: d.Radius[0] + rng.Float64()*(d.Radius[1]-d.Radius[0]),
		Height: 1 + rng.Float64()*float64(d.Max),
	}
}

// RollCount würfelt, wie viele Hügel es geben soll.
func (d *HillsDef) RollCount(rng *rand.Rand) int {
	return d.Count[0] + rng.IntN(d.Count[1]-d.Count[0]+1)
}

// Heights rechnet Hügel in eine Höhe pro Feld um (Index wie im Grid). Jeder
// Hügel fällt von seiner Mitte zum Rand gleichmäßig ab; wo sich zwei
// überlappen, zählt der höhere. Gerundet, höchstens top.
func Heights(g grid.Grid, hills []Hill, top int) []int {
	out := make([]int, g.Len())
	for i := range out {
		c := g.Coord(i)
		v := 0.0
		for _, h := range hills {
			if d := float64(c.Dist(h.At)); d < h.Radius {
				v = math.Max(v, h.Height*(1-d/h.Radius))
			}
		}
		out[i] = min(top, int(math.Round(v)))
	}
	return out
}
