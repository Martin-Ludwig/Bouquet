package mods

import (
	"strings"
	"testing"
	"testing/fstest"

	"bouquet/content"
	"bouquet/game"
)

func TestShippedContentLoadsAndPlays(t *testing.T) {
	c, infos, err := Load(content.FS)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) == 0 || infos[0].ID != BaseID {
		t.Fatalf("base muss zuerst geladen werden: %v", infos)
	}
	if len(c.Places) == 0 || c.Places[c.World.Start] == nil || c.World.Background != "base/worldmap.jpg" {
		t.Fatalf("Weltkarte fehlt: %d Orte, Start %q, Bild %q", len(c.Places), c.World.Start, c.World.Background)
	}
	if _, err := content.FS.Open(c.World.Background); err != nil {
		t.Fatalf("Hintergrundbild fehlt: %v", err)
	}
	// Jeder Ort ist vom Start aus über Routen erreichbar.
	w := game.NewWorld(c)
	reached := map[string]bool{c.World.Start: true}
	for changed := true; changed; {
		changed = false
		for a := range c.Places {
			for b := range c.Places {
				if reached[a] && !reached[b] && w.Adjacent(a, b) {
					reached[b], changed = true, true
				}
			}
		}
	}
	if len(reached) != len(c.Places) {
		t.Fatalf("nur %d von %d Orten erreichbar", len(reached), len(c.Places))
	}
	for id := range c.Maps {
		for seed := uint64(1); seed <= 100; seed++ {
			s, err := game.New(c, id, seed)
			if err != nil {
				t.Fatal(err)
			}
			if !s.Passable(s.Player) || !s.Passable(s.Goal) {
				t.Fatalf("Karte %q, seed %d: Start oder Ziel nicht begehbar", id, seed)
			}
			if d := s.FieldTo(s.Goal)[s.Grid.Index(s.Player)]; d < 0 {
				t.Fatalf("Karte %q, seed %d: Ziel unerreichbar", id, seed)
			}
		}
	}
}

func file(s string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(s)} }

func baseFS() fstest.MapFS {
	return fstest.MapFS{
		"base/mod.json":     file(`{"id":"base","name":"Basis","version":"1"}`),
		"base/terrain.json": file(`{"plain":{"name":"Ebene","passable":true,"fill":"#ffffff"}}`),
		"base/enemies.json": file(`{"hunter":{"name":"Jagdbot","letter":"J","hp":3,"speed":1,"behavior":"chase","params":{"aggro":8},"color":"#000000","radius":0.6}}`),
		"base/maps.json":    file(`{"default":{"name":"Test","width":5,"rows":5,"base":"plain","player":{"hp":5,"moves":2},"enemies":[{"type":"hunter","count":1}]}}`),
	}
}

func TestLaterModPatchesAndAdds(t *testing.T) {
	fsys := baseFS()
	fsys["zz-hart/mod.json"] = file(`{"id":"zz-hart","name":"Hart","version":"1"}`)
	fsys["zz-hart/enemies.json"] = file(`{
		"hunter": {"hp": 9, "params": {"wander": 0.5}},
		"drone":  {"name":"Drohne","letter":"D","hp":1,"speed":2,"behavior":"wander","color":"#111111","radius":0.4}
	}`)

	c, infos, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 2 || infos[1].ID != "zz-hart" {
		t.Fatalf("Ladereihenfolge: %v", infos)
	}
	h := c.Enemies["hunter"]
	if h.HP != 9 || h.Name != "Jagdbot" || h.Speed != 1 {
		t.Fatalf("Patch soll nur hp ändern: %+v", h)
	}
	if h.Param("aggro", 0) != 8 || h.Param("wander", 0) != 0.5 {
		t.Fatalf("params sollen zusammengeführt werden: %v", h.Params)
	}
	if d := c.Enemies["drone"]; d == nil || d.ID != "drone" {
		t.Fatal("neuer Gegnertyp fehlt")
	}
}

func TestWorldSettingsMerge(t *testing.T) {
	fsys := baseFS()
	fsys["base/places.json"] = file(`{"a":{"name":"A","x":0.1,"y":0.1,"map":"default"}}`)
	fsys["base/world.json"] = file(`{"start":"a","background":"karte.jpg"}`)
	fsys["zz/mod.json"] = file(`{"id":"zz"}`)
	fsys["zz/places.json"] = file(`{"b":{"name":"B","x":0.9,"y":0.9,"map":"default","links":["a"]}}`)
	fsys["zz/world.json"] = file(`{"start":"b"}`)
	c, _, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	if c.World.Start != "b" || c.World.Background != "base/karte.jpg" || len(c.Places) != 2 {
		t.Fatalf("Start überschrieben, Bild behalten, Ort dazu: %+v", c.World)
	}
}

func TestErrorsNameTheProblem(t *testing.T) {
	cases := map[string]struct {
		path, data, want string
	}{
		"Tippfehler im Feldnamen": {"base/enemies.json", `{"hunter":{"hitpoints":3}}`, "hitpoints"},
		"unbekanntes Verhalten":   {"base/enemies.json", `{"hunter":{"behavior":"fly"}}`, `"fly"`},
		"unbekanntes Gelände":     {"base/maps.json", `{"default":{"base":"lava"}}`, `"lava"`},
		"falsche Mod-ID":          {"base/mod.json", `{"id":"other"}`, "Ordnernamen"},
	}
	for name, tc := range cases {
		fsys := baseFS()
		if tc.path == "base/mod.json" {
			fsys[tc.path] = file(tc.data)
		} else {
			fsys["zz/mod.json"] = file(`{"id":"zz"}`)
			fsys["zz/"+strings.TrimPrefix(tc.path, "base/")] = file(tc.data)
		}
		_, _, err := Load(fsys)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: Fehler soll %q nennen, war: %v", name, tc.want, err)
		}
	}
}
