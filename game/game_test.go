package game

import (
	"fmt"
	"strings"
	"testing"

	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// testContent baut kleine Inhalte direkt in Go, ohne Mod-Dateien.
func testContent(t *testing.T) *Content {
	t.Helper()
	c := &Content{
		Terrain: map[string]*TerrainDef{
			"plain":  {Passable: true},
			"wall":   {BlocksSight: true},
			"forest": {Passable: true, BlocksSight: true},
		},
		Enemies: map[string]*EnemyDef{
			"hunter": {Name: "Jagdbot", HP: 3, Speed: 1, Behavior: "chase", Params: map[string]float64{"aggro": 8, "wander": 0}},
			"runner": {Name: "Läufer", HP: 1, Speed: 2, Behavior: "chase", Params: map[string]float64{"wander": 0}},
			"tank":   {Name: "Panzer", HP: 8, Speed: 0, Behavior: "idle"},
			"raider": {Name: "Marodeur", HP: 2, Speed: 1, Behavior: "raid", Razes: true, Becomes: "warlord",
				Params: map[string]float64{"aggro": 3, "raidRange": 10, "wander": 0}},
			"warlord": {Name: "Kriegsmech", HP: 5, Speed: 2, Behavior: "raid", Razes: true,
				Params: map[string]float64{"aggro": 3, "raidRange": 10, "wander": 0}},
		},
		Objects: map[string]*ObjectDef{
			"ruin": {Name: "Ruine", Shape: "cross"},
			"tower": {Name: "Sensorturm", Shape: "triangle", Sight: 3, Jump: true,
				OnEnter: []EffectCall{{Effect: "claim"}}},
			"colony": {Name: "Kolonie", Shape: "square", EnterMessage: "gesichert", RazesTo: "ruin",
				OnEnter: []EffectCall{{Effect: "heal", Params: map[string]float64{"amount": 2}}, {Effect: "remove"}}},
			"mine": {Name: "Mine", Shape: "triangle",
				OnEnter: []EffectCall{{Effect: "damage", Params: map[string]float64{"amount": 9}}, {Effect: "remove"}}},
		},
		Maps: map[string]*MapDef{
			"default": {
				Width: 13, Rows: 12, Base: "plain", Player: PlayerDef{HP: 5, Moves: 2},
				Objects: []ObjectSpawn{{Type: "colony", Count: 5, MinSpacing: 4, MinDistance: 4}},
				Storm:   &StormDef{Name: "Ionensturm", Delay: 1, Wave: []int{0, 1, 2}, Damage: 1},
				Features: []FeatureDef{
					{Terrain: "wall", Count: 4, Size: [2]int{3, 5}, Shape: "chain"},
					{Terrain: "forest", Count: 4, Size: [2]int{4, 7}, Shape: "blob"},
				},
				Enemies: []SpawnDef{{Type: "hunter", Count: 2, MinDistance: 5}},
			},
		},
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

// blank ist eine leere 7x7-Karte mit 5 HP und Laufweite 2.
func blank(t *testing.T) *State {
	s := NewBlank(testContent(t), "plain", 7, 7, 1)
	s.PlayerHP, s.PlayerMoves = 5, 2
	return s
}

func up(c hex.Coord, n int) hex.Coord { return c.Add(hex.Coord{Q: 0, R: -n}) }

// ---------- Kartenaufbau ----------

func TestNewIsWinnableAndRepeatable(t *testing.T) {
	c := testContent(t)
	for seed := uint64(1); seed <= 200; seed++ {
		s, err := New(c, "default", seed)
		if err != nil {
			t.Fatal(err)
		}
		if !s.goalReachable() {
			t.Fatalf("seed %d: Ziel unerreichbar", seed)
		}
		if len(s.Enemies) != 2 || len(s.Intents) != 2 {
			t.Fatalf("seed %d: %d Gegner, %d Pläne", seed, len(s.Enemies), len(s.Intents))
		}
		for _, e := range s.Enemies {
			if e.Pos.Dist(s.Player) < 5 || s.ObjectAt(e.Pos) != nil {
				t.Fatalf("seed %d: Gegner zu nah am Start oder auf einem Objekt", seed)
			}
		}
		if len(s.Objects) != 5 {
			t.Fatalf("seed %d: %d Kolonien", seed, len(s.Objects))
		}
		for i, o := range s.Objects {
			if !s.Passable(o.Pos) || o.Pos.Dist(s.Player) < 4 || o.Pos == s.Goal {
				t.Fatalf("seed %d: Kolonie auf %v liegt falsch", seed, o.Pos)
			}
			for _, other := range s.Objects[:i] {
				if other.Pos.Dist(o.Pos) < 4 {
					t.Fatalf("seed %d: Kolonien zu dicht", seed)
				}
			}
		}
	}
	c.Maps["default"].Player.Sight = 2
	fog, _ := New(c, "default", 7)
	c.Maps["default"].Player.Sight = 0
	if fog.Explored[fog.Grid.Index(fog.Goal)] || !fog.Explored[fog.Grid.Index(fog.Player)] {
		t.Fatal("mit Sichtweite 2 ist zu Beginn nur die Umgebung des Starts erkundet")
	}
	a, _ := New(c, "default", 42)
	b, _ := New(c, "default", 42)
	for i := range a.Tiles {
		if a.Tiles[i] != b.Tiles[i] {
			t.Fatal("gleicher seed muss dieselbe Karte ergeben")
		}
	}
	if _, err := New(c, "nope", 1); err == nil {
		t.Fatal("unbekannte Karte muss ein Fehler sein")
	}
}

func TestValidateReportsAllProblems(t *testing.T) {
	c := testContent(t)
	c.Enemies["hunter"].Behavior = "fly"
	c.Maps["default"].Base = "lava"
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), `"fly"`) || !strings.Contains(err.Error(), `"lava"`) {
		t.Fatalf("beide Fehler erwartet, war: %v", err)
	}
}

// ---------- Spieler ----------

func TestMovePlayer(t *testing.T) {
	s := blank(t)
	start := s.Player
	if s.MovePlayer(up(start, 3)) || s.Player != start {
		t.Fatal("drei Felder sind weiter als die Laufweite")
	}
	s.SetTerrain(up(start, 1), "wall")
	if s.MovePlayer(up(start, 1)) {
		t.Fatal("auf eine Mauer kann man nicht laufen")
	}
	side := start.Add(hex.Coord{Q: 1, R: -1})
	if !s.MovePlayer(side) || s.Player != side || s.Result != Running {
		t.Fatal("ein freies Nachbarfeld muss erreichbar sein")
	}
	s.Goal = up(side, 2)
	if !s.MovePlayer(s.Goal) || s.Result != Won {
		t.Fatal("auf dem Ziel ist der Einsatz gewonnen")
	}
}

func TestReachStopsAtFigures(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("tank", up(s.Player, 1))
	reach := s.Reach()
	if reach[s.Grid.Index(e.Pos)] != grid.Unreachable {
		t.Fatal("ein besetztes Feld ist nicht erreichbar")
	}
	if reach[s.Grid.Index(s.Player)] != 0 {
		t.Fatal("das Spielerfeld hat Abstand 0")
	}
}

func TestAttackEnemy(t *testing.T) {
	s := blank(t)
	far := s.AddEnemy("hunter", up(s.Player, 2))
	tough := s.AddEnemy("tank", up(s.Player, 1))
	weak := s.AddEnemy("hunter", s.Player.Add(hex.Coord{Q: 1, R: -1}))

	if s.AttackEnemy(far) || far.HP != 3 {
		t.Fatal("ein Gegner zwei Felder entfernt ist nicht angreifbar")
	}
	if !s.AttackEnemy(tough) || tough.HP != 3 || len(s.Enemies) != 3 {
		t.Fatalf("Schaden = HP des Spielers, Gegner bleibt: HP %d", tough.HP)
	}
	if !s.AttackEnemy(weak) || len(s.Enemies) != 2 || s.EnemyAt(weak.Pos) != nil {
		t.Fatal("ein besiegter Gegner verschwindet")
	}
}

func TestClickRunsEnemyTurn(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("hunter", up(s.Player, 5))
	s.planEnemies()
	s.Click(s.Player.Add(hex.Coord{Q: 1, R: -1}))
	if s.Turn != 2 || e.Pos != up(hex.FromOffset(3, 6), 4) {
		t.Fatalf("nach dem Klick ziehen die Gegner: Zug %d, Gegner auf %v", s.Turn, e.Pos)
	}
	s.Click(up(s.Player, 6)) // außer Reichweite: kein Zug
	if s.Turn != 2 {
		t.Fatal("eine ungültige Aktion darf den Zug nicht beenden")
	}
}

// ---------- Gegner ----------

func TestChaseWalksAroundWalls(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("hunter", up(s.Player, 3))
	s.planEnemies()
	in := s.Intents[0]
	if len(in.Steps) != 1 || in.Steps[0].To != up(s.Player, 2) || !in.Seen {
		t.Fatalf("freie Bahn: ein Schritt geradeaus, war %+v", in)
	}
	if e.Pos != up(s.Player, 3) {
		t.Fatal("Planen darf den Gegner nicht bewegen")
	}

	// Eingemauert: kein Schritt.
	for _, n := range s.Grid.Neighbors(e.Pos) {
		s.SetTerrain(n, "wall")
	}
	s.planEnemies()
	if len(s.Intents[0].Steps) != 0 {
		t.Fatal("eingemauert gibt es keinen Schritt")
	}
}

func TestChaseNeedsSightAndRemembers(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("hunter", up(s.Player, 4))
	s.SetTerrain(up(s.Player, 2), "forest") // Wald auf der Linie versperrt die Sicht
	s.planEnemies()
	if in := s.Intents[0]; in.Seen || len(in.Steps) != 0 {
		t.Fatalf("ohne Sicht und ohne Erinnerung bleibt er stehen: %+v", in)
	}

	// Mit Erinnerung geht er dorthin, auch ohne Sicht.
	seen := s.Player
	e.LastSeen = &seen
	s.planEnemies()
	if in := s.Intents[0]; in.Seen || len(in.Steps) != 1 {
		t.Fatalf("er soll zur letzten bekannten Position gehen: %+v", in)
	}
}

func TestSpeedAndBlocking(t *testing.T) {
	s := blank(t)
	s.AddEnemy("runner", up(s.Player, 4))
	s.planEnemies()
	if len(s.Intents[0].Steps) != 2 {
		t.Fatalf("speed 2 heißt zwei Schritte, waren %d", len(s.Intents[0].Steps))
	}
	// Zwei Gegner planen nie dasselbe Feld.
	s.AddEnemy("hunter", up(s.Player, 3).Add(hex.Coord{Q: 1, R: 0}))
	s.planEnemies()
	a, b := s.Intents[0].Steps, s.Intents[1].Steps
	if len(a) > 0 && len(b) > 0 && a[len(a)-1].To == b[len(b)-1].To {
		t.Fatalf("beide wollen nach %v", a[len(a)-1].To)
	}
}

func TestEnemyAttack(t *testing.T) {
	s := blank(t)
	s.AddEnemy("hunter", up(s.Player, 1))
	s.planEnemies()
	if !s.Intents[0].Attack {
		t.Fatal("ein Nachbar plant einen Angriff")
	}
	s.EndTurn()
	if s.PlayerHP != 2 || s.Turn != 2 {
		t.Fatalf("HP %d, Zug %d", s.PlayerHP, s.Turn)
	}
	s.EndTurn()
	if s.Result != Lost || s.PlayerHP != 0 {
		t.Fatal("der zweite Treffer besiegt den Spieler")
	}
}

func TestAttackMissesAfterDodge(t *testing.T) {
	s := blank(t)
	s.AddEnemy("hunter", up(s.Player, 1))
	s.planEnemies()
	s.Player = s.Player.Add(hex.Coord{Q: 2, R: -1}) // ausgewichen
	s.EndTurn()
	if s.PlayerHP != 5 {
		t.Fatal("der Angriff muss ins Leere gehen")
	}
}

func TestStepIntoPlayerBecomesAttack(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("hunter", up(s.Player, 2))
	s.planEnemies()
	s.Player = up(s.Player, 1) // dem Gegner in den Weg gelaufen
	s.EndTurn()
	if s.PlayerHP != 2 || e.Pos == s.Player {
		t.Fatalf("der Gegner bleibt stehen und schlägt zu: HP %d", s.PlayerHP)
	}
}

func TestRegisterBehavior(t *testing.T) {
	RegisterBehavior("flee-test", func(s *State, e *Enemy) Decision {
		return Decision{To: up(e.Pos, 1), Move: true}
	})
	defer delete(behaviors, "flee-test")
	c := testContent(t)
	c.Enemies["coward"] = &EnemyDef{Name: "Feigling", HP: 1, Speed: 1, Behavior: "flee-test"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	s := NewBlank(c, "plain", 7, 7, 1)
	e := s.AddEnemy("coward", up(s.Player, 3))
	s.planEnemies()
	s.EndTurn()
	if e.Pos != up(s.Player, 4) {
		t.Fatalf("das eigene Verhalten soll gelten, Gegner auf %v", e.Pos)
	}
}

// ---------- Objekte ----------

func TestColonyHealsOnceAndCounts(t *testing.T) {
	s := blank(t)
	pos := up(s.Player, 1)
	s.AddObject("colony", pos)
	if !s.MovePlayer(pos) {
		t.Fatal("ein Feld mit Objekt ist begehbar")
	}
	if s.PlayerHP != 7 || s.ObjectAt(pos) != nil || s.Collected["colony"] != 1 {
		t.Fatalf("HP %d, Objekt %v, gezählt %d", s.PlayerHP, s.ObjectAt(pos), s.Collected["colony"])
	}
	if s.Message != "gesichert" {
		t.Fatalf("Meldung: %q", s.Message)
	}
}

func TestDeadlyObjectBeatsGoal(t *testing.T) {
	s := blank(t)
	s.Goal = up(s.Player, 1)
	s.AddObject("mine", s.Goal)
	s.MovePlayer(s.Goal)
	if s.Result != Lost || s.PlayerHP != 0 {
		t.Fatalf("die Mine auf dem Ziel besiegt den Spieler: Result %v", s.Result)
	}
	if s.ObjectAt(s.Goal) == nil {
		t.Fatal("nach der Niederlage laufen keine weiteren Wirkungen")
	}
}

func TestRegisterEffect(t *testing.T) {
	RegisterEffect("moves-test", func(s *State, _ *Object, call *EffectCall) {
		s.PlayerMoves += int(call.Param("amount", 1))
	})
	defer delete(effects, "moves-test")
	c := testContent(t)
	c.Objects["boots"] = &ObjectDef{Shape: "square", OnEnter: []EffectCall{{Effect: "moves-test"}}}
	c.Objects["broken"] = &ObjectDef{Shape: "square", OnEnter: []EffectCall{{Effect: "nope"}}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("unbekannte Wirkung muss gemeldet werden: %v", err)
	}
	delete(c.Objects, "broken")
	s := NewBlank(c, "plain", 7, 7, 1)
	s.AddObject("boots", up(s.Player, 1))
	s.MovePlayer(up(s.Player, 1))
	if s.PlayerMoves != 2 {
		t.Fatalf("die eigene Wirkung soll gelten: Laufweite %d", s.PlayerMoves)
	}
}

// ---------- Sturm ----------

func TestStormAdvancesInWaves(t *testing.T) {
	s := blank(t)
	s.Storm = &StormDef{Name: "Sturm", Delay: 1, Wave: []int{0, 1, 2}, Damage: 1}
	s.Player = up(s.Player, 5) // weit weg vom Sturm
	start := s.StormFront
	if s.InStorm(hex.FromOffset(3, 6)) {
		t.Fatal("zu Beginn liegt nichts im Sturm")
	}
	var got []int
	for i := 0; i < 7; i++ {
		if s.NextStormFront() != s.StormFront-[]int{0, 0, 1, 2, 0, 1, 2}[i] {
			t.Fatalf("Zug %d: Vorschau stimmt nicht", s.Turn)
		}
		s.EndTurn()
		got = append(got, start-s.StormFront)
	}
	want := []int{0, 0, 1, 3, 3, 4, 6} // ein Zug Schonzeit, dann 0, 1, 2 im Wechsel
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Vorrücken %v, erwartet %v", got, want)
		}
	}
	// Ungerade Spalten liegen eine halbe Reihe tiefer und werden zuerst erfasst.
	if !s.InStorm(hex.FromOffset(3, 6)) || !s.InStorm(hex.FromOffset(2, 6)) || s.InStorm(hex.FromOffset(2, 3)) {
		t.Fatal("nach 6 halben Reihen liegt die unterste Reihe im Sturm, Reihe 3 nicht")
	}
}

