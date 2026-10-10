package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"

	"image/color"
)

func TestRarityColorsExact(t *testing.T) {
	want := map[string]color.RGBA{
		"common":       {R: 0x9d, G: 0x9d, B: 0x9d, A: 255},
		"uncommon":     {R: 0x3b, G: 0xbd, B: 0x40, A: 255},
		"rare":         {R: 0x4b, G: 0x8b, B: 0xe8, A: 255},
		"very rare":    {R: 0x9b, G: 0x59, B: 0xb6, A: 255},
		"legendary":    {R: 0xe8, G: 0xa3, B: 0x3a, A: 255},
		"e-tech":       {R: 0xd9, G: 0x4f, B: 0xff, A: 255},
		"seraph":       {R: 0xff, G: 0x40, B: 0x81, A: 255},
		"pearlescent":  {R: 0x00, G: 0xe5, B: 0xff, A: 255},
		"effervescent": {R: 0xff, G: 0x6e, B: 0xd4, A: 255},
	}
	if len(RarityColors) != len(want) {
		t.Fatalf("want %d keys, got %d", len(want), len(RarityColors))
	}
	for name, wantC := range want {
		got, ok := RarityColors[name]
		if !ok {
			t.Fatalf("missing rarity key %q", name)
		}
		if got != (color.NRGBA{R: wantC.R, G: wantC.G, B: wantC.B, A: 255}) {
			t.Fatalf("rarity %q: want %v, got %v", name, wantC, got)
		}
	}
	// Spot-check the plan's named assertion: legendary = #e8a33a.
	lg := RarityColors["legendary"]
	if lg.R != 232 || lg.G != 163 || lg.B != 58 {
		t.Fatalf("legendary must be #e8a33a (232,163,58), got %v", lg)
	}
}

func TestThemeDarkOnly(t *testing.T) {
	th := NewTheme()
	bg := th.Color(fynetheme.ColorNameBackground, fynetheme.VariantDark)
	nrgba := color.NRGBAModel.Convert(bg).(color.NRGBA)
	if nrgba.R != 0x04 || nrgba.G != 0x06 || nrgba.B != 0x0c {
		t.Fatalf("background want #04060c, got %v", nrgba)
	}
	primary := th.Color(fynetheme.ColorNamePrimary, fynetheme.VariantDark)
	pn := color.NRGBAModel.Convert(primary).(color.NRGBA)
	if pn.R != 0x3e || pn.G != 0xcf || pn.B != 0xff {
		t.Fatalf("primary want #3ecfff, got %v", pn)
	}
	disabled := th.Color(fynetheme.ColorNameDisabled, fynetheme.VariantDark)
	dn := color.NRGBAModel.Convert(disabled).(color.NRGBA)
	if dn == (color.NRGBA{}) {
		t.Fatal("disabled color must be non-zero")
	}
}

func TestFontBundled(t *testing.T) {
	for name, res := range fonts() {
		sr, ok := res.(*fyne.StaticResource)
		if !ok || sr == nil || len(sr.StaticContent) == 0 {
			t.Fatalf("font %s missing content", name)
		}
	}
}

func TestThemeScaleEnlarged(t *testing.T) {
	th := NewTheme()
	// Base sizes (spec) × uiScale 1.5: text 14→21, heading 20→30.
	if got := th.Size(fynetheme.SizeNameText); got != 21 {
		t.Fatalf("text size want 21, got %v", got)
	}
	if got := th.Size(fynetheme.SizeNameHeadingText); got != 30 {
		t.Fatalf("heading size want 30, got %v", got)
	}
	if got := th.Size(fynetheme.SizeNameCaptionText); got != 16.5 {
		t.Fatalf("caption size want 16.5, got %v", got)
	}
}
