// Package mods lädt Spielinhalte aus Mod-Ordnern.
//
// Ein Mod ist ein Ordner mit einer mod.json und beliebig vielen dieser
// Dateien:
//
//	terrain.json   Geländearten
//	enemies.json   Gegnertypen
//	objects.json   Objekte auf der Karte (Kolonien, ...)
//	maps.json      Kartenrezepte
//	places.json    Orte der Weltkarte
//	classes.json   Heldenklassen im Duell
//	arenas.json    Duellkarten
//	world.json     Einstellungen der Weltkarte (ein einzelnes Objekt, keine IDs)
//
// Jede Datei ist ein JSON-Objekt von ID auf Eintrag. Mods werden
// nacheinander geladen, "base" zuerst, die übrigen alphabetisch. Nennt ein
// späterer Mod eine ID, die es schon gibt, überschreibt er nur die Felder,
// die er angibt; neue IDs kommen dazu.
package mods

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"

	"bouquet/game"
)

// BaseID ist der Mod, der immer zuerst geladen wird.
const BaseID = "base"

// Info ist der Inhalt einer mod.json.
type Info struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Load liest alle Mod-Ordner in fsys und liefert die zusammengeführten,
// geprüften Inhalte sowie die geladenen Mods in Ladereihenfolge.
func Load(fsys fs.FS) (*game.Content, []Info, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, nil, err
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool {
		if (dirs[i] == BaseID) != (dirs[j] == BaseID) {
			return dirs[i] == BaseID
		}
		return dirs[i] < dirs[j]
	})

	c := &game.Content{
		Terrain: map[string]*game.TerrainDef{},
		Enemies: map[string]*game.EnemyDef{},
		Objects: map[string]*game.ObjectDef{},
		Maps:    map[string]*game.MapDef{},
		Places:  map[string]*game.PlaceDef{},
		Classes: map[string]*game.ClassDef{},
		Arenas:  map[string]*game.ArenaDef{},
	}
	var infos []Info
	for _, dir := range dirs {
		var info Info
		if err := readJSON(fsys, path.Join(dir, "mod.json"), &info); err != nil {
			return nil, nil, err
		}
		if info.ID != dir {
			return nil, nil, fmt.Errorf("%s/mod.json: id %q muss dem Ordnernamen entsprechen", dir, info.ID)
		}
		infos = append(infos, info)

		err := errors.Join(
			merge(fsys, path.Join(dir, "terrain.json"), c.Terrain),
			merge(fsys, path.Join(dir, "enemies.json"), c.Enemies),
			merge(fsys, path.Join(dir, "objects.json"), c.Objects),
			merge(fsys, path.Join(dir, "maps.json"), c.Maps),
			merge(fsys, path.Join(dir, "places.json"), c.Places),
			merge(fsys, path.Join(dir, "classes.json"), c.Classes),
			merge(fsys, path.Join(dir, "arenas.json"), c.Arenas),
		)
		if err != nil {
			return nil, nil, err
		}

		// world.json: Nennt der Mod ein Hintergrundbild, liegt es in seinem
		// Ordner.
		before := c.World.Background
		c.World.Background = ""
		if err := readJSON(fsys, path.Join(dir, "world.json"), &c.World); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, nil, err
		}
		if c.World.Background == "" {
			c.World.Background = before
		} else {
			c.World.Background = path.Join(dir, c.World.Background)
		}
	}
	if err := c.Validate(); err != nil {
		return nil, nil, err
	}
	return c, infos, nil
}

// merge liest eine Inhaltsdatei in into ein. Eine fehlende Datei ist kein
// Fehler: Ein Mod muss nicht jede Art von Inhalt mitbringen.
func merge[T any](fsys fs.FS, name string, into map[string]*T) error {
	var raw map[string]json.RawMessage
	if err := readJSON(fsys, name, &raw); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for id, data := range raw {
		entry, ok := into[id]
		if !ok {
			entry = new(T)
			into[id] = entry
		}
		// In einen vorhandenen Eintrag hinein gelesen bleiben alle Felder
		// stehen, die der Mod nicht nennt.
		if err := decodeStrict(data, entry); err != nil {
			return fmt.Errorf("%s, Eintrag %q: %w", name, id, err)
		}
	}
	return nil
}

func readJSON(fsys fs.FS, name string, into any) error {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return err
	}
	if err := decodeStrict(data, into); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

// decodeStrict lehnt unbekannte Felder ab, damit Tippfehler in Mod-Dateien
// auffallen statt still ignoriert zu werden.
func decodeStrict(data []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
