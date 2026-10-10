// Package ui — CHARACTER tab (Fyne). Form grid + read-only badges; edits
// commit on focus lost (same commit-on-blur pattern the g3n form used) and
// re-render from the returned view. Guard off (game running) disables all
// inputs.
package ui

import (
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/native/session"
)

// numField binds one numeric SetCharacter change key to its label and its
// current value in a CharacterView (used for initial text and revert).
type numField struct {
	key   string
	label string
	get   func(*session.CharacterView) uint64
}

// charNumFields lists every numeric edit on the form in display order:
// identity (level/skill points/op level), currencies, sizes.
func charNumFields() []numField {
	return []numField{
		{"level", "Level", func(v *session.CharacterView) uint64 { return v.Level }},
		{"skill_points", "Skill Points", func(v *session.CharacterView) uint64 { return v.SkillPoints }},
		{"money", "Money", func(v *session.CharacterView) uint64 { return v.Money }},
		{"eridium", "Eridium", func(v *session.CharacterView) uint64 { return v.Eridium }},
		{"seraph", "Seraph Crystals", func(v *session.CharacterView) uint64 { return v.Seraph }},
		{"torgue", "Torgue Tokens", func(v *session.CharacterView) uint64 { return v.Torgue }},
		{"golden_keys", "Golden Keys", func(v *session.CharacterView) uint64 { return v.GoldenKeys }},
		{"inventory_size", "Inventory Size", func(v *session.CharacterView) uint64 { return v.InventorySize }},
		{"bank_size", "Bank Size", func(v *session.CharacterView) uint64 { return v.BankSize }},
		{"weapon_slots", "Weapon Slots", func(v *session.CharacterView) uint64 { return v.WeaponSlots }},
		{"op_level", "OP Level", func(v *session.CharacterView) uint64 { return v.OpLevel }},
	}
}

// commitChanges parses one numeric field edit into a single-key
// SetCharacter map. Empty or unparseable text returns ok=false with no
// mutation (backend clamps out-of-range values itself).
func commitChanges(nf numField, text string) (map[string]any, bool) {
	t := strings.TrimSpace(text)
	if t == "" {
		return nil, false
	}
	val, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return nil, false
	}
	return map[string]any{nf.key: val}, true
}

// charForm binds widgets to one save's CharacterView.
type charForm struct {
	ses      *session.Session
	filename string
	view     session.CharacterView
	fields   []numField
	enabled  func() bool

	status *widget.Label
	edits  map[string]*widget.Entry
	nameEd *widget.Entry
	headEd *widget.Entry
	skinEd *widget.Entry
	rgbEds []*widget.Entry
	tvhm   *widget.Button
	uvhm   *widget.Button
}

// newCharacterPanel builds the CHARACTER tab content for an opened save.
// enabled is the shell guard hook; false disables every input.
func newCharacterPanel(ses *session.Session, sv *session.SaveView, enabled func() bool) fyne.CanvasObject {
	f := &charForm{
		ses:      ses,
		filename: sv.Filename,
		view:     sv.Character,
		fields:   charNumFields(),
		enabled:  enabled,
		edits:    map[string]*widget.Entry{},
	}
	return f.build(sv)
}

// formRow is one label+input grid row.
func formRow(label string, w fyne.CanvasObject) (fyne.CanvasObject, fyne.CanvasObject) {
	return widget.NewLabel(label), w
}