func TestStormHurtsEveryoneInside(t *testing.T) {
	s := blank(t)
	s.Storm = &StormDef{Name: "Sturm", Wave: []int{2}, Damage: 1}
	weak := s.AddEnemy("runner", hex.FromOffset(0, 6))
	tough := s.AddEnemy("tank", hex.FromOffset(6, 6))
	safe := s.AddEnemy("tank", hex.FromOffset(0, 0))
	s.Enemies[0].Def = s.Content.Enemies["tank"] // bleibt stehen
	weak.HP = 1
	s.EndTurn()
	if s.PlayerHP != 4 {
		t.Fatalf("der Spieler in der untersten Reihe verliert 1 HP, hat %d", s.PlayerHP)
	}
	if len(s.Enemies) != 2 || tough.HP != 7 || safe.HP != 8 {
		t.Fatalf("schwacher Gegner weg, starker -1, sicherer unberührt: %d Gegner, %d, %d", len(s.Enemies), tough.HP, safe.HP)
	}
	s.PlayerHP = 1
	s.EndTurn()
	if s.Result != Lost {
		t.Fatal("im Sturm ohne HP ist der Einsatz verloren")
	}
}

func TestNoStormWithoutDef(t *testing.T) {
	s := blank(t)
	for i := 0; i < 20; i++ {
		s.EndTurn()
	}
	if s.PlayerHP != 5 || s.InStorm(s.Player) {
		t.Fatal("ohne storm im Rezept gibt es keinen Sturm")
	}
}

