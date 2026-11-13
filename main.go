package main

import (
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/content"
	"bouquet/game"
	"bouquet/mods"
	"bouquet/view"
)

// screen ist, was gerade zu sehen ist.
type screen int

const (
	menuScreen    screen = iota // Hauptmenü: Modus wählen
	worldScreen                 // Weltkarte der Kampagne
	missionScreen               // ein Einsatz (aus der Kampagne oder "Überleben")
	duelScreen                  // ein Duell, mit Klassenwahl davor
)

// app ist das laufende Spiel.
type app struct {
	content  *game.Content
	world    *game.World
	board    *view.Board
	worldMap *view.WorldMap
	screen   screen

	mission    *game.State
	missionMap string // Kartenrezept des Einsatzes, für einen Neustart
	target     string // Ort des Einsatzes auf der Weltkarte; leer: "Überleben" aus dem Menü

	duel *duelGame
}

func main() {
	rl.SetConfigFlags(rl.FlagVsyncHint | rl.FlagWindowResizable)
	rl.InitWindow(960, 720, "Bouquet")
	rl.SetTargetFPS(60)
	rl.SetExitKey(0) // Esc führt zurück, beendet aber nicht das Spiel

	a, err := load()
	update := func() {
		w, h := float64(rl.GetScreenWidth()), float64(rl.GetScreenHeight())
		rl.BeginDrawing()
		rl.ClearBackground(view.Paper)
		switch {
		case err != nil:
			// Fehler in den Inhalten (z. B. ein Tippfehler in einem Mod)
			// werden angezeigt statt das Spiel abstürzen zu lassen.
			rl.DrawText("Inhalte konnten nicht geladen werden:", 16, 16, 22, view.Ink)
			rl.DrawText(err.Error(), 16, 52, 18, view.Ink)
		case w <= 0 || h <= 0:
		default:
			err = a.update(w, h)
		}
		rl.EndDrawing()
	}
	// Die JS-Laufzeit ruft update() in jedem Frame auf; SetMain kehrt nie zurück.
	// (Die Anbindung gibt es nur für wasm, siehe replace in go.mod.)
	rl.SetMain(update)
}

// load liest die Inhalte aller Mods und den Spielstand.
func load() (*app, error) {
	c, _, err := mods.Load(content.FS)
	if err != nil {
		return nil, err
	}
	board, err := view.NewBoard(c)
	if err != nil {
		return nil, err
	}
	a := &app{content: c, board: board, worldMap: view.NewWorldMap(c, content.FS)}
	if data := readSave(); data != nil {
		// Ein unlesbarer Spielstand ist kein Grund, nicht zu starten.
		a.world, _ = game.LoadWorld(c, data)
	}
	if a.world == nil {
		a.world = game.NewWorld(c)
	}
	return a, nil
}

func (a *app) save() {
	if data, err := a.world.Save(); err == nil {
		writeSave(data)
	}
}

func seed() uint64 { return uint64(time.Now().UnixNano()) }

// update ist ein Frame.
func (a *app) update(w, h float64) error {
	switch a.screen {
	case worldScreen:
		return a.updateWorld(w, h)
	case missionScreen:
		return a.updateMission(w, h)
	case duelScreen:
		return a.duel.update(a, w, h)
	default:
		return a.updateMenu(w, h)
	}
}

// ---------- Hauptmenü ----------

// menuEntry ist ein Eintrag im Hauptmenü und was er startet.
type menuEntry struct {
	item  view.MenuItem
	start func() error
}

