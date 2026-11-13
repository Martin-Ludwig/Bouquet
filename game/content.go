package game

import (
	"errors"
	"fmt"
	"slices"
	"sort"
)

// Content ist alles, was ein Spiel an Inhalten kennt: Gelände, Gegnertypen,
// Objekte, Karten, die Orte der Weltkarte und für das Duell Heldenklassen
// und Arenen. Es kommt aus Mod-Dateien (siehe bouquet/mods) oder wird in
// Tests direkt in Go gebaut. Der Schlüssel jeder Map ist die ID des Eintrags.
type Content struct {
	Terrain map[string]*TerrainDef
	Enemies map[string]*EnemyDef
	Objects map[string]*ObjectDef
	Maps    map[string]*MapDef
	Places  map[string]*PlaceDef
	Classes map[string]*ClassDef
	Arenas  map[string]*ArenaDef
	World   WorldDef
}

// WorldDef sind die Einstellungen der Weltkarte.
type WorldDef struct {
	Start      string `json:"start"`      // Ort, an dem ein neues Spiel beginnt
	Background string `json:"background"` // Bilddatei im Mod-Ordner, oder leer
	Survival   string `json:"survival"`   // Kartenrezept für "Überleben" im Hauptmenü, oder leer
}

// PlaceDef ist ein Ort auf der Weltkarte. X und Y sind seine Lage als Anteil
// der Bildbreite und -höhe (0 bis 1). Links sind die Routen zu anderen
// Orten; eine Route gilt in beide Richtungen, auch wenn nur ein Ort sie
// nennt. Map ist das Kartenrezept für Einsätze im Umland des Ortes.
type PlaceDef struct {
	ID    string   `json:"-"`
	Name  string   `json:"name"`
	X     float64  `json:"x"`
	Y     float64  `json:"y"`
	Links []string `json:"links"`
	Map   string   `json:"map"`
}

// Hatch ist eine Schraffur. Farben sind "#rrggbb" oder "#rrggbbaa".
type Hatch struct {
	Lines int     `json:"lines"`
	Width float64 `json:"width"`
	Color string  `json:"color"`
}

// TerrainDef beschreibt eine Geländeart.
type TerrainDef struct {
	ID          string `json:"-"`
	Name        string `json:"name"`
	Passable    bool   `json:"passable"`    // Figuren können das Feld betreten
	BlocksSight bool   `json:"blocksSight"` // versperrt die Sichtlinie
	Hides       int    `json:"hides"`       // im Duell: wer hier steht, ist nur aus so vielen Feldern zu sehen; 0: immer
	Fill        string `json:"fill"`
	Hatch       *Hatch `json:"hatch"`
}

// EnemyDef beschreibt einen Gegnertyp. Behavior ist der Name eines
// registrierten Verhaltens (siehe RegisterBehavior); Params sind dessen
// Regler, z. B. "aggro".
//
// Mit Squad > 1 tritt der Typ als Trupp auf: so viele Gegner nebeneinander.
// Nennt er eine Tactic (siehe RegisterTactic), plant sie den Zug des ganzen
// Trupps; Behavior ist dann nicht nötig.
type EnemyDef struct {
	ID       string             `json:"-"`
	Name     string             `json:"name"`
	Letter   string             `json:"letter"`
	HP       int                `json:"hp"`
	Speed    int                `json:"speed"` // Schritte pro Zug
	Behavior string             `json:"behavior"`
	Params   map[string]float64 `json:"params"`
	Color    string             `json:"color"`
	Radius   float64            `json:"radius"`  // Größe der Figur als Anteil des Feldes
	Razes    bool               `json:"razes"`   // zerstört Objekte mit razesTo, wenn er sie betritt
	Becomes  string             `json:"becomes"` // wird danach zu diesem Gegnertyp, mit dessen vollen HP
	Squad    int                `json:"squad"`   // Größe des Trupps; 0 oder 1: einzeln
	Tactic   string             `json:"tactic"`  // plant den Trupp gemeinsam, oder leer
}

