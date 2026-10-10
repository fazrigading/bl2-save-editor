package ui

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/internal/platform"
	"bl2save/desktop/native/setup"
)

// time3s ticks every 3 seconds, matching the IsGameRunning cache window.
func time3s() <-chan time.Time {
	return time.Tick(3 * time.Second)
}

// newStatusBar builds the bottom status bar: asset dir · game-guard chip ·
// last status line, and returns a setter for the status line. The guard chip
// polls IsGameRunning every 3s (same cadence as app.py:_is_game_running).
func newStatusBar(m *model, st setup.AssetStatus) (fyne.CanvasObject, func(string)) {
	assets := widget.NewLabel("assets: " + st.Dir)
	assets.TextStyle = fyne.TextStyle{Monospace: true}

	guard := widget.NewLabel("game: ok")
	guard.TextStyle = fyne.TextStyle{Bold: true}

	status := widget.NewLabel("Ready.")
	status.Wrapping = fyne.TextTruncate

	go func() {
		for range time3s() {
			if platform.IsGameRunning() {
				guard.SetText("game: RUNNING — edits blocked")
				guard.Importance = widget.HighImportance
			} else {
				guard.SetText("game: ok")
				guard.Importance = widget.MediumImportance
			}
			guard.Refresh()
		}
	}()

	setLine := func(s string) { status.SetText(s) }
	return container.NewHBox(assets, guard, status), setLine
}
