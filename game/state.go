// Package game enthält die Spielregeln. Es importiert raylib nicht und läuft
// deshalb nativ unter "go test ./game".
//
// Aufteilung:
//
//	content.go   Beschreibung der Inhalte (Gelände, Gegner, Karten)
//	state.go     Zustand eines Einsatzes und Abfragen darauf
//	mapgen.go    Karte aus einem Rezept würfeln
//	hills.go     Hügel und Höhen
//	rules.go     Aktionen des Spielers
//	effect.go    Objekte und ihre Wirkungen, per Name registriert
//	vision.go    Sicht des Spielers, Nebel, Laufweite und Sprung
//	storm.go     der Sturm
//	alarm.go     Alarm und Nachschub
//	turn.go      Gegnerplan und Zugablauf
//	behavior.go  Gegnerverhalten, per Name registriert
//	tactic.go    gemeinsame Planung von Trupps, per Name registriert
package game

import (
	"fmt"
	"math/rand/v2"

	"bouquet/lib/grid"
	"bouquet/lib/hex"
)

// Result ist der Stand des Einsatzes.
type Result int

const (
	Running Result = iota
	Won
	Lost
)

// Enemy ist ein Gegner auf der Karte.
type Enemy struct {
	Def      *EnemyDef
	Pos      hex.Coord
	HP       int
	LastSeen *hex.Coord // wo er den Spieler zuletzt gesehen hat, sonst nil
	Squad    int        // Nummer seines Trupps; 0: allein
}

// Object ist ein Objekt auf der Karte, z. B. eine Kolonie.
type Object struct {
	Def   *ObjectDef
	Pos   hex.Coord
	Owned bool // vom Spieler besetzt (Wirkung "claim")
}

// State ist der komplette Zustand eines Einsatzes.
type State struct {
	Content     *Content
	Grid        grid.Grid
	Tiles       []*TerrainDef // ein Eintrag pro Feld, Zugriff über Grid.Index
	Height      []int         // Höhe pro Feld, 0 ist flach (nur für die Anzeige)
	ViewRows    int           // so viele Reihen zeigt die Kamera; 0: die ganze Karte
	Player      hex.Coord
	PlayerHP    int
	PlayerMoves int
	PlayerSight int        // Sichtweite in Feldern; 0: unbegrenzt
	Visible     []bool     // pro Feld: sieht der Spieler es gerade?
	Explored    []bool     // pro Feld: hat er es je gesehen?
	Remembered  []Sighting // pro Feld: was er dort zuletzt gesehen hat
	Enemies     []*Enemy
	Objects     []*Object
	Collected   map[string]int // Objekt-ID -> wie viele der Spieler eingesammelt hat
	Storm       *StormDef      // nil: kein Sturm
	StormFront  int            // Felder mit StormDepth darüber liegen im Sturm
	Alarm       *AlarmDef      // nil: kein Alarm, kein Nachschub
	AlarmValue  int
	Goal        hex.Coord
	Turn        int
	Result      Result
	Intents     []Intent // was die Gegner nach der nächsten Aktion tun
	Message     string   // letzte Meldung für die Anzeige
	Log         []string // die letzten Meldungen, die neueste zuerst

	rng          *rand.Rand
	areaRows     int              // Reihen pro Gebiet
	lastSquad    int              // zuletzt vergebene Trupp-Nummer
	spawnCounter int              // füllt sich je nach Alarmstufe; voll: Nachschub
	fields       map[string][]int // Distanzfelder, pro Planung nur einmal berechnet
	plannedAt    hex.Coord        // wo der Spieler beim Planen stand
}

// NewBlank legt eine leere Karte an, gefüllt mit dem Gelände base: Spieler
// unten in der Mitte, Ziel oben in der Mitte, keine Gegner.
func NewBlank(c *Content, base string, w, h int, seed uint64) *State {
	g := grid.New(w, h)
	s := &State{
		Content:     c,
		Grid:        g,
		Tiles:       make([]*TerrainDef, g.Len()),
		Height:      make([]int, g.Len()),
		Visible:     make([]bool, g.Len()),
		Explored:    make([]bool, g.Len()),
		Remembered:  make([]Sighting, g.Len()),
		Player:      hex.FromOffset(w/2, h-1),
		PlayerHP:    1,
		PlayerMoves: 1,
		Collected:   map[string]int{},
		Goal:        hex.FromOffset(w/2, 0),
		Turn:        1,
		rng:         rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		areaRows:    h,
	}
	for i := range s.Tiles {
		s.Tiles[i] = c.Terrain[base]
	}
	s.StormFront = s.stormStart()
	s.UpdateVision()
	return s
}

// ---------- Abfragen ----------

// logLines ist die Länge des Protokolls.
const logLines = 8

// say zeigt eine Meldung an und schreibt sie ins Protokoll.
func (s *State) say(msg string) {
	s.Message = msg
	s.Log = append([]string{msg}, s.Log[:min(len(s.Log), logLines-1)]...)
}

// Rand ist die Zufallsquelle des Einsatzes.
func (s *State) Rand() *rand.Rand { return s.rng }

// AreaOf ist das Gebiet, in dem c liegt: 0 ganz im Süden am Start, nach
// Norden aufsteigend; -1 für eine Reihe zwischen zwei Gebieten.
func (s *State) AreaOf(c hex.Coord) int {
	_, row := c.Offset()
	fromSouth := s.Grid.H - 1 - row
	if fromSouth%(s.areaRows+1) == s.areaRows {
		return -1
	}
	return fromSouth / (s.areaRows + 1)
}

