// Package ui — guard helpers. The game-running guard disables every
// user-editable control (Entries, Selects, Buttons, Checks) across all
// panels so in-game state is never raced from the UI. Session methods
// re-check IsGameRunning server-side; this is the affordance layer.
package ui

import "fyne.io/fyne/v2"

// setTreeDisabled walks the tree and disables (or re-enables) every
// Disableable control.
func setTreeDisabled(obj fyne.CanvasObject, dis bool) {
	walkTree(obj, func(n fyne.CanvasObject) {
		d, ok := n.(fyne.Disableable)
		if !ok {
			return
		}
		if dis {
			d.Disable()
		} else {
			d.Enable()
		}
	})
}

// allDisabled reports whether every Disableable control in the tree is
// disabled. A tree with no interactive control is vacuously disabled.
func allDisabled(obj fyne.CanvasObject) bool {
	ok := true
	walkTree(obj, func(n fyne.CanvasObject) {
		if d, is := n.(fyne.Disableable); is && !d.Disabled() {
			ok = false
		}
	})
	return ok
}
