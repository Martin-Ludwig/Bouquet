package game

import (
	"strings"
	"testing"
)

// worldContent: a - b - c in einer Reihe, d ohne Route.
func worldContent(t *testing.T) *Content {
	c := testContent(t)
	c.Maps["tiny"] = &MapDef{Width: 5, Rows: 5, Base: "plain", Player: PlayerDef{HP: 5, Moves: 2}}
	c.Places = map[string]*PlaceDef{
		"a": {Name: "A", X: 0.1, Y: 0.5, Links: []string{"b"}, Map: "tiny"},
		"b": {Name: "B", X: 0.5, Y: 0.5, Map: "tiny"},
		"c": {Name: "C", X: 0.9, Y: 0.5, Links: []string{"b"}, Map: "default"},
		"d": {Name: "D", X: 0.5, Y: 0.1, Map: "tiny"},
	}
	c.World.Start = "a"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestWorldActions(t *testing.T) {
	w := NewWorld(worldContent(t))
	want := map[string]WorldAction{"a": MissionAction, "b": MissionAction, "c": NoAction, "d": NoAction, "x": NoAction}
	for id, a := range want {
		if got := w.ActionFor(id); got != a {
			t.Errorf("ActionFor(%q) = %v, erwartet %v", id, got, a)
		}
	}
	if !w.Adjacent("b", "a") || !w.Adjacent("b", "c") || w.Adjacent("a", "c") {
		t.Fatal("Routen gelten in beide Richtungen, aber nur direkt")
	}
	if w.Travel("b") {
		t.Fatal("zu einem unbekannten Ort kann man nicht einfach reisen")
	}
	if _, err := w.StartMission("c", 1); err == nil {
		t.Fatal("ein Einsatz ohne Route muss ein Fehler sein")
	}
}

func TestWinUnlocksAndMoves(t *testing.T) {
	w := NewWorld(worldContent(t))
	s, err := w.StartMission("b", 1)
	if err != nil {
		t.Fatal(err)
	}
	s.Result = Won
	s.Collected["colony"] = 2
	w.FinishMission(s)
	if w.Current != "b" || !w.Known["b"] {
		t.Fatalf("nach dem Sieg ist der Spieler in b: %q", w.Current)
	}
	if st := w.StatsFor("b"); st.Wins != 1 || st.Collected["colony"] != 2 {
		t.Fatalf("Bilanz: %+v", st)
	}
	if w.ActionFor("a") != TravelAction || w.ActionFor("c") != MissionAction {
		t.Fatal("zurück nach a reisen, c ist jetzt erreichbar")
	}
	if !w.Travel("a") || w.Current != "a" {
		t.Fatal("Reise zum bekannten Nachbarn")
	}
}

func TestPlacesAreRepeatable(t *testing.T) {
	w := NewWorld(worldContent(t))
	for i := 0; i < 3; i++ {
		s, err := w.StartMission("a", uint64(i))
		if err != nil {
			t.Fatal(err)
		}
		s.Result = Won
		s.Collected["colony"] = 1
		w.FinishMission(s)
	}
	if st := w.StatsFor("a"); st.Wins != 3 || st.Collected["colony"] != 3 || w.Current != "a" {
		t.Fatalf("der eigene Ort ist beliebig oft spielbar: %+v", st)
	}
}

func TestLossAndAbortChangeNothingElse(t *testing.T) {
	w := NewWorld(worldContent(t))
	s, _ := w.StartMission("b", 1)
	s.Result = Lost
	s.Collected["colony"] = 4
	w.FinishMission(s)
	if w.Current != "a" || w.Known["b"] || w.StatsFor("b").Losses != 1 || w.StatsFor("b").Collected["colony"] != 0 {
		t.Fatalf("eine Niederlage zählt nur als Niederlage: %+v", w.StatsFor("b"))
	}
	s, _ = w.StartMission("b", 2)
	w.FinishMission(s) // abgebrochen: Result ist noch Running
	if w.StatsFor("b").Losses != 1 || w.Known["b"] || !strings.Contains(w.Message, "abgebrochen") {
		t.Fatal("ein Abbruch ändert nichts")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	c := worldContent(t)
	w := NewWorld(c)
	s, _ := w.StartMission("b", 1)
	s.Result = Won
	s.Collected["colony"] = 2
	w.FinishMission(s)

	data, err := w.Save()
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadWorld(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != "b" || !got.Known["a"] || !got.Known["b"] || got.StatsFor("b").Collected["colony"] != 2 {
		t.Fatalf("geladen: %+v", got)
	}

	// Ein Ort, den es nicht mehr gibt, fällt weg; es geht am Start weiter.
	delete(c.Places, "b")
	c.Places["c"].Links = nil
	got, err = LoadWorld(c, data)
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != "a" || got.Known["b"] || len(got.Stats) != 0 {
		t.Fatalf("ohne Ort b: %+v", got)
	}
	if _, err := LoadWorld(c, []byte(`{"version":99}`)); err == nil {
		t.Fatal("eine fremde Version muss ein Fehler sein")
	}
	if _, err := LoadWorld(c, []byte(`kaputt`)); err == nil {
		t.Fatal("kaputte Daten müssen ein Fehler sein")
	}
}

func TestPlaceValidation(t *testing.T) {
	c := worldContent(t)
	c.Places["a"].Links = []string{"nowhere"}
	c.Places["b"].Map = "void"
	c.World.Start = "zz"
	err := c.Validate()
	for _, want := range []string{`"nowhere"`, `"void"`, `"zz"`} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("Fehler soll %s nennen: %v", want, err)
		}
	}
}