// build lays out the two-column grid and wires commits.
func (f *charForm) build(sv *session.SaveView) fyne.CanvasObject {
	left := container.NewVBox()

	// Class read-only: UpdateCharacter has no class key; render locked so
	// the layout matches the spec.
	classSel := widget.NewSelect([]string{f.view.ClassName}, nil)
	classSel.SetSelected(f.view.ClassName)
	classSel.Disable()

	f.nameEd = newNumEntry()
	f.nameEd.SetText(f.view.Name)
	left.Add(gridRow("Name", f.nameEd))
	watchCommit(f.nameEd, func() { f.commitName() })

	// Appearance: head/skin asset text inputs + 3×RGB numeric inputs.
	f.headEd = newNumEntry()
	f.headEd.SetText(f.view.HeadAsset)
	left.Add(gridRow("Head Asset", f.headEd))
	watchCommit(f.headEd, func() { f.commitAsset("head_asset", f.headEd, f.view.HeadAsset) })

	f.skinEd = newNumEntry()
	f.skinEd.SetText(f.view.SkinAsset)
	left.Add(gridRow("Skin Asset", f.skinEd))
	watchCommit(f.skinEd, func() { f.commitAsset("skin_asset", f.skinEd, f.view.SkinAsset) })

	for i := range 3 {
		ed := newNumEntry()
		if i < len(f.view.Colors) {
			ed.SetText(strconv.FormatUint(f.view.Colors[i].R, 10))
		}
		f.rgbEds = append(f.rgbEds, ed)
		idx := i
		watchCommit(ed, func() { f.commitRGB(idx, ed) })
		left.Add(gridRow("Color R/G/B", ed))
	}

	for _, nf := range f.fields {
		nf := nf
		ed := newNumEntry()
		ed.SetText(strconv.FormatUint(nf.get(&f.view), 10))
		f.edits[nf.key] = ed
		left.Add(gridRow(nf.label, ed))
		watchCommit(ed, func() { f.commitNumeric(nf, ed) })
	}

	// Right column: read-only badges, headed by the character name so a
	// save switch visibly rebuilds content from the fresh SaveView.
	right := container.NewVBox(
		widget.NewLabel(f.view.Name),
		widget.NewLabel("Class: "+f.view.ClassName),
		widget.NewLabel("XP: "+strconv.FormatUint(f.view.Experience, 10)),
		widget.NewLabel("Playthroughs: "+strconv.FormatUint(f.view.PlaythroughsCompleted, 10)),
		widget.NewLabel("Time played: "+strconv.FormatUint(f.view.TimePlayed, 10)+"s"),
	)

	f.tvhm = widget.NewButton("UNLOCK TVHM", func() { f.unlockPlaythrough("tvhm") })
	f.uvhm = widget.NewButton("UNLOCK UVHM", func() { f.unlockPlaythrough("uvhm") })
	right.Add(container.NewHBox(f.tvhm, f.uvhm))

	f.status = widget.NewLabel("Edits apply on Enter.")
	left.Add(f.status)

	// Apply guard.
	if f.enabled != nil && !f.enabled() {
		f.setDisabled(true)
	}

	return container.NewHSplit(container.NewVScroll(left), container.NewVScroll(right))
}

// gridRow returns a two-cell row for the form grid.
func gridRow(label string, w fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(widget.NewLabel(label), w)
}

// newNumEntry makes a single-line numeric-ish entry.
func newNumEntry() *widget.Entry {
	e := widget.NewEntry()
	e.PlaceHolder = "—"
	return e
}

// watchCommit commits on Enter (OnSubmitted). Fyne entries have no
// focus-lost hook; Enter is the explicit commit gesture.
func watchCommit(e *widget.Entry, commit func()) {
	e.OnSubmitted = func(string) { commit() }
}

// setDisabled walks every input and disables (or re-enables) it.
func (f *charForm) setDisabled(dis bool) {
	for _, ed := range f.edits {
		if dis {
			ed.Disable()
		} else {
			ed.Enable()
		}
	}
	for _, ed := range append([]*widget.Entry{f.nameEd, f.headEd, f.skinEd}, f.rgbEds...) {
		if dis {
			ed.Disable()
		} else {
			ed.Enable()
		}
	}
	for _, b := range []*widget.Button{f.tvhm, f.uvhm} {
		if b == nil {
			continue
		}
		if dis {
			b.Disable()
		} else {
			b.Enable()
		}
	}
}

// walkEntries visits every widget.Entry in the tree. Fyne exposes no
// universal child iterator, so the container kinds the panels build
// (VBox/HBox/Split/Scroll/Border/AppTabs) are unwrapped explicitly.
func walkEntries(obj fyne.CanvasObject, fn func(*widget.Entry)) {
	walkTree(obj, func(o fyne.CanvasObject) {
		if e, ok := o.(*widget.Entry); ok {
			fn(e)
		}
	})
}

// walkLabels reports whether any widget.Label in the tree satisfies pred.
func walkLabels(obj any, pred func(string) bool) bool {
	o, ok := obj.(fyne.CanvasObject)
	if !ok {
		return false
	}
	found := false
	walkTree(o, func(n fyne.CanvasObject) {
		if l, ok := n.(*widget.Label); ok && pred(l.Text) {
			found = true
		}
	})
	return found
}