// Param liefert einen Regler des Verhaltens oder def, wenn er fehlt.
func (d *EnemyDef) Param(name string, def float64) float64 {
	if v, ok := d.Params[name]; ok {
		return v
	}
	return def
}

// EffectCall ruft eine registrierte Wirkung auf (siehe RegisterEffect), mit
// deren Reglern, z. B. {"effect": "heal", "params": {"amount": 2}}.
type EffectCall struct {
	Effect string             `json:"effect"`
	Params map[string]float64 `json:"params"`
}

// Param liefert einen Regler der Wirkung oder def, wenn er fehlt.
func (c *EffectCall) Param(name string, def float64) float64 {
	if v, ok := c.Params[name]; ok {
		return v
	}
	return def
}

// ObjectDef beschreibt etwas, das auf einem Feld liegt und etwas auslöst,
// wenn der Spieler es betritt: eine Kolonie, eine Mine, ein Vorrat.
type ObjectDef struct {
	ID           string       `json:"-"`
	Name         string       `json:"name"`
	Shape        string       `json:"shape"` // "square", "triangle" oder "cross"
	Color        string       `json:"color"`
	RazesTo      string       `json:"razesTo"`      // Objekt, das ein zerstörender Gegner hinterlässt; leer: unzerstörbar
	OwnedColor   string       `json:"ownedColor"`   // Farbe, sobald der Spieler es besetzt hat
	Sight        int          `json:"sight"`        // besetzt: sieht so weit, auch über Hindernisse
	Jump         bool         `json:"jump"`         // von hier aus kann der Spieler über ein unbegehbares Feld springen
	OnEnter      []EffectCall `json:"onEnter"`      // Wirkungen, der Reihe nach
	EnterMessage string       `json:"enterMessage"` // Meldung danach
}

// MapDef ist das Rezept für eine Karte: Größe, Gelände und Gegner. Daraus
// würfelt New bei jedem Start eine neue Karte.
//
// Eine Karte kann aus mehreren Gebieten bestehen, die von Süden (Start) nach
// Norden (Ziel) aufeinander folgen, getrennt durch eine Mauerreihe mit
// Durchgängen (Separator). Gelände, Objekte und Gegner gelten pro Gebiet.
type MapDef struct {
	ID        string        `json:"-"`
	Name      string        `json:"name"`
	Width     int           `json:"width"`
	Rows      int           `json:"rows"`  // Reihen pro Gebiet
	Areas     int           `json:"areas"` // Anzahl der Gebiete; 0 heißt 1
	Separator *SeparatorDef `json:"separator"`
	ViewRows  int           `json:"viewRows"` // so viele Reihen sind zu sehen, die Kamera folgt dem Spieler; 0: alles
	Base      string        `json:"base"`     // Gelände, mit dem die Karte gefüllt wird
	Hills     *HillsDef     `json:"hills"`    // fehlt es, ist die Karte flach
	Player    PlayerDef     `json:"player"`
	Features  []FeatureDef  `json:"features"`
	Objects   []ObjectSpawn `json:"objects"`
	Enemies   []SpawnDef    `json:"enemies"`
	Storm     *StormDef     `json:"storm"` // fehlt es, gibt es keinen Sturm
	Alarm     *AlarmDef     `json:"alarm"` // fehlt es, kommt kein Nachschub
}

// AreaCount ist die Anzahl der Gebiete, mindestens 1.
func (m *MapDef) AreaCount() int { return max(1, m.Areas) }

// TotalRows ist die Höhe der ganzen Karte: alle Gebiete und die Trennmauern
// dazwischen.
func (m *MapDef) TotalRows() int { return m.AreaCount()*m.Rows + m.AreaCount() - 1 }

// SeparatorDef ist die Mauerreihe zwischen zwei Gebieten. Gaps ist die Zahl
// der Durchgänge je Trennmauer, von Süden nach Norden; fehlen Werte, gilt
// der letzte. Jeder Durchgang ist Width Felder breit.
type SeparatorDef struct {
	Terrain string `json:"terrain"`
	Gaps    []int  `json:"gaps"`
	Width   int    `json:"width"`
}

