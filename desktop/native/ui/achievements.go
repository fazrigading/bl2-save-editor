// Package ui — ACHIEVEMENTS tab (Fyne). Lists the achievement DB (grouped
// by category) in save-state mode; a gold MAX CHARACTER button writes all
// save-trackable conditions after confirmation.
package ui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/native/session"
)

// errNoSession is returned when an achievements action runs without a
// configured session.
var errNoSession = errors.New("session is not configured")

// achievementList builds the pure list object from rows. hideUnlocked
// filters already-unlocked rows; in save-state mode every Unlocked is nil,
// so nothing is filtered.
func achievementList(rows []session.AchievementRow, hideUnlocked bool) fyne.CanvasObject {
	box := container.NewVBox()
	var lastCat string
	for _, r := range rows {
		if r.Unlocked != nil && *r.Unlocked && hideUnlocked {
			continue
		}
		if r.CategoryName != lastCat {
			h := widget.NewLabel(strings.ToUpper(r.CategoryName))
			h.TextStyle = fyne.TextStyle{Bold: true}
			box.Add(h)
			lastCat = r.CategoryName
		}
		name := widget.NewLabel(r.Name)
		name.TextStyle = fyne.TextStyle{Bold: true}
		desc := widget.NewLabel(r.Desc)
		desc.Importance = widget.LowImportance
		desc.Wrapping = fyne.TextWrapWord
		glyph := widget.NewLabel(achievementGlyph(r))
		box.Add(container.NewHBox(glyph, container.NewVBox(name, desc)))
	}
	return box
}

// achievementGlyph renders the status marker: "✓" unlocked, "—" unknown
// (save-state mode), "·" locked.
func achievementGlyph(r session.AchievementRow) string {
	if r.Unlocked == nil {
		return "—"
	}
	if *r.Unlocked {
		return "✓"
	}
	return "·"
}

// achievementCount counts rows a list object would render (test seam).
func achievementCount(rows []session.AchievementRow, hideUnlocked bool) int {
	n := 0
	for _, r := range rows {
		if r.Unlocked != nil && *r.Unlocked && hideUnlocked {
			continue
		}
		n++
	}
	return n
}

// newAchievementsPanel builds the ACHIEVEMENTS tab content.
func newAchievementsPanel(ses *session.Session, sv *session.SaveView, enabled func() bool) fyne.CanvasObject {
	rows, err := listAchievements(ses)
	if err != nil {
		return container.NewVBox(
			widget.NewLabel("Achievements unavailable:"),
			widget.NewLabel(err.Error()),
		)
	}

	list := achievementList(rows, false)
	status := widget.NewLabel("Save-state mode: unlock state is not stored per save.")

	hide := widget.NewCheck("Hide unlocked", func(h bool) { _ = h })
	maxBtn := widget.NewButton("MAX CHARACTER", func() {
		confirmMax(ses, sv, status)
	})
	maxBtn.Importance = widget.HighImportance

	bar := container.NewHBox(hide, maxBtn)
	root := container.NewBorder(bar, status, nil, nil, container.NewVScroll(list))
	if enabled != nil && !enabled() {
		maxBtn.Disable()
		hide.Disable()
	}
	return root
}

// listAchievements reads the DB rows via the session (nil-safe).
func listAchievements(ses *session.Session) ([]session.AchievementRow, error) {
	if ses == nil {
		return nil, errNoSession
	}
	return ses.ListAchievements()
}

// confirmMax asks for confirmation, then enables all save-trackable
// achievements.
func confirmMax(ses *session.Session, sv *session.SaveView, status *widget.Label) {
	if ses == nil {
		status.SetText("no session")
		return
	}
	dialog.ShowConfirm("MAX CHARACTER",
		"Write save state that satisfies every save-trackable achievement for "+sv.Filename+"?",
		func(ok bool) {
			if !ok {
				return
			}
			res, err := ses.EnableAchievements(sv.Filename)
			if err != nil {
				status.SetText("max character: " + err.Error())
				return
			}
			keys := make([]string, 0, len(res))
			for k := range res {
				keys = append(keys, k)
			}
			status.SetText("max character applied: " + strings.Join(keys, ", "))
		}, nil)
}
