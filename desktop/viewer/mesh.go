package viewer

import (
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"

	"strconv"
	"strings"

	"bl2save/desktop/native/session"
)

// elementColors maps DetectElement names (internal/assets/assets.go) to
// flat PBR tints.
func elementColors() map[string]string {
	return map[string]string{
		"Fire":      "#ff6600",
		"Shock":     "#0099ff",
		"Corrosive": "#00dd00",
		"Slag":      "#cc00ff",
		"Explosive": "#ffdd00",
	}
}

// parseHexColor parses "#rrggbb" into a g3n color; bad input yields gray.
func parseHexColor(s string) math32.Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return math32.Color{R: 0.6, G: 0.6, B: 0.6}
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return math32.Color{R: 0.6, G: 0.6, B: 0.6}
	}
	return math32.Color{
		R: float32(n>>16&0xff) / 255,
		G: float32(n>>8&0xff) / 255,
		B: float32(n&0xff) / 255,
	}
}

// previewMesh builds the flat-PBR proxy for an item: rarity hex as the
// base Standard color, element hex as emissive. Geometry is a proxy by
// kind (weapons read as box "guns", everything else as compact solids).
// Mask/MIC compositing is explicitly out of scope (slice-1 brief R1).
func previewMesh(view session.ItemView) *graphic.Mesh {
	base := parseHexColor(view.RarityColor)
	mat := material.NewStandard(&base)
	if ec, ok := elementColors()[view.ElementName]; ok {
		em := parseHexColor(ec)
		mat.SetEmissiveColor(&em)
	}
	var mesh *graphic.Mesh
	switch {
	case view.IsWeapon:
		mesh = graphic.NewMesh(geometry.NewBox(0.9, 0.25, 0.25), mat)
	case strings.Contains(view.Category, "Shield"):
		mesh = graphic.NewMesh(geometry.NewBox(0.5, 0.7, 0.12), mat)
	case strings.Contains(view.Category, "Grenade"):
		mesh = graphic.NewMesh(geometry.NewSphere(0.3, 16, 12), mat)
	default:
		mesh = graphic.NewMesh(geometry.NewBox(0.35, 0.35, 0.35), mat)
	}
	return mesh
}
