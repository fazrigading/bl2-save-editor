package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	fynetheme "fyne.io/fyne/v2/theme"
)

// Theme sizes (spec: heading 20, subheading 15, text 14, caption 11, input 14).
const (
	sizeHeading    = float32(20)
	sizeSubHeading = float32(15)
	sizeText       = float32(14)
	sizeCaption    = float32(11)
	sizeInput      = float32(14)
)

// Palette from static/style.css (source of truth).
var (
	colBackground    = color.NRGBA{R: 0x04, G: 0x06, B: 0x0c, A: 255} // --bg #04060c
	colPanel         = color.NRGBA{R: 0x08, G: 0x0e, B: 0x1c, A: 255} // --bg-panel #080e1c
	colCard          = color.NRGBA{R: 0x0a, G: 0x10, B: 0x1e, A: 255} // --bg-card #0a101e
	colInput         = color.NRGBA{R: 0x04, G: 0x08, B: 0x10, A: 255} // --bg-input #040810
	colCyan          = color.NRGBA{R: 0x3e, G: 0xcf, B: 0xff, A: 255} // --cyan #3ecfff
	colGold          = color.NRGBA{R: 0xe8, G: 0xa3, B: 0x3a, A: 255} // --accent #e8a33a
	colText          = color.NRGBA{R: 0xd4, G: 0xd8, B: 0xe8, A: 255} // --text #d4d8e8
	colTextDim       = color.NRGBA{R: 0x6b, G: 0x7a, B: 0x9e, A: 255} // --text-dim #6b7a9e
	colTextMuted     = color.NRGBA{R: 0x3a, G: 0x46, B: 0x68, A: 255} // --text-muted #3a4668
	colDanger        = color.NRGBA{R: 0xe0, G: 0x40, B: 0x40, A: 255} // --red #e04040
	colSuccess       = color.NRGBA{R: 0x3b, G: 0xbd, B: 0x40, A: 255} // --green #3bbd40
	colDisabledText  = color.NRGBA{R: 0x55, G: 0x60, B: 0x78, A: 255}
	colDisabledInput = color.NRGBA{R: 0x0c, G: 0x12, B: 0x20, A: 255}
)

// RarityColors mirrors --rarity-* exactly; key = lowercased rarity name as
// assets.DetectRarity reports it ("Very Rare" → "very rare").
var RarityColors = map[string]color.NRGBA{
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

// RarityColor resolves a rarity name (case-insensitive) to its color with
// the common gray as fallback.
func RarityColor(name string) color.NRGBA {
	if c, ok := RarityColors[nameLower(name)]; ok {
		return c
	}
	return RarityColors["common"]
}

func nameLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// bl2Theme is the dark-only Borderlands 2 theme.
type bl2Theme struct {
	fyne.Theme
}

// NewTheme returns the BL2 authentic theme (dark-only).
func NewTheme() fyne.Theme {
	return &bl2Theme{Theme: fynetheme.DarkTheme()}
}

func (t *bl2Theme) Color(id fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch id {
	case fynetheme.ColorNameBackground:
		return colBackground
	case fynetheme.ColorNameForeground:
		return colText
	case fynetheme.ColorNamePrimary:
		return colCyan
	case fynetheme.ColorNameHover:
		return colGold
	case fynetheme.ColorNameFocus:
		return colCyan
	case fynetheme.ColorNameSelection:
		return color.NRGBA{R: 0x3e, G: 0xcf, B: 0xff, A: 0x33}
	case fynetheme.ColorNameDisabled:
		return colDisabledText
	case fynetheme.ColorNameDisabledButton:
		return colDisabledInput
	case fynetheme.ColorNameError:
		return colDanger
	case fynetheme.ColorNameSuccess:
		return colSuccess
	case fynetheme.ColorNameWarning:
		return colGold
	case fynetheme.ColorNameInputBorder:
		return colCyan
	case fynetheme.ColorNameButton:
		return colPanel
	case fynetheme.ColorNameMenuBackground:
		return colPanel
	case fynetheme.ColorNameOverlayBackground:
		return colPanel
	case fynetheme.ColorNameShadow:
		return color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x88}
	case fynetheme.ColorNameSeparator:
		return color.NRGBA{R: 0x3e, G: 0xcf, B: 0xff, A: 0x1a}
	case fynetheme.ColorNamePlaceHolder:
		return colTextMuted
	default:
		return t.Theme.Color(id, fynetheme.VariantDark)
	}
}

func (t *bl2Theme) Font(style fyne.TextStyle) fyne.Resource {
	if style.Bold || style.Monospace {
		return resourceOswaldTtf
	}
	return resourceRajdhaniTtf
}

// uiScale enlarges every theme size (text, padding, icons) — widget
// min-sizes derive from these, so the whole UI grows proportionally.
// System-level scaling stays Fyne's job (FYNE_SCALE env).
const uiScale = float32(1.5)

func (t *bl2Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case fynetheme.SizeNameHeadingText:
		return sizeHeading * uiScale
	case fynetheme.SizeNameSubHeadingText:
		return sizeSubHeading * uiScale
	case fynetheme.SizeNameText:
		return sizeText * uiScale
	case fynetheme.SizeNameCaptionText:
		return sizeCaption * uiScale
	case fynetheme.SizeNamePadding:
		return 6 * uiScale
	case fynetheme.SizeNameInlineIcon:
		return 20 * uiScale
	default:
		return t.Theme.Size(name) * uiScale
	}
}

// Icon serves the bundled static/icons copies through theme.Icon.
func (t *bl2Theme) Icon(name fyne.ThemeIconName) fyne.Resource {
	switch name {
	case fynetheme.IconNameHome:
		return resourceVaultsymbolPng
	default:
		return t.Theme.Icon(name)
	}
}

// tabIcon returns the bundled icon for a tab label.
func tabIcon(label string) fyne.Resource {
	switch label {
	case "INVENTORY":
		return resourceBackpackPng
	case "BANK":
		return resourceBankPng
	case "WEAPONS":
		return resourceGunsPng
	default:
		return resourceVaultsymbolPng
	}
}
