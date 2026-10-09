package ui

import (
	"github.com/g3n/engine/gui"
)

// g3n v0.2.0 TabBar notes (module cache gui/tabbar.go):
// - gui.NewTabBar(w, h) + AddTab(text) *Tab; AddTab auto-selects the new tab,
//   so buildTabs re-selects CHARACTER after adding both.
// - Tab content is caller-supplied via tab.SetContent(IPanel); TabBar.recalc
//   sizes and positions it below the headers (no auto-created panels).
// - Clicking a header selects it (onMouseHeader); setSelected toggles content
//   visibility, so unselected tabs cost nothing.
// - Headers carry a close icon that removes the tab (onMouseIcon); editor tabs
//   are pinned to hide it.

// Tab indices for the editor TabBar.
const (
	tabCharacter = iota
	tabInventory
)

// buildTabs returns the editor TabBar with pinned, empty CHARACTER and
// INVENTORY tabs. Callers set each tab's content panel; character.go owns the
// CHARACTER panel, Task 5 owns INVENTORY.
func buildTabs(w, h float32) *gui.TabBar {
	tb := gui.NewTabBar(w, h)
	tb.AddTab("CHARACTER").SetPinned(true)
	tb.AddTab("INVENTORY").SetPinned(true)
	tb.SetSelected(tabCharacter)
	return tb
}
