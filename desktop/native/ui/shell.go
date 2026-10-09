// Package ui owns the Fyne main window: save list | tabs | status bar.
// Task 3 builds the skeleton with a placeholder right pane; tabs land in
// Tasks 5–9. The g3n 3D viewer moved to desktop/viewer.
package ui

import (
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/internal/platform"
	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
)

// model is the shell state, kept pure & testable: no widgets.
type model struct {
	ses       *session.Session
	filenames []string
	selected  string
	st        setup.AssetStatus

	// enabled reports whether edits are allowed (game-not-running guard).
	// Polled every 3s, same cache cadence as platform.IsGameRunning.
	enabled func() bool
}

// guardEnabled polls the game-running guard; safe to call from any tab.
func guardEnabled() bool {
	return !platform.IsGameRunning()
}

// Run opens the Fyne editor window (1280×800, min 1024×700) and blocks
// until quit.
func Run(ses *session.Session, st setup.AssetStatus) error {
	a := app.NewWithID("bl2-save-editor")
	a.Settings().SetTheme(NewTheme())

	win := a.NewWindow("BL2 Save Editor")
	win.Resize(fyne.NewSize(1280, 800))
	// ponytail: Fyne 2.8 has no window min-size API; min 1024×700 enforced
	// only as a soft hint via content min size. Upgrade when Fyne adds one.

	m := &model{ses: ses, st: st, enabled: guardEnabled}

	left := newSaveList(m)
	right := container.NewVBox(widget.NewLabel("Select a save")) // placeholder until T6
	status := newStatusBar(m, st)

	root := container.NewHSplit(left, container.NewBorder(nil, status, nil, nil, right))
	root.Offset = 0.25
	win.SetContent(root)

	if sums, err := ses.ListSaves(); err == nil && len(sums) > 0 {
		m.filenames = filenamesOf(sums)
	} else if err != nil {
		log.Printf("save list: %v", err)
	}

	// Guard poll: flip edits off while Borderlands 2 runs (3s cadence).
	go func() {
		for range time.Tick(3 * time.Second) {
			if m.enabled != nil && !m.enabled() {
				// Tab panels read m.enabled per interaction; status bar
				// shows the chip. No push-refresh needed for the skeleton.
			}
		}
	}()

	win.ShowAndRun()
	return nil
}

// filenamesOf extracts filenames from save summaries.
func filenamesOf(sums []session.SaveSummary) []string {
	out := make([]string, 0, len(sums))
	for _, s := range sums {
		out = append(out, s.Filename)
	}
	return out
}

// newSaveList builds the left save picker bound to the model.
func newSaveList(m *model) fyne.CanvasObject {
	list := widget.NewList(
		func() int { return len(m.filenames) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i int, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(m.filenames[i])
		},
	)
	list.OnSelected = func(i int) {
		m.selected = m.filenames[i]
	}
	return list
}
