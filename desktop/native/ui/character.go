package ui

import (
	"strconv"
	"strings"

	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/window"

	"bl2save/desktop/native/session"
)

// Character form notes:
// - gui.Edit dispatches OnChange per keystroke (edit.go CursorInput), so
//   committing there would fight typing (clearing "12" to type "20" would
//   revert mid-edit). Commit on OnFocusLost (dispatched by
//   Manager.SetKeyFocus, manager.go) and on Enter (OnKeyDown KeyEnter).
// - Numeric edits parse as int64 so negatives reach the backend, which clamps
//   out-of-range values itself (level -5 ignored, money -5 -> 0); the form
//   then re-renders from the returned view, so the clamp is visible.
// - Each commit sends a single-key SetCharacter map and re-renders every
//   widget from the returned view in one pass (mirrors applySaveState).
// - Unparseable text never reaches the backend: the edit reverts to the last
//   known value with a status note.

// numField binds one numeric SetCharacter change key to its label and its
// current value in a CharacterView (used for initial text and revert).
type numField struct {
	key   string
	label string
	get   func(*session.CharacterView) uint64
}

// charNumFields lists every numeric edit on the form in display order:
// identity (level/skill points), currencies, sizes.
func charNumFields() []numField {
	return []numField{
		{"level", "Level", func(v *session.CharacterView) uint64 { return v.Level }},
		{"skill_points", "Skill Points", func(v *session.CharacterView) uint64 { return v.SkillPoints }},
		{"money", "Money", func(v *session.CharacterView) uint64 { return v.Money }},
		{"eridium", "Eridium", func(v *session.CharacterView) uint64 { return v.Eridium }},
		{"seraph", "Seraph", func(v *session.CharacterView) uint64 { return v.Seraph }},
		{"torgue", "Torgue", func(v *session.CharacterView) uint64 { return v.Torgue }},
		{"golden_keys", "Golden Keys", func(v *session.CharacterView) uint64 { return v.GoldenKeys }},
		{"inventory_size", "Inventory Size", func(v *session.CharacterView) uint64 { return v.InventorySize }},
		{"bank_size", "Bank Size", func(v *session.CharacterView) uint64 { return v.BankSize }},
		{"weapon_slots", "Weapon Slots", func(v *session.CharacterView) uint64 { return v.WeaponSlots }},
	}
}

// charForm binds widgets to one save's CharacterView.
type charForm struct {
	ses      *session.Session
	filename string
	view     session.CharacterView
	fields   []numField

	classLabel *gui.Label
	xpLabel    *gui.Label
	status     *gui.Label
	nameEdit   *gui.Edit
	edits      map[string]*gui.Edit
}

// newCharacterPanel builds the CHARACTER tab content for an opened save.
func newCharacterPanel(ses *session.Session, sv *session.SaveView) *gui.Panel {
	p := gui.NewPanel(600, 400)
	p.SetLayout(gui.NewVBoxLayout())
	f := &charForm{
		ses:      ses,
		filename: sv.Filename,
		view:     sv.Character,
		fields:   charNumFields(),
		edits:    map[string]*gui.Edit{},
	}
	f.classLabel = gui.NewLabel("")
	p.Add(f.classLabel)
	f.xpLabel = gui.NewLabel("")
	p.Add(f.xpLabel)

	f.nameEdit = gui.NewEdit(200, "name")
	p.Add(newFormRow("Name", f.nameEdit))
	f.watchCommit(f.nameEdit, func() { f.commitName() })

	for _, nf := range f.fields {
		nf := nf
		ed := gui.NewEdit(120, nf.label)
		p.Add(newFormRow(nf.label, ed))
		f.edits[nf.key] = ed
		f.watchCommit(ed, func() { f.commitNumeric(nf) })
	}

	f.status = gui.NewLabel("Edits apply on Enter or focus loss.")
	p.Add(f.status)
	f.render(&sv.Character)
	return p
}

// newFormRow returns a label+edit row.
func newFormRow(label string, ed *gui.Edit) *gui.Panel {
	row := gui.NewPanel(600, 40)
	row.SetLayout(gui.NewHBoxLayout())
	row.Add(gui.NewLabel(label))
	row.Add(ed)
	return row
}

// watchCommit runs commit when the edit loses key focus or on Enter.
func (f *charForm) watchCommit(ed *gui.Edit, commit func()) {
	ed.Subscribe(gui.OnFocusLost, func(string, interface{}) { commit() })
	ed.Subscribe(gui.OnKeyDown, func(_ string, ev interface{}) {
		if kev, ok := ev.(*window.KeyEvent); ok && kev.Key == window.KeyEnter {
			commit()
		}
	})
}

// render rewrites every widget from the view in a single pass.
func (f *charForm) render(v *session.CharacterView) {
	f.view = *v
	f.classLabel.SetText("Class: " + v.ClassName)
	f.xpLabel.SetText("XP: " + strconv.FormatUint(v.Experience, 10))
	f.nameEdit.SetText(v.Name)
	for _, nf := range f.fields {
		if ed, ok := f.edits[nf.key]; ok {
			ed.SetText(strconv.FormatUint(nf.get(v), 10))
		}
	}
}

// commitName applies the name edit and re-renders from the returned view.
func (f *charForm) commitName() {
	updated, err := f.ses.SetCharacter(f.filename, map[string]any{"name": f.nameEdit.Text()})
	if err != nil {
		f.nameEdit.SetText(f.view.Name)
		f.status.SetText("name: " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText("name updated")
}

// commitNumeric parses the edit as int64 (negatives allowed, backend clamps),
// applies the single-key change, and re-renders from the returned view.
// Unparseable text reverts without a backend call.
func (f *charForm) commitNumeric(nf numField) {
	ed := f.edits[nf.key]
	val, err := strconv.ParseInt(strings.TrimSpace(ed.Text()), 10, 64)
	if err != nil {
		ed.SetText(strconv.FormatUint(nf.get(&f.view), 10))
		f.status.SetText("invalid input ignored: " + nf.key)
		return
	}
	updated, err := f.ses.SetCharacter(f.filename, map[string]any{nf.key: val})
	if err != nil {
		ed.SetText(strconv.FormatUint(nf.get(&f.view), 10))
		f.status.SetText(nf.key + ": " + err.Error())
		return
	}
	f.render(updated)
	f.status.SetText(nf.key + " updated")
}
