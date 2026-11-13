package duel

import (
	"math/rand/v2"
	"testing"

	"bouquet/game"
	"bouquet/lib/hex"
)

func testContent(t *testing.T) *game.Content {
	t.Helper()
	c := &game.Content{
		Terrain: map[string]*game.TerrainDef{
			"plain":  {Passable: true},
			"wall":   {BlocksSight: true},
			"forest": {Passable: true, BlocksSight: true, Hides: 2},
		},
		Classes: map[string]*game.ClassDef{
			"fighter": {Name: "Kämpfer", HP: 10, AP: 3, Melee: 3, Ranged: 3, Range: 2},
			"archer":  {Name: "Schütze", HP: 11, AP: 3, Ranged: 3, Range: 3},
			"brute":   {Name: "Berserker", HP: 12, AP: 3, Melee: 4},
		},
		Arenas: map[string]*game.ArenaDef{
			"arena": {
				Width: 9, Rows: 11, Base: "plain",
				Hills: &game.HillsDef{Count: [2]int{2, 3}, Radius: [2]float64{2, 3.5}, Max: 2, Flat: 1},
				Features: []game.FeatureDef{
					{Terrain: "wall", Count: 4, Size: [2]int{2, 3}, Shape: "chain"},
					{Terrain: "forest", Count: 3, Size: [2]int{3, 5}, Shape: "blob"},
				},
				Tower:   &game.TowerDef{Sight: 5, Height: 3, MinDistance: 3},
				Storm:   &game.StormDef{Shape: "ring", Delay: 1, Wave: []int{1}, Damage: 2},
				FirstAP: 1, HeightBonus: 1, Trail: 3, TrailBonus: 1, ShieldCost: 1, ShieldDamage: 1,
			},
			// leer: flach, ohne Gelände, Turm und Sturm
			"empty": {Width: 9, Rows: 11, Base: "plain", FirstAP: 1, HeightBonus: 1, Trail: 3, TrailBonus: 1, ShieldCost: 1, ShieldDamage: 1},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func newDuel(t *testing.T, arena string, a, b string, ai [2]bool) *Duel {
	t.Helper()
	c := testContent(t)
	d, err := New(c, arena, [2]*game.ClassDef{c.Classes[a], c.Classes[b]}, ai, 7)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// place stellt beide Helden hin und gibt Spieler 1 volle AP.
func place(d *Duel, p0, p1 hex.Coord) {
	d.Heroes[0].Pos, d.Heroes[1].Pos = p0, p1
	d.Turn, d.AP = 0, d.Heroes[0].Class.AP
	d.updateBeliefs()
}

func TestArenaIsMirroredAndConnected(t *testing.T) {
	c := testContent(t)
	for seed := uint64(1); seed <= 50; seed++ {
		a, err := NewArena(c, "arena", rand.New(rand.NewPCG(seed, 1)))
		if err != nil {
			t.Fatal(err)
		}
		if !a.connected() || !a.HasTower {
			t.Fatalf("seed %d: Starts verbunden %v, Turm %v", seed, a.connected(), a.HasTower)
		}
		if a.Tower.Dist(a.Start[0]) != a.Tower.Dist(a.Start[1]) {
			t.Fatalf("seed %d: Turm nicht gleich weit von beiden Starts", seed)
		}
		for i := range a.Tiles {
			c := a.Grid.Coord(i)
			if !a.OnMap(c) {
				continue
			}
			m := a.Mirror(c)
			if !a.OnMap(m) || a.Terrain(c) != a.Terrain(m) || a.HeightAt(c) != a.HeightAt(m) {
				t.Fatalf("seed %d: %v und %v sind nicht gespiegelt", seed, c, m)
			}
		}
	}
}

func TestAttacksGoInStraightLinesOnly(t *testing.T) {
	d := newDuel(t, "empty", "archer", "fighter", [2]bool{})
	from := d.Center
	archer := d.Heroes[0].Class
	straight := from.Add(hex.Coord{Q: 0, R: -3})
	bent := from.Add(hex.Coord{Q: 1, R: -3})
	near := from.Add(hex.Coord{Q: 0, R: -1})
	if !d.CanTarget(from, straight, archer) || d.CanTarget(from, bent, archer) {
		t.Fatal("Fernangriff nur auf gerader Linie bis zur Reichweite")
	}
	if d.CanTarget(from, near, archer) {
		t.Fatal("ohne Nahangriff kein Ziel auf dem Nachbarfeld")
	}
}

func TestDamageBonusesAndShield(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "fighter", [2]bool{})
	foe := d.Center
	me := foe.Add(hex.Coord{Q: 0, R: 1})
	place(d, me, foe)
	if got := d.Damage(&d.State, 0, foe); got != 3 {
		t.Fatalf("Grundschaden 3, war %d", got)
	}
	d.Height[d.Grid.Index(me)] = 1
	if got := d.Damage(&d.State, 0, foe); got != 4 {
		t.Fatalf("von oben +1, war %d", got)
	}
	d.Heroes[1].Trail = []Mark{{Pos: me, Left: 2}}
	if got := d.Damage(&d.State, 0, foe); got != 5 {
		t.Fatalf("aus der Spur des Gegners +1, war %d", got)
	}
	d.Heroes[1].Shield = true
	if got := d.Damage(&d.State, 0, foe); got != 1 {
		t.Fatalf("Schild lässt nur 1 durch, war %d", got)
	}
	if d.Damage(&d.State, 0, foe.Add(hex.Coord{Q: 1, R: 0})) != 0 {
		t.Fatal("wo niemand steht, gibt es keinen Schaden")
	}
}

func TestMustMoveAndTrailAges(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "fighter", [2]bool{})
	start := d.Heroes[0].Pos
	if d.EndTurn() {
		t.Fatal("ohne Schritt darf der Zug nicht enden")
	}
	d.Move(start.Add(hex.Coord{Q: 0, R: -1}))
	if !OnTrail(&d.State, 0, start) {
		t.Fatal("das verlassene Feld wird Spur")
	}
	for range 3 {
		d.Heroes[0].Trail = ageTrail(d.Heroes[0].Trail)
	}
	if OnTrail(&d.State, 0, start) {
		t.Fatal("nach drei eigenen Zugenden ist die Spur weg")
	}
}

func TestSecondPlayerStartsWithFewerAP(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "fighter", [2]bool{})
	d.Move(d.Heroes[0].Pos.Add(hex.Coord{Q: 0, R: -1}))
	d.EndTurn()
	if d.Turn != 1 || d.AP != 1 {
		t.Fatalf("Spieler 2 hat im ersten Zug 1 AP, hat %d", d.AP)
	}
}

func TestForestHidesFromAfar(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "fighter", [2]bool{})
	foe := d.Center
	d.Tiles[d.Grid.Index(foe)] = d.Tiles[0] // erst einmal Ebene
	place(d, foe.Add(hex.Coord{Q: 0, R: 4}), foe)
	if !d.Sees(&d.State, 0) {
		t.Fatal("auf freier Ebene sichtbar")
	}
	forest := &game.TerrainDef{Passable: true, Hides: 2}
	d.Tiles[d.Grid.Index(foe)] = forest
	if d.Sees(&d.State, 0) {
		t.Fatal("im Wald aus 4 Feldern nicht zu sehen")
	}
	d.Heroes[0].Pos = foe.Add(hex.Coord{Q: 0, R: 2})
	if !d.Sees(&d.State, 0) {
		t.Fatal("aus 2 Feldern zu sehen")
	}
}