// ---------- Marodeure ----------

// raidSetup: Marodeur oben links, Kolonie zwei Felder unter ihm, Spieler
// weit weg unten rechts.
func raidSetup(t *testing.T) (s *State, raider *Enemy, colony *Object) {
	s = blank(t)
	s.Player = hex.FromOffset(6, 6)
	raider = s.AddEnemy("raider", hex.FromOffset(0, 0))
	colony = s.AddObject("colony", hex.FromOffset(0, 2))
	s.planEnemies()
	return s, raider, colony
}

func TestRaiderRazesColonyAndTransforms(t *testing.T) {
	s, raider, colony := raidSetup(t)
	if in := s.Intents[0]; len(in.Steps) != 1 || in.Steps[0].Razes || in.Seen {
		t.Fatalf("erster Zug: ein Schritt zur Kolonie, noch keine Zerstörung: %+v", in)
	}
	s.EndTurn()
	in := s.Intents[0]
	if len(in.Steps) != 1 || !in.Steps[0].Razes || in.Steps[0].To != colony.Pos {
		t.Fatalf("zweiter Zug: der Plan kündigt die Zerstörung an: %+v", in)
	}
	if colony.Def.ID != "colony" || raider.Def.ID != "raider" || raider.Pos == colony.Pos {
		t.Fatal("Planen darf nichts zerstören oder verwandeln")
	}
	s.EndTurn()
	if colony.Def.ID != "ruin" {
		t.Fatalf("die Kolonie ist jetzt eine Ruine, war %q", colony.Def.ID)
	}
	if raider.Def.ID != "warlord" || raider.HP != 5 {
		t.Fatalf("der Marodeur wird zum Kriegsmech mit vollen HP: %s, %d HP", raider.Def.ID, raider.HP)
	}
	if !strings.Contains(s.Message, "Marodeur zerstört Kolonie") || !strings.Contains(s.Message, "Kriegsmech") {
		t.Fatalf("Meldung: %q", s.Message)
	}
	// Eine Ruine gibt dem Spieler nichts und zählt nicht.
	s.Player = colony.Pos.Add(hex.Coord{Q: 1, R: 0})
	raider.Pos = hex.FromOffset(6, 0)
	s.MovePlayer(colony.Pos)
	if s.PlayerHP != 5 || s.Collected["colony"] != 0 || s.ObjectAt(colony.Pos) == nil {
		t.Fatal("eine Ruine heilt nicht und bleibt liegen")
	}
}

