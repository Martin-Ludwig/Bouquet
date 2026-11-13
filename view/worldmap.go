package view

import (
	"fmt"
	"io/fs"
	"math"
	"path"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"

	"bouquet/game"
)

var (
	placeKnown   = rl.NewColor(58, 124, 190, 255)
	placeUnknown = rl.NewColor(122, 116, 104, 255)
	routeIdle    = rl.NewColor(60, 40, 20, 110)
	routeActive  = rl.NewColor(36, 100, 170, 230)
	labelHalo    = rl.NewColor(255, 250, 235, 230)
)

// WorldMap zeichnet die Weltkarte: Hintergrundbild, Routen und Orte.
type WorldMap struct {
	tex    rl.Texture2D
	hasTex bool
}

// NewWorldMap lädt das Hintergrundbild aus fsys, falls die Inhalte eines
// nennen. Fehlt es oder lässt es sich nicht lesen, bleibt der Hintergrund
// leer; das ist kein Fehler. Aufrufen nach rl.InitWindow.
func NewWorldMap(c *game.Content, fsys fs.FS) *WorldMap {
	m := &WorldMap{}
	if c.World.Background == "" {
		return m
	}
	data, err := fs.ReadFile(fsys, c.World.Background)
	if err != nil {
		return m
	}
	img := rl.LoadImageFromMemory(path.Ext(c.World.Background), data, int32(len(data)))
	// Gültigkeit selbst prüfen: rl.IsImageValid stürzt in dieser
	// Anbindung ab.
	if img.Width <= 0 || img.Height <= 0 {
		return m
	}
	// Das Bild bleibt im Speicher liegen: rl.UnloadImage stürzt in dieser
	// Anbindung ebenfalls ab. Es wird nur einmal geladen.
	m.tex = rl.LoadTextureFromImage(img)
	if m.hasTex = m.tex.ID != 0; m.hasTex {
		rl.SetTextureFilter(m.tex, rl.FilterBilinear)
	}
	return m
}

// frame ist das Rechteck, in dem die Karte im Fenster liegt: so groß wie
// möglich zwischen Kopf- und Fußzeile, im Seitenverhältnis des Bildes.
func (m *WorldMap) frame(w, h float64) (x, y, fw, fh float64) {
	ratio := 16.0 / 9.0
	if m.hasTex {
		ratio = float64(m.tex.Width) / float64(m.tex.Height)
	}
	availH := h - HudHeight - FootHeight
	fw = math.Min(w, availH*ratio)
	fh = fw / ratio
	return (w - fw) / 2, HudHeight + (availH-fh)/2, fw, fh
}

// spot ist die Bildschirmposition eines Ortes und sein Radius.
func (m *WorldMap) spot(p *game.PlaceDef, w, h float64) (pos rl.Vector2, radius float32) {
	x, y, fw, fh := m.frame(w, h)
	k := math.Max(0.6, math.Min(1, fw/1400)) // kleine Fenster: kleinere Orte
	return rl.NewVector2(float32(x+p.X*fw), float32(y+p.Y*fh)), float32(14 * k)
}

// PlaceAt liefert die ID des Ortes unter der Maus, oder "".
func (m *WorldMap) PlaceAt(world *game.World, w, h float64, mouse rl.Vector2) string {
	for id, p := range world.Content.Places {
		pos, r := m.spot(p, w, h)
		if math.Hypot(float64(mouse.X-pos.X), float64(mouse.Y-pos.Y)) <= float64(r)+4 {
			return id
		}
	}
	return ""
}

// Draw zeichnet die Weltkarte in ein Fenster der Größe w x h. hovered ist
// die ID des Ortes unter der Maus, oder "".
func (m *WorldMap) Draw(world *game.World, w, h float64, hovered string) {
	x, y, fw, fh := m.frame(w, h)
	if m.hasTex {
		src := rl.NewRectangle(0, 0, float32(m.tex.Width), float32(m.tex.Height))
		dst := rl.NewRectangle(float32(x), float32(y), float32(fw), float32(fh))
		rl.DrawTexturePro(m.tex, src, dst, rl.NewVector2(0, 0), 0, white)
	}

	// Feste Reihenfolge, damit sich überlappende Orte nicht flackern.
	places := world.Content.Places
	ids := make([]string, 0, len(places))
	for id := range places {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// Routen: jede einmal; die vom aktuellen Ort aus kräftig.
	for i, a := range ids {
		for _, b := range ids[i+1:] {
			if !world.Adjacent(a, b) {
				continue
			}
			pa, _ := m.spot(places[a], w, h)
			pb, _ := m.spot(places[b], w, h)
			if a == world.Current || b == world.Current {
				rl.DrawLineEx(pa, pb, 4, routeActive)
			} else {
				rl.DrawLineEx(pa, pb, 2, routeIdle)
			}
		}
	}

	for _, id := range ids {
		p := places[id]
		pos, r := m.spot(p, w, h)
		fill := placeUnknown
		if world.Known[id] {
			fill = placeKnown
		}
		rim := Ink
		if !world.Known[id] && world.ActionFor(id) == game.MissionAction {
			rim = goal // von hier aus erreichbar
		}
		rl.DrawCircleV(pos, r+3, rim)
		rl.DrawCircleV(pos, r, fill)
		if id == world.Current {
			rl.DrawRing(pos, r+6, r+9, 0, 360, 36, routeActive)
		}
		if id == hovered && world.ActionFor(id) != game.NoAction {
			rl.DrawRing(pos, r+11, r+13, 0, 360, 36, hover)
		}

		size := int32(math.Max(12, float64(r)*1.15))
		tw := rl.MeasureText(p.Name, size)
		tx, ty := int32(pos.X)-tw/2, int32(pos.Y+r)+6
		for _, d := range [][2]int32{{-2, 0}, {2, 0}, {0, -2}, {0, 2}} {
			rl.DrawText(p.Name, tx+d[0], ty+d[1], size, labelHalo)
		}
		rl.DrawText(p.Name, tx, ty, size, Ink)
	}
}

// DrawWorldHud zeichnet Kopf- und Fußzeile der Weltkarte: wo der Spieler
// ist, und was ein Klick auf den Ort unter der Maus bewirkt.
func DrawWorldHud(world *game.World, w, h int32, hovered string) {
	rl.DrawText(world.Place().Name, 16, 12, 22, Ink)
	known := fmt.Sprintf("Orte %d / %d", len(world.Known), len(world.Content.Places))
	rl.DrawText(known, w-rl.MeasureText(known, 18)-16, 14, 18, Ink)

	msg := world.Message
	if msg == "" {
		msg = "Klicke deinen Ort für einen Einsatz oder einen Nachbarort."
	}
	if p := world.Content.Places[hovered]; p != nil {
		msg = p.Name + ": " + describe(world, hovered)
	}
	rl.DrawText(msg, 16, h-int32(FootHeight)+8, 20, Ink)
}

func describe(world *game.World, id string) string {
	st := world.StatsFor(id)
	record := ""
	if st.Wins+st.Losses > 0 {
		record = fmt.Sprintf(" (%d gewonnen, %d verloren)", st.Wins, st.Losses)
	}
	switch world.ActionFor(id) {
	case game.TravelAction:
		return "klicken zum Reisen" + record
	case game.MissionAction:
		if id == world.Current {
			return "du bist hier, klicken für einen Einsatz im Umland" + record
		}
		return "unbekannt, klicken für den Einsatz dorthin" + record
	}
	return "keine Route von hier" + record
}