// HillsDef würfelt runde Hügel: Count[0] bis Count[1] Stück pro Gebiet, mit
// einem Radius zwischen Radius[0] und Radius[1] Feldern. Die Höhe eines
// Feldes reicht von 0 bis Max. Im Umkreis Flat um den Start bleibt es flach.
type HillsDef struct {
	Count  [2]int     `json:"count"`
	Radius [2]float64 `json:"radius"`
	Max    int        `json:"max"`
	Flat   int        `json:"flat"`
}

// PlayerDef sind die Startwerte des Spielers auf einer Karte.
type PlayerDef struct {
	HP    int `json:"hp"`
	Moves int `json:"moves"` // Laufweite pro Zug
	Sight int `json:"sight"` // Sichtweite in Feldern; 0 oder weggelassen: unbegrenzt
}

// FeatureDef streut ein Gelände über die Karte: Count Stück, jedes mit einer
// Größe zwischen Size[0] und Size[1] Feldern. Shape "chain" wächst als Kette
// (Mauern), "blob" als Fläche (Wälder).
type FeatureDef struct {
	Terrain string `json:"terrain"`
	Count   int    `json:"count"`
	Size    [2]int `json:"size"`
	Shape   string `json:"shape"`
}

// ObjectSpawn verteilt Count Objekte eines Typs auf freiem Grundgelände:
// mindestens MinDistance Felder vom Start und MinSpacing Felder voneinander
// entfernt. AtWall ist die Chance (0 bis 1), dass ein Objekt bevorzugt an
// einem Hindernis liegt, über das man von dort springen kann.
type ObjectSpawn struct {
	Type        string  `json:"type"`
	Count       int     `json:"count"`
	MinSpacing  int     `json:"minSpacing"`
	MinDistance int     `json:"minDistance"`
	AtWall      float64 `json:"atWall"`
}

// StormDef ist ein Sturm, der sich über die Karte schiebt. Shape "front"
// (Standard) ist eine Front, die von Süden nach Norden zieht; Wave gibt an,
// wie weit sie pro Zug vorrückt, in halben Feldreihen (2 = eine Reihe).
// Shape "ring" zieht sich von außen zur Kartenmitte zusammen, Wave in
// Feldringen; die Mitte und ihre Nachbarn bleiben immer frei. Die Folge Wave
// wiederholt sich. Wer am Zugende im Sturm steht, verliert Damage HP.
type StormDef struct {
	Name   string `json:"name"`
	Shape  string `json:"shape"`
	Delay  int    `json:"delay"` // so viele Züge Schonzeit vor der ersten Welle
	Wave   []int  `json:"wave"`
	Damage int    `json:"damage"`
	Color  string `json:"color"`
}

// Ring meldet, ob sich der Sturm ringförmig zusammenzieht.
func (d *StormDef) Ring() bool { return d.Shape == "ring" }

// Advance ist, wie weit der Sturm am Ende von Zug turn (ab 1) vorrückt.
func (d *StormDef) Advance(turn int) int {
	if turn-1 < d.Delay {
		return 0
	}
	return d.Wave[(turn-1-d.Delay)%len(d.Wave)]
}

// AlarmDef beschreibt den Alarm: Er steigt jeden Zug um PerTurn (höchstens
// bis Max) und sinkt um Kill, wenn der Spieler einen Gegner besiegt.
//
// Levels sind die Schwellen der Alarmstufen. SpawnRate hat einen Eintrag
// mehr als Levels: So viel wächst der Nachschub-Zähler pro Zug auf Stufe 0,
// 1, 2, ... Erreicht er SpawnAt, landet ein zufälliger Gegner aus Spawns,
// möglichst mindestens MinRows und höchstens MaxRows Reihen nördlich vom
// Spieler (MaxRows 0: beliebig weit).
type AlarmDef struct {
	PerTurn   int      `json:"perTurn"`
	Max       int      `json:"max"`
	Kill      int      `json:"kill"`
	Levels    []int    `json:"levels"`
	SpawnRate []int    `json:"spawnRate"`
	SpawnAt   int      `json:"spawnAt"`
	MinRows   int      `json:"minRows"`
	MaxRows   int      `json:"maxRows"`
	Spawns    []string `json:"spawns"`
}