func TestCapturedColonyCannotBeRazed(t *testing.T) {
	s, raider, colony := raidSetup(t)
	s.EndTurn() // Marodeur steht jetzt neben der Kolonie und plant die Zerstörung
	s.Player = colony.Pos.Add(hex.Coord{Q: 1, R: 0})
	s.MovePlayer(colony.Pos) // der Spieler war schneller
	if s.PlayerHP != 7 {
		t.Fatal("die Kolonie heilt den Spieler")
	}
	s.EndTurn()
	if raider.Def.ID != "raider" {
		t.Fatal("ohne Kolonie keine Verwandlung")
	}
	if s.PlayerHP != 5 {
		t.Fatalf("der Spieler steht im Weg und wird angegriffen: HP %d", s.PlayerHP)
	}
}

func TestRaidPrefersNearbyPlayer(t *testing.T) {
	s, raider, _ := raidSetup(t)
	s.Player = raider.Pos.Add(hex.Coord{Q: 2, R: 0}) // zwei Felder daneben, in Sicht
	s.planEnemies()
	in := s.Intents[0]
	if !in.Seen || len(in.Steps) != 1 || in.Steps[0].To.Dist(s.Player) != 1 {
		t.Fatalf("der nahe Spieler geht vor: %+v", in)
	}
}

