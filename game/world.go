package game

import (
	"encoding/json"
	"fmt"
	"sort"
)

// World ist der dauerhafte Zustand über alle Einsätze hinweg: wo der Spieler
// ist, welche Orte er kennt und was er dort erreicht hat. Einsätze werden
// von hier gestartet und ihr Ergebnis hier verbucht.
type World struct {
	Content *Content
	Current string                 // ID des Ortes, an dem der Spieler ist
	Known   map[string]bool        // Orte, die er erreicht hat
	Stats   map[string]*PlaceStats // pro Ort, angelegt beim ersten Einsatz
	Message string                 // letzte Meldung für die Anzeige

	target string // Ort des laufenden Einsatzes, sonst leer
}

// PlaceStats ist, was der Spieler an einem Ort erreicht hat.
type PlaceStats struct {
	Wins      int            `json:"wins"`
	Losses    int            `json:"losses"`
	Collected map[string]int `json:"collected"` // Objekt-ID -> insgesamt eingesammelt
}

// WorldAction ist, was ein Klick auf einen Ort bewirkt.
type WorldAction int

const (
	NoAction      WorldAction = iota // nicht erreichbar von hier
	TravelAction                     // bekannter Nachbarort: hinreisen
	MissionAction                    // Einsatz: am eigenen Ort erneut, oder um einen unbekannten Nachbarort zu erreichen
)

// NewWorld beginnt ein neues Spiel am Startort.
func NewWorld(c *Content) *World {
	return &World{
		Content: c,
		Current: c.World.Start,
		Known:   map[string]bool{c.World.Start: true},
		Stats:   map[string]*PlaceStats{},
	}
}

// Place liefert den Ort, an dem der Spieler ist.
func (w *World) Place() *PlaceDef { return w.Content.Places[w.Current] }

// Adjacent meldet, ob zwischen den Orten a und b eine Route besteht.
func (w *World) Adjacent(a, b string) bool {
	pa, pb := w.Content.Places[a], w.Content.Places[b]
	if pa == nil || pb == nil {
		return false
	}
	for _, l := range pa.Links {
		if l == b {
			return true
		}
	}
	for _, l := range pb.Links {
		if l == a {
			return true
		}
	}
	return false
}

// ActionFor sagt, was ein Klick auf den Ort id bewirken würde.
func (w *World) ActionFor(id string) WorldAction {
	switch {
	case w.Content.Places[id] == nil:
		return NoAction
	case id == w.Current:
		return MissionAction
	case !w.Adjacent(w.Current, id):
		return NoAction
	case w.Known[id]:
		return TravelAction
	default:
		return MissionAction
	}
}

// StatsFor liefert die Bilanz eines Ortes; nie nil.
func (w *World) StatsFor(id string) *PlaceStats {
	if st, ok := w.Stats[id]; ok {
		return st
	}
	return &PlaceStats{}
}

// Travel reist zu einem bekannten Nachbarort. Rückgabe: ob es geklappt hat.
func (w *World) Travel(id string) bool {
	if w.ActionFor(id) != TravelAction {
		return false
	}
	w.Current = id
	w.Message = fmt.Sprintf("Du reist nach %s.", w.Place().Name)
	return true
}

// StartMission würfelt einen Einsatz im Umland des Ortes id. Erlaubt ist der
// eigene Ort (beliebig oft) oder ein unbekannter Nachbarort.
func (w *World) StartMission(id string, seed uint64) (*State, error) {
	if w.ActionFor(id) != MissionAction {
		return nil, fmt.Errorf("von %q aus ist kein Einsatz in %q möglich", w.Current, id)
	}
	s, err := New(w.Content, w.Content.Places[id].Map, seed)
	if err != nil {
		return nil, err
	}
	w.target = id
	return s, nil
}

// FinishMission verbucht das Ergebnis des laufenden Einsatzes. Ein Sieg
// zählt für den Ort, bringt den Spieler dorthin und macht ihn bekannt; was
// er eingesammelt hat, behält der Ort. Ein abgebrochener Einsatz (Result
// Running) ändert nichts.
func (w *World) FinishMission(s *State) {
	id := w.target
	w.target = ""
	place := w.Content.Places[id]
	if place == nil {
		return
	}
	if s.Result == Running {
		w.Message = "Einsatz abgebrochen."
		return
	}
	st, ok := w.Stats[id]
	if !ok {
		st = &PlaceStats{Collected: map[string]int{}}
		w.Stats[id] = st
	}
	if s.Result == Lost {
		st.Losses++
		w.Message = fmt.Sprintf("%s hat dich zurückgeschlagen.", place.Name)
		return
	}
	st.Wins++
	for obj, n := range s.Collected {
		st.Collected[obj] += n
	}
	if w.Known[id] {
		w.Message = fmt.Sprintf("Einsatz in %s erfolgreich.", place.Name)
	} else {
		w.Message = fmt.Sprintf("%s erreicht. Neue Routen öffnen sich.", place.Name)
	}
	w.Known[id] = true
	w.Current = id
}

// ---------- Spielstand ----------

// saveData ist das Format des Spielstands. version erlaubt, alte Stände
// später zu erkennen.
type saveData struct {
	Version int                    `json:"version"`
	Current string                 `json:"current"`
	Known   []string               `json:"known"`
	Stats   map[string]*PlaceStats `json:"stats"`
}

const saveVersion = 1

// Save liefert den Spielstand als JSON.
func (w *World) Save() ([]byte, error) {
	d := saveData{Version: saveVersion, Current: w.Current, Stats: w.Stats}
	for id := range w.Known {
		d.Known = append(d.Known, id)
	}
	sort.Strings(d.Known)
	return json.Marshal(d)
}

// LoadWorld liest einen Spielstand. Orte, die es in den Inhalten nicht mehr
// gibt (z. B. weil ein Mod entfernt wurde), fallen weg; ist auch der
// aktuelle Ort weg, geht es am Startort weiter.
func LoadWorld(c *Content, data []byte) (*World, error) {
	var d saveData
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("Spielstand unlesbar: %w", err)
	}
	if d.Version != saveVersion {
		return nil, fmt.Errorf("Spielstand hat Version %d, erwartet %d", d.Version, saveVersion)
	}
	w := NewWorld(c)
	for _, id := range d.Known {
		if c.Places[id] != nil {
			w.Known[id] = true
		}
	}
	for id, st := range d.Stats {
		if c.Places[id] != nil && st != nil {
			if st.Collected == nil {
				st.Collected = map[string]int{}
			}
			w.Stats[id] = st
		}
	}
	if c.Places[d.Current] != nil && w.Known[d.Current] {
		w.Current = d.Current
	}
	return w, nil
}