// SpawnDef setzt Count Gegner eines Typs auf die Karte (bei einem Trupp:
// Count Trupps), mindestens MinDistance Felder vom Spieler entfernt.
type SpawnDef struct {
	Type        string `json:"type"`
	Count       int    `json:"count"`
	MinDistance int    `json:"minDistance"`
}

// ClassDef ist eine Heldenklasse im Duell. Melee ist der Schaden des
// Nahangriffs (1 Feld weit), Ranged der des Fernangriffs (2 bis Range Felder
// weit); 0 heißt, er kann das nicht.
type ClassDef struct {
	ID     string `json:"-"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	AP     int    `json:"ap"` // Aktionspunkte pro Zug
	Melee  int    `json:"melee"`
	Ranged int    `json:"ranged"`
	Range  int    `json:"range"`
}

// ArenaDef ist das Rezept einer Duellkarte. Hügel und Gelände werden an der
// waagerechten Mittellinie gespiegelt, damit beide Seiten gleich sind
// (Count gilt also pro Hälfte). Spieler 1 beginnt unten in der Mitte,
// Spieler 2 oben. Der Sturm zählt in Runden statt in Zügen.
type ArenaDef struct {
	ID       string       `json:"-"`
	Name     string       `json:"name"`
	Width    int          `json:"width"`
	Rows     int          `json:"rows"`
	Base     string       `json:"base"`
	Hills    *HillsDef    `json:"hills"`
	Features []FeatureDef `json:"features"`
	Tower    *TowerDef    `json:"tower"`
	Storm    *StormDef    `json:"storm"`

	FirstAP      int `json:"firstAP"`      // AP von Spieler 2 im allerersten Zug (Ausgleich für den Nachzug); 0: volle AP
	HeightBonus  int `json:"heightBonus"`  // mehr Schaden von höherem Boden
	Trail        int `json:"trail"`        // so viele eigene Zugenden bleibt ein verlassenes Feld Spur
	TrailBonus   int `json:"trailBonus"`   // mehr Schaden, wenn man aus der Spur des Gegners angreift
	ShieldCost   int `json:"shieldCost"`   // HP, die der Schild kostet
	ShieldDamage int `json:"shieldDamage"` // so viel Schaden nimmt man mit Schild höchstens pro Treffer
}

// TowerDef ist der Aussichtsturm einer Arena. Er steht von beiden Starts
// gleich weit entfernt, mindestens MinDistance Felder. Wer oben steht, sieht
// im Umkreis Sight alles, durch jedes Hindernis, und steht auf Höhe Height.
type TowerDef struct {
	Sight       int `json:"sight"`
	Height      int `json:"height"`
	MinDistance int `json:"minDistance"`
}

// Validate setzt die IDs der Einträge und prüft, ob alles zusammenpasst:
// Verweise auf Gelände, Gegnertypen und Verhalten müssen existieren. Alle
// gefundenen Fehler kommen zusammen zurück, damit man sie in einem Durchgang
// beheben kann.
func (c *Content) Validate() error {
	var errs []error
	bad := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	for _, id := range sortedKeys(c.Terrain) {
		c.Terrain[id].ID = id
	}
	for _, id := range sortedKeys(c.Enemies) {
		e := c.Enemies[id]
		e.ID = id
		if e.HP < 1 {
			bad("Gegner %q: hp muss mindestens 1 sein", id)
		}
		if e.Speed < 0 {
			bad("Gegner %q: speed darf nicht negativ sein", id)
		}
		if _, ok := tactics[e.Tactic]; e.Tactic != "" && !ok {
			bad("Gegner %q: unbekannte Taktik %q (bekannt: %v)", id, e.Tactic, TacticNames())
		}
		if _, ok := behaviors[e.Behavior]; e.Tactic == "" && !ok {
			bad("Gegner %q: unbekanntes Verhalten %q (bekannt: %v)", id, e.Behavior, BehaviorNames())
		}
		if e.Squad < 0 {
			bad("Gegner %q: squad darf nicht negativ sein", id)
		}
		if _, ok := c.Enemies[e.Becomes]; e.Becomes != "" && !ok {
			bad("Gegner %q: becomes nennt den unbekannten Gegnertyp %q", id, e.Becomes)
		}
	}
	for _, id := range sortedKeys(c.Objects) {
		o := c.Objects[id]
		o.ID = id
		if o.Shape != "square" && o.Shape != "triangle" && o.Shape != "cross" {
			bad("Objekt %q: shape muss \"square\", \"triangle\" oder \"cross\" sein, nicht %q", id, o.Shape)
		}
		if _, ok := c.Objects[o.RazesTo]; o.RazesTo != "" && !ok {
			bad("Objekt %q: razesTo nennt das unbekannte Objekt %q", id, o.RazesTo)
		}
		for i, call := range o.OnEnter {
			if _, ok := effects[call.Effect]; !ok {
				bad("Objekt %q, onEnter[%d]: unbekannte Wirkung %q (bekannt: %v)", id, i, call.Effect, EffectNames())
			}
		}
	}
	for _, id := range sortedKeys(c.Maps) {
		m := c.Maps[id]
		m.ID = id
		if m.Width < 3 || m.Rows < 3 {
			bad("Karte %q: mindestens 3 Spalten und 3 Reihen", id)
		}
		if m.Player.HP < 1 || m.Player.Moves < 1 {
			bad("Karte %q: player.hp und player.moves müssen mindestens 1 sein", id)
		}
		where := fmt.Sprintf("Karte %q", id)
		c.validateGround(where, m.Base, m.Hills, m.Features, bad)
		if m.Areas < 0 || m.ViewRows < 0 {
			bad("%s: areas und viewRows dürfen nicht negativ sein", where)
		}
		if sep := m.Separator; sep != nil {
			if _, ok := c.Terrain[sep.Terrain]; !ok {
				bad("%s, separator: unbekanntes Gelände %q", where, sep.Terrain)
			}
			if len(sep.Gaps) == 0 || sep.Width < 1 || slices.Min(sep.Gaps) < 1 {
				bad("%s, separator: braucht gaps mit Werten >= 1 und width >= 1", where)
			}
		}
		for i, sp := range m.Objects {
			if _, ok := c.Objects[sp.Type]; !ok {
				bad("Karte %q, objects[%d]: unbekanntes Objekt %q", id, i, sp.Type)
			}
		}
		validateStorm(where, m.Storm, bad)
		if a := m.Alarm; a != nil {
			if len(a.SpawnRate) != len(a.Levels)+1 {
				bad("Karte %q, alarm: spawnRate braucht einen Eintrag mehr als levels (%d statt %d)", id, len(a.Levels)+1, len(a.SpawnRate))
			}
			if a.SpawnAt < 1 || a.Max < 1 {
				bad("Karte %q, alarm: spawnAt und max müssen mindestens 1 sein", id)
			}
			for _, t := range a.Spawns {
				if _, ok := c.Enemies[t]; !ok {
					bad("Karte %q, alarm: spawns nennt den unbekannten Gegnertyp %q", id, t)
				}
			}
		}
		for i, sp := range m.Enemies {
			if _, ok := c.Enemies[sp.Type]; !ok {
				bad("Karte %q, enemies[%d]: unbekannter Gegnertyp %q", id, i, sp.Type)
			}
		}
	}
	for _, id := range sortedKeys(c.Places) {
		p := c.Places[id]
		p.ID = id
		if p.X < 0 || p.X > 1 || p.Y < 0 || p.Y > 1 {
			bad("Ort %q: x und y müssen zwischen 0 und 1 liegen", id)
		}
		if _, ok := c.Maps[p.Map]; !ok {
			bad("Ort %q: unbekannte Karte %q", id, p.Map)
		}
		for _, l := range p.Links {
			if _, ok := c.Places[l]; !ok || l == id {
				bad("Ort %q: links nennt %q, das ist kein anderer Ort", id, l)
			}
		}
	}
	for _, id := range sortedKeys(c.Classes) {
		k := c.Classes[id]
		k.ID = id
		if k.HP < 1 || k.AP < 1 || k.Melee < 0 || k.Ranged < 0 || k.Melee+k.Ranged == 0 {
			bad("Klasse %q: braucht hp >= 1, ap >= 1 und melee oder ranged", id)
		}
		if k.Ranged > 0 && k.Range < 2 {
			bad("Klasse %q: mit ranged muss range mindestens 2 sein", id)
		}
	}
	for _, id := range sortedKeys(c.Arenas) {
		a := c.Arenas[id]
		a.ID = id
		where := fmt.Sprintf("Arena %q", id)
		if a.Width < 3 || a.Rows < 5 {
			bad("%s: mindestens 3 Spalten und 5 Reihen", where)
		}
		c.validateGround(where, a.Base, a.Hills, a.Features, bad)
		validateStorm(where, a.Storm, bad)
		if a.FirstAP < 0 || a.Trail < 0 || a.ShieldCost < 0 || a.ShieldDamage < 0 {
			bad("%s: firstAP, trail, shieldCost und shieldDamage dürfen nicht negativ sein", where)
		}
	}
	if len(c.Places) > 0 {
		if _, ok := c.Places[c.World.Start]; !ok {
			bad("Welt: start %q ist kein Ort (bekannt: %v)", c.World.Start, sortedKeys(c.Places))
		}
	}
	if _, ok := c.Maps[c.World.Survival]; c.World.Survival != "" && !ok {
		bad("Welt: survival %q ist keine Karte", c.World.Survival)
	}
	return errors.Join(errs...)
}

// validateGround prüft, woraus eine Karte oder Arena gebaut wird.
func (c *Content) validateGround(where, base string, hills *HillsDef, features []FeatureDef, bad func(string, ...any)) {
	if t, ok := c.Terrain[base]; !ok {
		bad("%s: unbekanntes Gelände %q als base", where, base)
	} else if !t.Passable {
		bad("%s: base %q muss begehbar sein", where, base)
	}
	if h := hills; h != nil {
		if h.Count[0] < 0 || h.Count[1] < h.Count[0] || h.Radius[0] <= 0 || h.Radius[1] < h.Radius[0] || h.Max < 1 {
			bad("%s, hills: braucht count und radius als [min, max] und max >= 1", where)
		}
	}
	for i, f := range features {
		if _, ok := c.Terrain[f.Terrain]; !ok {
			bad("%s, features[%d]: unbekanntes Gelände %q", where, i, f.Terrain)
		}
		if f.Shape != "chain" && f.Shape != "blob" {
			bad("%s, features[%d]: shape muss \"chain\" oder \"blob\" sein, nicht %q", where, i, f.Shape)
		}
		if f.Size[0] < 1 || f.Size[1] < f.Size[0] {
			bad("%s, features[%d]: size braucht [min, max] mit 1 <= min <= max", where, i)
		}
	}
}

func validateStorm(where string, st *StormDef, bad func(string, ...any)) {
	if st == nil {
		return
	}
	if len(st.Wave) == 0 || st.Damage < 1 || st.Delay < 0 || slices.Min(st.Wave) < 0 {
		bad("%s, storm: braucht wave mit Werten >= 0, damage >= 1 und delay >= 0", where)
	}
	if st.Shape != "" && st.Shape != "front" && st.Shape != "ring" {
		bad("%s, storm: shape muss \"front\" oder \"ring\" sein, nicht %q", where, st.Shape)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