func TestRaidIgnoresHiddenAndFarColonies(t *testing.T) {
	s, _, colony := raidSetup(t)
	s.SetTerrain(hex.FromOffset(0, 1), "forest") // Wald zwischen Marodeur und Kolonie
	s.planEnemies()
	if len(s.Intents[0].Steps) != 0 {
		t.Fatal("eine Kolonie außer Sicht ist kein Ziel")
	}
	s.SetTerrain(hex.FromOffset(0, 1), "plain")
	s.Content.Enemies["raider"].Params["raidRange"] = 3 // Spieler ist weiter weg
	s.planEnemies()
	if len(s.Intents[0].Steps) != 0 {
		t.Fatalf("eine Kolonie fern vom Spieler ist kein Ziel (Abstand %d)", colony.Pos.Dist(s.Player))
	}
}

func TestRaidHuntsWhenNothingLeft(t *testing.T) {
	s := blank(t)
	e := s.AddEnemy("raider", up(s.Player, 5))
	s.planEnemies()
	if in := s.Intents[0]; !in.Seen || len(in.Steps) != 1 {
		t.Fatalf("ohne Kolonien jagt er den Spieler auf Jagdweite: %+v", in)
	}
	_ = e
}

func TestSecondRaiderSeesFirstOnesRuin(t *testing.T) {
	s := blank(t)
	s.Player = hex.FromOffset(6, 6)
	colony := s.AddObject("colony", hex.FromOffset(1, 1))
	s.AddEnemy("raider", colony.Pos.Add(hex.Coord{Q: -1, R: 0}))
	s.AddEnemy("raider", colony.Pos.Add(hex.Coord{Q: 1, R: 0}))
	s.planEnemies()
	a, b := s.Intents[0], s.Intents[1]
	if len(a.Steps) != 1 || !a.Steps[0].Razes {
		t.Fatalf("der erste zerstört die Kolonie: %+v", a)
	}
	for _, st := range b.Steps {
		if st.Razes || st.To == colony.Pos {
			t.Fatalf("für den zweiten ist sie schon Ruine und besetzt: %+v", b)
		}
	}
}

func TestWarlordPlansTwoStepsOnlyNextTurn(t *testing.T) {
	s, raider, _ := raidSetup(t)
	s.AddObject("colony", hex.FromOffset(0, 5))
	s.EndTurn()
	if n := len(s.Intents[0].Steps); n != 1 {
		t.Fatalf("im Zug der Verwandlung bleibt es bei einem Schritt, waren %d", n)
	}
	s.EndTurn()
	if raider.Def.ID != "warlord" || len(s.Intents[0].Steps) != 2 {
		t.Fatalf("als Kriegsmech plant er zwei Schritte: %s, %d", raider.Def.ID, len(s.Intents[0].Steps))
	}
}

// ---------- Alarm ----------

func alarmDef() *AlarmDef {
	return &AlarmDef{PerTurn: 2, Max: 40, Kill: 12, Levels: []int{15, 25, 31},
		SpawnRate: []int{0, 2, 3, 4}, SpawnAt: 10, MinRows: 3, Spawns: []string{"tank"}}
}

func TestAlarmRisesAndSpawns(t *testing.T) {
	s := blank(t)
	s.Alarm = alarmDef()
	for i := 0; i < 7; i++ {
		s.EndTurn()
	}
	if s.AlarmValue != 14 || s.AlarmLevel() != 0 || len(s.Enemies) != 0 {
		t.Fatalf("unter der ersten Schwelle kommt niemand: Alarm %d, %d Gegner", s.AlarmValue, len(s.Enemies))
	}
	// Ab Stufe 1 wächst der Zähler um 2 pro Zug: nach 5 Zügen ist er voll.
	for i := 0; i < 4; i++ {
		s.EndTurn()
	}
	if len(s.Enemies) != 0 {
		t.Fatal("der Zähler ist noch nicht voll")
	}
	s.EndTurn()
	if len(s.Enemies) != 1 || s.AlarmLevel() != 1 {
		t.Fatalf("jetzt landet der erste Gegner: %d Gegner, Stufe %d", len(s.Enemies), s.AlarmLevel())
	}
	e := s.Enemies[0]
	_, row := e.Pos.Offset()
	if row > 6-3 || e.Pos.Dist(s.Player) < 4 || !strings.Contains(s.Message, "Landung") {
		t.Fatalf("Nachschub landet nördlich mit Abstand: Reihe %d, Meldung %q", row, s.Message)
	}
	if len(s.Intents) != 1 {
		t.Fatal("der Neue bekommt sofort einen Plan")
	}
	for i := 0; i < 30; i++ {
		s.EndTurn()
	}
	if s.AlarmValue != 40 || s.AlarmLevel() != 3 {
		t.Fatalf("der Alarm bleibt am Maximum stehen: %d, Stufe %d", s.AlarmValue, s.AlarmLevel())
	}
}