// walkTree visits every node in the tree, unwrapping the container kinds
// the panels build. Fyne exposes no universal child iterator.
func walkTree(obj fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	if obj == nil {
		return
	}
	fn(obj)
	switch o := obj.(type) {
	case *container.Split:
		walkTree(o.Leading, fn)
		walkTree(o.Trailing, fn)
	case *container.Scroll:
		walkTree(o.Content, fn)
	case *container.AppTabs:
		for _, it := range o.Items {
			walkTree(it.Content, fn)
		}
	case *fyne.Container:
		for _, c := range o.Objects {
			walkTree(c, fn)
		}
	}
}

// commitName applies the name edit and re-renders from the returned view.
func (f *charForm) commitName() {
	if f.ses == nil {
		return
	}
	updated, err := f.ses.SetCharacter(f.filename, map[string]any{"name": f.nameEd.Text})
	if err != nil {
		f.nameEd.SetText(f.view.Name)
		f.status.SetText("name: " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText("name updated")
}

// commitAsset applies head/skin asset edits.
func (f *charForm) commitAsset(key string, ed *widget.Entry, current string) {
	if f.ses == nil {
		return
	}
	updated, err := f.ses.SetCharacter(f.filename, map[string]any{key: ed.Text})
	if err != nil {
		ed.SetText(current)
		f.status.SetText(key + ": " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText(key + " updated")
}

// commitRGB applies one appearance color zone (keeps other channels).
func (f *charForm) commitRGB(idx int, ed *widget.Entry) {
	if f.ses == nil {
		return
	}
	r, err := strconv.ParseUint(strings.TrimSpace(ed.Text), 10, 64)
	if err != nil {
		if idx < len(f.view.Colors) {
			ed.SetText(strconv.FormatUint(f.view.Colors[idx].R, 10))
		}
		f.status.SetText("invalid color ignored")
		return
	}
	colors := make([]map[string]any, 3)
	for i := range 3 {
		c := map[string]any{"a": 255, "r": 127, "g": 127, "b": 127}
		if i < len(f.view.Colors) {
			c["a"] = f.view.Colors[i].A
			c["r"] = f.view.Colors[i].R
			c["g"] = f.view.Colors[i].G
			c["b"] = f.view.Colors[i].B
		}
		colors[i] = c
	}
	colors[idx]["r"] = r
	updated, err := f.ses.SetCharacter(f.filename, map[string]any{"appearance_colors": colors})
	if err != nil {
		f.status.SetText("appearance: " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText("appearance updated")
}

// commitNumeric parses the edit as int64 (negatives allowed, backend
// clamps), applies the single-key change, and re-renders. Unparseable text
// reverts without a backend call.
func (f *charForm) commitNumeric(nf numField, ed *widget.Entry) {
	if f.ses == nil {
		return
	}
	changes, ok := commitChanges(nf, ed.Text)
	if !ok {
		ed.SetText(strconv.FormatUint(nf.get(&f.view), 10))
		f.status.SetText("invalid input ignored: " + nf.key)
		return
	}
	updated, err := f.ses.SetCharacter(f.filename, changes)
	if err != nil {
		ed.SetText(strconv.FormatUint(nf.get(&f.view), 10))
		f.status.SetText(nf.key + ": " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText(nf.key + " updated")
}

// unlockPlaythrough wires the TVHM/UVHM buttons to session.UnlockPlaythrough.
func (f *charForm) unlockPlaythrough(target string) {
	if f.ses == nil {
		return
	}
	ok, err := f.ses.UnlockPlaythrough(f.filename, target)
	if err != nil {
		f.status.SetText(target + ": " + err.Error())
		return
	}
	if !ok {
		f.status.SetText(target + ": already unlocked")
		return
	}
	sv, err := f.ses.OpenSave(f.filename)
	if err != nil {
		f.status.SetText(target + ": " + err.Error())
		return
	}
	f.render(&sv.Character)
	f.status.SetText(target + " unlocked")
}

// render rewrites every widget from the view in a single pass.
func (f *charForm) render(v *session.CharacterView) {
	f.view = *v
	f.nameEd.SetText(v.Name)
	for _, nf := range f.fields {
		if ed, ok := f.edits[nf.key]; ok {
			ed.SetText(strconv.FormatUint(nf.get(v), 10))
		}
	}
}
