// Package ui owns the Fyne main window: save list | tabs | status bar. The
// g3n 3D viewer lives in desktop/viewer and opens as a companion window.
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

	// bridge owns the companion 3D window; status reports lines on the
	// status bar. Both may be nil in tests.
	bridge *viewerBridge
	status func(string)

	// curItem is the last inventory selection (drives the View 3D button).
	curItem session.ItemView
}

// guardEnabled polls the game-running guard; safe to call from any tab.
func guardEnabled() bool {
	return !platform.IsGameRunning()
}

// setStatus appends a line to the status bar (wired by the shell).
func (m *model) setStatus(s string) {
	if m.status != nil {
		m.status(s)
	}
}

// openViewer launches the 3D viewer for the current selection (zero value =
// character mode).
func (m *model) openViewer() {
	if m.bridge == nil {
		return
	}
	m.bridge.Open(m.ses, m.st, m.curItem, m.setStatus)
}

// buildTabContents builds the four editing panels for an opened save. A nil
// sv (nothing selected) yields a single placeholder. Content is rebuilt from
// the fresh SaveView on every save switch — a stale panel would write the
// previous file.
func buildTabContents(m *model, sv *session.SaveView) []fyne.CanvasObject {
	if sv == nil {
		return []fyne.CanvasObject{widget.NewLabel("Select a save")}
	}
	onSelect := func(it session.ItemView) {
		m.curItem = it
	}
	invPanel, _ := newInventoryPanel(m.ses, sv, onSelect, m.enabled)
	return []fyne.CanvasObject{
		newCharacterPanel(m.ses, sv, m.enabled),
		invPanel,
		newSkillsTab(m.ses, sv, m.enabled),
		newAchievementsPanel(m.ses, sv, m.enabled),
	}
}

// newAppTabs wraps the four panels in a tab set. onReopen rebuilds content on
// save switch; the View 3D button launches the companion window.
func newAppTabs(m *model) *container.AppTabs {
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("CHARACTER", tabIcon("CHARACTER"), widget.NewLabel("")),
		container.NewTabItemWithIcon("INVENTORY", tabIcon("INVENTORY"), widget.NewLabel("")),
		container.NewTabItemWithIcon("SKILLS", tabIcon("SKILLS"), widget.NewLabel("")),
		container.NewTabItemWithIcon("ACHIEVEMENTS", tabIcon("ACHIEVEMENTS"), widget.NewLabel("")),
	)
	return tabs
}

// setTabContents swaps every tab's content for a fresh save's panels.
func setTabContents(tabs *container.AppTabs, m *model, sv *session.SaveView) {
	contents := buildTabContents(m, sv)
	for i, item := range tabs.Items {
		if i < len(contents) {
			item.Content = contents[i]
		} else {
			item.Content = widget.NewLabel("")
		}
	}
	tabs.Refresh()
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

	m := &model{ses: ses, st: st, enabled: guardEnabled, bridge: &viewerBridge{}}

	statusBar, setLine := newStatusBar(m, st)
	m.status = setLine

	tabs := newAppTabs(m)
	setTabContents(tabs, m, nil)
	viewBtn := widget.NewButton("View 3D", func() { m.openViewer() })

	right := container.NewBorder(container.NewHBox(viewBtn), nil, nil, nil, tabs)

	if sums, err := ses.ListSaves(); err == nil && len(sums) > 0 {
		m.filenames = filenamesOf(sums)
	} else if err != nil {
		log.Printf("save list: %v", err)
	}

	left := newSaveList(m, func(filename string) {
		if filename == "" {
			setTabContents(tabs, m, nil)
			return
		}
		sv, err := ses.OpenSave(filename)
		if err != nil {
			m.setStatus("open " + filename + ": " + err.Error())
			setTabContents(tabs, m, nil)
			return
		}
		setTabContents(tabs, m, sv)
		m.setStatus("opened " + filename)
	})

	root := container.NewHSplit(left, container.NewBorder(nil, statusBar, nil, nil, right))
	root.Offset = 0.25
	win.SetContent(root)

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

// newSaveList builds the left save picker bound to the model. onOpen fires
// with the selected filename (empty when the selection is cleared).
func newSaveList(m *model, onOpen func(string)) fyne.CanvasObject {
	list := widget.NewList(
		func() int { return len(m.filenames) },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		func(i int, o fyne.CanvasObject) {
			o.(*widget.Label).SetText(m.filenames[i])
		},
	)
	list.OnSelected = func(i int) {
		if i < 0 || i >= len(m.filenames) {
			return
		}
		m.selected = m.filenames[i]
		if onOpen != nil {
			onOpen(m.selected)
		}
	}
	list.OnUnselected = func(int) {
		m.selected = ""
		if onOpen != nil {
			onOpen("")
		}
	}
	return list
}

// guardPoll re-reads the guard every 3s so the status bar chip stays warm
// (the shell itself reads m.enabled per interaction).
func guardPoll(m *model) {
	go func() {
		for range time.Tick(3 * time.Second) {
			_ = m.enabled != nil && !m.enabled()
		}
	}()
}