func TestKillCalmsAlarm(t *testing.T) {
	s := blank(t)
	s.Alarm = alarmDef()
	s.AlarmValue = 20
	weak := s.AddEnemy("hunter", up(s.Player, 1))
	tough := s.AddEnemy("tank", s.Player.Add(hex.Coord{Q: 1, R: -1}))
	s.AttackEnemy(tough)
	if s.AlarmValue != 20 {
		t.Fatal("ein Treffer ohne Sieg senkt den Alarm nicht")
	}
	s.AttackEnemy(weak)
	if s.AlarmValue != 8 || !strings.Contains(s.Message, "Alarm -12") {
		t.Fatalf("ein Sieg senkt ihn um kill: %d, %q", s.AlarmValue, s.Message)
	}
	s.AlarmValue = 5
	tough.HP = 1
	s.AttackEnemy(tough)
	if s.AlarmValue != 0 {
		t.Fatal("der Alarm fällt nicht unter 0")
	}
}

func TestStormKillDoesNotCalmAlarm(t *testing.T) {
	s := blank(t)
	s.Alarm = alarmDef()
	s.AlarmValue = 20
	s.Storm = &StormDef{Name: "Sturm", Wave: []int{2}, Damage: 9}
	s.Player = up(s.Player, 5)
	s.AddEnemy("tank", hex.FromOffset(0, 6))
	s.EndTurn()
	if len(s.Enemies) != 0 || s.AlarmValue != 22 {
		t.Fatalf("nur Siege des Spielers zählen: %d Gegner, Alarm %d", len(s.Enemies), s.AlarmValue)
	}
}

func TestAlarmValidation(t *testing.T) {
	c := testContent(t)
	bad := alarmDef()
	bad.SpawnRate = []int{0, 2}
	bad.Spawns = []string{"ghost"}
	c.Maps["default"].Alarm = bad
	err := c.Validate()
	if err == nil || !strings.Contains(err.Error(), "spawnRate") || !strings.Contains(err.Error(), `"ghost"`) {
		t.Fatalf("beide Fehler erwartet: %v", err)
	}
}

// ---------- Sicht ----------

func TestSightRangeAndLineOfSight(t *testing.T) {
	s := blank(t)
	s.PlayerSight = 3
	s.SetTerrain(up(s.Player, 2), "forest")
	s.UpdateVision()
	cases := map[string]struct {
		c    hex.Coord
		want bool
	}{
		"eigenes Feld":                 {s.Player, true},
		"Nachbar":                      {up(s.Player, 1), true},
		"der Wald selbst":              {up(s.Player, 2), true},
		"hinter dem Wald":              {up(s.Player, 3), false},
		"freie Linie in Sichtweite":    {s.Player.Add(hex.Coord{Q: 3, R: -3}), true},
		"freie Linie außer Sichtweite": {s.Player.Add(hex.Coord{Q: -3, R: -1}), false},
	}
	for name, tc := range cases {
		if got := s.CanSee(tc.c); got != tc.want {
			t.Errorf("%s (%v): sichtbar %v, erwartet %v", name, tc.c, got, tc.want)
		}
	}
	s.PlayerSight = 0
	s.UpdateVision()
	if !s.CanSee(hex.FromOffset(0, 0)) || s.CanSee(up(s.Player, 3)) {
		t.Fatal("Sichtweite 0 heißt unbegrenzt, aber Wald versperrt weiter")
	}
}

func TestExploredStaysAndRemembersObjects(t *testing.T) {
	s := blank(t)
	s.PlayerSight = 2
	clear(s.Explored) // die leere Karte wurde mit unbegrenzter Sicht angelegt
	near := s.AddObject("colony", up(s.Player, 2))
	far := s.AddObject("colony", up(s.Player, 5))
	raider := s.AddEnemy("raider", near.Pos.Add(hex.Coord{Q: 1, R: -1}))
	s.planEnemies()
	s.UpdateVision()
	i := s.Grid.Index(near.Pos)
	if !s.Explored[i] || s.Remembered[i].Def.ID != "colony" {
		t.Fatal("die nahe Kolonie ist gesehen und gemerkt")
	}
	if j := s.Grid.Index(far.Pos); s.Explored[j] || s.Remembered[j].Def != nil {
		t.Fatal("die ferne Kolonie ist unbekannt")
	}

	// Spieler geht weg; der Marodeur zerstört die Kolonie außer Sicht.
	s.Player = s.Player.Add(hex.Coord{Q: -3, R: 0})
	s.EndTurn()
	if near.Def.ID != "ruin" || raider.Pos != near.Pos {
		t.Fatalf("Vorbedingung: Kolonie zerstört, ist %q", near.Def.ID)
	}
	if s.Visible[i] || !s.Explored[i] {
		t.Fatal("das Feld ist erkundet, aber außer Sicht")
	}
	if s.Remembered[i].Def.ID != "colony" {
		t.Fatal("in Erinnerung bleibt die Kolonie, bis er wieder hinsieht")
	}
	s.Player = up(near.Pos, -1)
	s.UpdateVision()
	if s.Remembered[i].Def.ID != "ruin" {
		t.Fatal("wieder in Sicht: jetzt weiß er von der Ruine")
	}
}