// menuEntries listet die Modi, die die Inhalte hergeben.
func (a *app) menuEntries() []menuEntry {
	entries := []menuEntry{{
		view.MenuItem{Label: "Kampagne", Note: "Weltkarte: reisen, Orte erkunden, Spielstand bleibt"},
		func() error { a.screen = worldScreen; return nil },
	}}
	if id := a.content.World.Survival; id != "" {
		name := a.content.Maps[id].Name
		entries = append(entries, menuEntry{
			view.MenuItem{Label: "Überleben", Note: name + ": bis zum Extraktionspunkt, der Sturm im Nacken"},
			func() error { return a.startMission(id, "") },
		})
	}
	if len(a.content.Classes) > 0 && len(a.content.Arenas) > 0 {
		entries = append(entries,
			menuEntry{view.MenuItem{Label: "Duell gegen KI", Note: "Held gegen Held auf einer gespiegelten Karte"},
				func() error { a.duel, a.screen = newDuelGame([2]bool{false, true}), duelScreen; return nil }},
			menuEntry{view.MenuItem{Label: "Duell zu zweit", Note: "an einem Gerät, abwechselnd"},
				func() error { a.duel, a.screen = newDuelGame([2]bool{}), duelScreen; return nil }},
		)
	}
	return entries
}

func (a *app) updateMenu(w, h float64) error {
	entries := a.menuEntries()
	items := make([]view.MenuItem, len(entries))
	for i, e := range entries {
		items[i] = e.item
	}
	if i := view.Menu("Bouquet", items, int32(w), int32(h)); i >= 0 {
		return entries[i].start()
	}
	return nil
}

// ---------- Weltkarte ----------

func (a *app) updateWorld(w, h float64) error {
	if rl.IsKeyPressed(rl.KeyEscape) {
		a.screen = menuScreen
		return nil
	}
	hovered := a.worldMap.PlaceAt(a.world, w, h, rl.GetMousePosition())
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		switch a.world.ActionFor(hovered) {
		case game.TravelAction:
			a.world.Travel(hovered)
			a.save()
		case game.MissionAction:
			return a.startMission(a.content.Places[hovered].Map, hovered)
		}
	}
	a.worldMap.Draw(a.world, w, h, hovered)
	view.DrawWorldHud(a.world, int32(w), int32(h), hovered)
	return nil
}

// ---------- Einsatz ----------

// startMission beginnt einen Einsatz nach dem Kartenrezept mapID. target ist
// der Ort auf der Weltkarte, leer für "Überleben" aus dem Menü.
func (a *app) startMission(mapID, target string) error {
	var s *game.State
	var err error
	if target != "" {
		s, err = a.world.StartMission(target, seed())
	} else {
		s, err = game.New(a.content, mapID, seed())
	}
	if err != nil {
		return err
	}
	a.mission, a.missionMap, a.target, a.screen = s, mapID, target, missionScreen
	return nil
}

// finishMission verbucht den Einsatz (nur in der Kampagne) und führt
// zurück: zur Weltkarte oder ins Menü.
func (a *app) finishMission() {
	a.screen = menuScreen
	if a.target != "" {
		a.world.FinishMission(a.mission)
		a.save()
		a.screen = worldScreen
	}
	a.mission = nil
}

func (a *app) updateMission(w, h float64) error {
	s := a.mission
	layout := view.Layout(s, w, h)
	mouse := rl.GetMousePosition()
	hovered := layout.At(float64(mouse.X), float64(mouse.Y))
	click := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	switch {
	case rl.IsKeyPressed(rl.KeyR):
		// Neu: Der laufende Einsatz zählt als abgebrochen, ein beendeter
		// wird verbucht.
		mapID, target := a.missionMap, a.target
		a.finishMission()
		return a.startMission(mapID, target)
	case rl.IsKeyPressed(rl.KeyEscape), s.Result != game.Running && click:
		a.finishMission()
		return nil
	case rl.IsKeyPressed(rl.KeySpace):
		s.EndTurn() // stehen bleiben und abwarten
	case click:
		s.Click(hovered)
	}
	title := "Überleben: " + a.content.Maps[a.missionMap].Name
	if a.target != "" {
		title = a.content.Places[a.target].Name
	}
	a.board.Draw(s, layout, hovered)
	view.DrawHud(s, title, int32(w), int32(h))
	return nil
}