// HeightAt ist die Höhe von c, 0 außerhalb der Karte.
func (s *State) HeightAt(c hex.Coord) int {
	if i := s.Grid.Index(c); i >= 0 {
		return s.Height[i]
	}
	return 0
}

// Terrain liefert das Gelände von c. c muss auf der Karte liegen.
func (s *State) Terrain(c hex.Coord) *TerrainDef { return s.Tiles[s.Grid.Index(c)] }

// SetTerrain setzt das Gelände von c auf die Geländeart id.
func (s *State) SetTerrain(c hex.Coord, id string) {
	s.Tiles[s.Grid.Index(c)] = s.Content.Terrain[id]
	s.fields = nil
}

// Passable meldet, ob c auf der Karte liegt und begehbar ist.
func (s *State) Passable(c hex.Coord) bool {
	return s.Grid.Contains(c) && s.Terrain(c).Passable
}

// EnemyAt liefert den Gegner auf c oder nil.
func (s *State) EnemyAt(c hex.Coord) *Enemy {
	for _, e := range s.Enemies {
		if e.Pos == c {
			return e
		}
	}
	return nil
}

// Free meldet, ob eine Figur c betreten kann: begehbar, kein Gegner, nicht
// der Spieler.
func (s *State) Free(c hex.Coord) bool {
	return s.Passable(c) && s.EnemyAt(c) == nil && c != s.Player
}

// ObjectAt liefert das Objekt auf c oder nil.
func (s *State) ObjectAt(c hex.Coord) *Object {
	for _, o := range s.Objects {
		if o.Pos == c {
			return o
		}
	}
	return nil
}

// AddObject legt ein Objekt des Typs typeID auf pos.
func (s *State) AddObject(typeID string, pos hex.Coord) *Object {
	o := &Object{Def: s.Content.Objects[typeID], Pos: pos}
	s.Objects = append(s.Objects, o)
	return o
}

// AddEnemy setzt einen Gegner des Typs typeID auf pos.
func (s *State) AddEnemy(typeID string, pos hex.Coord) *Enemy {
	def := s.Content.Enemies[typeID]
	e := &Enemy{Def: def, Pos: pos, HP: def.HP}
	s.Enemies = append(s.Enemies, e)
	return e
}

// AddSquad setzt einen Gegner des Typs typeID auf pos. Tritt der Typ als
// Trupp auf, kommen die übrigen Mitglieder auf freie Nachbarfelder, die ok
// erfüllen. Rückgabe: alle gesetzten Gegner.
func (s *State) AddSquad(typeID string, pos hex.Coord, ok func(hex.Coord) bool) []*Enemy {
	first := s.AddEnemy(typeID, pos)
	members := []*Enemy{first}
	if first.Def.Squad <= 1 {
		return members
	}
	s.lastSquad++
	first.Squad = s.lastSquad
	for len(members) < first.Def.Squad {
		var opts []hex.Coord
		for _, m := range members {
			for _, n := range s.Grid.Neighbors(m.Pos) {
				if s.Free(n) && ok(n) {
					opts = append(opts, n)
				}
			}
		}
		if len(opts) == 0 {
			break
		}
		e := s.AddEnemy(typeID, opts[s.rng.IntN(len(opts))])
		e.Squad = s.lastSquad
		members = append(members, e)
	}
	return members
}

// SquadOf liefert den Trupp von e, mit e selbst; ohne Trupp nur e.
func (s *State) SquadOf(e *Enemy) []*Enemy {
	if e.Squad == 0 {
		return []*Enemy{e}
	}
	var out []*Enemy
	for _, m := range s.Enemies {
		if m.Squad == e.Squad {
			out = append(out, m)
		}
	}
	return out
}

// FieldTo ist das Distanzfeld zum Feld target über alle begehbaren Felder,
// ohne Rücksicht auf Figuren. Kleinere Zahl heißt näher am Ziel.
func (s *State) FieldTo(target hex.Coord) []int {
	return s.FieldToAny([]hex.Coord{target})
}

// FieldToAny ist wie FieldTo, aber zum jeweils nächsten von mehreren Zielen.
func (s *State) FieldToAny(targets []hex.Coord) []int {
	key := fmt.Sprint(targets)
	if f, ok := s.fields[key]; ok {
		return f
	}
	f := s.Grid.DistanceField(targets, s.Passable, -1)
	if s.fields == nil {
		s.fields = map[string][]int{}
	}
	s.fields[key] = f
	return f
}

// Razable liefert die Felder aller Objekte, die ein Gegner zerstören kann.
func (s *State) Razable() []hex.Coord {
	var out []hex.Coord
	for _, o := range s.Objects {
		if o.Def.RazesTo != "" {
			out = append(out, o.Pos)
		}
	}
	return out
}

// StepToward liefert den freien Nachbarn von from, der laut field am
// nächsten am Ziel liegt und näher ist als from selbst.
func (s *State) StepToward(from hex.Coord, field []int) (hex.Coord, bool) {
	best := field[s.Grid.Index(from)]
	if best == grid.Unreachable {
		return hex.Coord{}, false
	}
	var to hex.Coord
	found := false
	for _, n := range s.Grid.Neighbors(from) {
		if d := field[s.Grid.Index(n)]; d != grid.Unreachable && d < best && s.Free(n) {
			best, to, found = d, n, true
		}
	}
	return to, found
}

// SightClear meldet, ob zwischen a und b freie Sicht besteht. Gelände mit
// blocksSight dazwischen versperrt sie.
func (s *State) SightClear(a, b hex.Coord) bool {
	return s.Grid.LineClear(a, b, func(c hex.Coord) bool { return s.Terrain(c).BlocksSight })
}