func TestTowerSeesOverObstaclesOnceClaimed(t *testing.T) {
	s := blank(t)
	s.PlayerSight = 1
	tower := s.AddObject("tower", up(s.Player, 1))
	behind := up(tower.Pos, 2)
	s.SetTerrain(up(tower.Pos, 1), "wall")
	s.UpdateVision()
	if s.CanSee(behind) {
		t.Fatal("ein unbesetzter Turm zeigt nichts")
	}
	s.MovePlayer(tower.Pos)
	if !tower.Owned || !s.CanSee(behind) || !s.CanSee(tower.Pos.Add(hex.Coord{Q: 3, R: -3})) {
		t.Fatal("besetzt sieht der Turm 3 Felder weit, auch hinter die Mauer")
	}
	if s.CanSee(up(tower.Pos, 4)) {
		t.Fatal("jenseits der Turmsicht bleibt es dunkel")
	}
	// Der Turm sieht weiter, auch wenn der Spieler weggeht.
	s.Player = hex.FromOffset(0, 6)
	s.UpdateVision()
	if !s.CanSee(behind) || !s.Remembered[s.Grid.Index(tower.Pos)].Owned {
		t.Fatal("der besetzte Turm bleibt ein Auge auf der Karte")
	}
}

func TestJumpOverWallFromTower(t *testing.T) {
	s := blank(t)
	wall, land := up(s.Player, 1), up(s.Player, 2)
	s.SetTerrain(wall, "wall")
	// Die Mauer ist breit genug, dass es außen herum zu weit ist.
	for _, d := range []hex.Coord{{Q: 1, R: -1}, {Q: -1, R: 0}, {Q: 2, R: -2}, {Q: -2, R: 0}} {
		s.SetTerrain(s.Player.Add(d), "wall")
	}
	if s.MovePlayer(land) {
		t.Fatal("ohne Turm kommt man nicht über die Mauer")
	}
	s.AddObject("tower", s.Player)
	if s.Reach()[s.Grid.Index(land)] != s.PlayerMoves {
		t.Fatal("vom Turm aus ist das Feld hinter der Mauer ein Sprungziel")
	}
	if s.Reach()[s.Grid.Index(wall)] != grid.Unreachable {
		t.Fatal("die Mauer selbst bleibt unbetretbar")
	}
	if !s.MovePlayer(land) || s.Player != land {
		t.Fatal("der Sprung ist ein normaler Zug")
	}

	// Kein Sprung über zwei Mauern, auf besetzte Felder oder über freies Feld.
	s.Player = hex.FromOffset(3, 3)
	s.AddObject("tower", s.Player)
	s.SetTerrain(up(s.Player, 1), "wall")
	s.SetTerrain(up(s.Player, 2), "wall")
	s.SetTerrain(s.Player.Add(hex.Coord{Q: 1, R: 0}), "wall")
	s.AddEnemy("tank", s.Player.Add(hex.Coord{Q: 2, R: 0}))
	reach := s.Reach()
	if reach[s.Grid.Index(up(s.Player, 3))] != grid.Unreachable {
		t.Fatal("zwei Mauern hintereinander sind zu viel")
	}
	if reach[s.Grid.Index(s.Player.Add(hex.Coord{Q: 2, R: 0}))] != grid.Unreachable {
		t.Fatal("auf einen Gegner kann man nicht springen")
	}
	if len(s.JumpTargets(hex.FromOffset(0, 0))) != 0 {
		t.Fatal("der Kartenrand ist keine Mauer zum Überspringen")
	}
}

func TestTowersPreferJumpSpots(t *testing.T) {
	c := testContent(t)
	m := c.Maps["default"]
	m.Objects = []ObjectSpawn{{Type: "tower", Count: 3, MinSpacing: 4, MinDistance: 3, AtWall: 1}}
	atWall, total := 0, 0
	for seed := uint64(1); seed <= 50; seed++ {
		s, err := New(c, "default", seed)
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range s.Objects {
			total++
			if len(s.JumpTargets(o.Pos)) > 0 {
				atWall++
			}
		}
	}
	if total < 140 || atWall*10 < total*8 {
		t.Fatalf("mit atWall 1 stehen fast alle Türme an einer Sprungstelle: %d von %d", atWall, total)
	}
}

// ---------- Gebiete, Hügel, Trupps ----------

// longMap ist eine Karte aus drei Gebieten mit Trennmauern.
func longMap(c *Content) {
	c.Maps["long"] = &MapDef{
		Width: 13, Rows: 8, Areas: 3, ViewRows: 6, Base: "plain", Player: PlayerDef{HP: 5, Moves: 2},
		Separator: &SeparatorDef{Terrain: "wall", Gaps: []int{3, 2}, Width: 2},
		Hills:     &HillsDef{Count: [2]int{2, 3}, Radius: [2]float64{2, 4}, Max: 3, Flat: 2},
		Features:  []FeatureDef{{Terrain: "wall", Count: 3, Size: [2]int{3, 4}, Shape: "chain"}},
		Objects:   []ObjectSpawn{{Type: "colony", Count: 2, MinSpacing: 3, MinDistance: 3}},
		Enemies:   []SpawnDef{{Type: "hunter", Count: 1, MinDistance: 4}},
	}
}