func TestStormClosesToTheCenter(t *testing.T) {
	d := newDuel(t, "arena", "fighter", "fighter", [2]bool{})
	if d.StormRadius(2) != d.Radius || d.StormRadius(3) != d.Radius-1 {
		t.Fatalf("Sturm ab Runde 3: %d, %d (Rand %d)", d.StormRadius(2), d.StormRadius(3), d.Radius)
	}
	if d.StormRadius(100) != 1 || d.InStorm(d.Center, 100) {
		t.Fatal("die Mitte bleibt frei")
	}
}

func TestBlindAttackMissesAndForgets(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "fighter", [2]bool{})
	guess, hidden := d.Center, d.Center.Add(hex.Coord{Q: 3, R: -3})
	d.Tiles[d.Grid.Index(hidden)] = &game.TerrainDef{Passable: true, Hides: 2}
	place(d, guess.Add(hex.Coord{Q: 0, R: 2}), hidden)
	d.Belief[0] = &guess
	if !d.AttackAt(guess) || d.Heroes[1].HP != d.Heroes[1].Class.HP || d.Belief[0] != nil {
		t.Fatalf("ins Leere: kein Schaden, Vermutung weg (%v)", d.Belief[0])
	}
	if d.AttackAt(guess) {
		t.Fatal("nur ein Angriff pro Zug")
	}
}

func TestAIStrikesWhenItCan(t *testing.T) {
	d := newDuel(t, "empty", "fighter", "brute", [2]bool{false, true})
	foe := d.Center
	place(d, foe.Add(hex.Coord{Q: 0, R: 3}), foe)
	d.Turn, d.AP = 1, 3
	d.startTurn()
	for d.AIStep() {
	}
	if d.Heroes[0].HP == d.Heroes[0].Class.HP {
		t.Fatal("die KI läuft heran und schlägt zu")
	}
}

// Die KI spielt gegen sich selbst. Jede Partie muss enden, und keine Seite
// soll haushoch überlegen sein.
func TestAIGamesEnd(t *testing.T) {
	c := testContent(t)
	wins := [3]int{} // Spieler 1, Spieler 2, unentschieden
	games := 30
	if testing.Short() {
		games = 6
	}
	for g := range games {
		d, err := New(c, "arena", [2]*game.ClassDef{c.Classes["fighter"], c.Classes["fighter"]}, [2]bool{true, true}, uint64(g+1))
		if err != nil {
			t.Fatal(err)
		}
		for turns := 0; !d.Over; turns++ {
			if turns > 200 {
				t.Fatalf("Partie %d endet nicht", g)
			}
			for d.AIStep() {
			}
		}
		if d.Winner < 0 {
			wins[2]++
		} else {
			wins[d.Winner]++
		}
	}
	t.Logf("Spieler 1: %d, Spieler 2: %d, unentschieden: %d", wins[0], wins[1], wins[2])
	if wins[0] == games || wins[1] == games {
		t.Fatalf("eine Seite gewinnt immer: %v", wins)
	}
}