func TestLongMapHasAreasAndSeparators(t *testing.T) {
	c := testContent(t)
	longMap(c)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= 50; seed++ {
		s, err := New(c, "long", seed)
		if err != nil {
			t.Fatal(err)
		}
		if s.Grid.H != 3*8+2 || !s.goalReachable() {
			t.Fatalf("seed %d: %d Reihen, Ziel erreichbar %v", seed, s.Grid.H, s.goalReachable())
		}
		if s.AreaOf(s.Player) != 0 || s.AreaOf(s.Goal) != 2 {
			t.Fatalf("seed %d: Start im Süden, Ziel im Norden", seed)
		}
		// Südliche Trennmauer: 3 Durchgänge à 2 Felder.
		open := 0
		for col := range s.Grid.W {
			c := hex.FromOffset(col, s.Grid.H-1-8)
			if s.AreaOf(c) != -1 {
				t.Fatalf("seed %d: Reihe %d sollte Trennmauer sein", seed, s.Grid.H-1-8)
			}
			if s.Passable(c) {
				open++
			}
		}
		if open != 6 {
			t.Fatalf("seed %d: %d offene Felder in der Trennmauer, erwartet 6", seed, open)
		}
		perArea := map[int]int{}
		for _, o := range s.Objects {
			perArea[s.AreaOf(o.Pos)]++
		}
		if len(perArea) != 3 || len(s.Enemies) != 3 {
			t.Fatalf("seed %d: Objekte pro Gebiet %v, %d Gegner", seed, perArea, len(s.Enemies))
		}
		for i, h := range s.Height {
			if h < 0 || h > 3 || (h > 0 && s.Grid.Coord(i).Dist(s.Player) <= 2) {
				t.Fatalf("seed %d: Höhe %d auf %v", seed, h, s.Grid.Coord(i))
			}
		}
	}
}

func TestLogKeepsRecentMessages(t *testing.T) {
	s := blank(t)
	for i := range logLines + 3 {
		s.say(fmt.Sprint(i))
	}
	if len(s.Log) != logLines || s.Log[0] != fmt.Sprint(logLines+2) || s.Message != s.Log[0] {
		t.Fatalf("Protokoll: %v", s.Log)
	}
}

func TestRingStormClosesToTheCenter(t *testing.T) {
	s := blank(t)
	s.Storm = &StormDef{Name: "Sturm", Shape: "ring", Wave: []int{1}, Damage: 1}
	s.StormFront = s.stormStart()
	corner := hex.FromOffset(0, 0)
	for range 10 {
		s.EndTurn()
	}
	if !s.InStorm(corner) || s.InStorm(s.Center()) || s.StormFront != 1 {
		t.Fatalf("Ring: Ecke im Sturm, Mitte frei, Front %d", s.StormFront)
	}
}

// squadContent hat eine Armee aus drei Troopern.
func squadContent(t *testing.T) *Content {
	c := testContent(t)
	c.Enemies["trooper"] = &EnemyDef{Name: "Trooper", HP: 1, Speed: 1, Squad: 3, Tactic: "army",
		Params: map[string]float64{"aggro": 8, "hold": 2, "strike": 2, "wander": 0}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSquadLandsTogether(t *testing.T) {
	s := NewBlank(squadContent(t), "plain", 7, 7, 1)
	members := s.AddSquad("trooper", hex.FromOffset(3, 1), func(hex.Coord) bool { return true })
	if len(members) != 3 || members[0].Squad == 0 || len(s.SquadOf(members[1])) != 3 {
		t.Fatalf("Trupp aus 3: %d Mitglieder", len(members))
	}
	for _, m := range members[1:] {
		near := false
		for _, o := range members {
			near = near || (o != m && o.Pos.Dist(m.Pos) == 1)
		}
		if !near {
			t.Fatal("Mitglieder stehen beieinander")
		}
	}
}

func TestArmyHoldsDistanceUntilItCanStrike(t *testing.T) {
	s := NewBlank(squadContent(t), "plain", 9, 9, 1)
	s.PlayerHP = 5
	p := s.Player
	// Einer steht schon bereit, die anderen sind weit weg: Abstand halten.
	a := s.AddEnemy("trooper", up(p, 2))
	b := s.AddEnemy("trooper", up(p, 6))
	c := s.AddEnemy("trooper", up(p, 7))
	for _, e := range []*Enemy{a, b, c} {
		e.Squad = 1
	}
	s.planEnemies()
	for _, in := range s.Intents {
		if in.Attack {
			t.Fatal("allein greift keiner an")
		}
	}
	// Jetzt kommen zwei im selben Zug heran: Sie schlagen gemeinsam zu.
	b.Pos = up(p, 2).Add(hex.Coord{Q: 1, R: 0})
	s.planEnemies()
	attackers := 0
	for _, in := range s.Intents {
		if in.Attack {
			attackers++
		}
	}
	if attackers < 2 {
		t.Fatalf("zwei schlagen gemeinsam zu, waren %d", attackers)
	}
	s.EndTurn()
	if s.PlayerHP >= 5 {
		t.Fatal("der Angriff trifft")
	}
}
