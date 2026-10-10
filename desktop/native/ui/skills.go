// Package ui — SKILLS tab (Fyne). One column per skill tree; each skill is
// a row with a 0..max stepper. A commit sends a single-path SetSkills map
// and re-renders points-remaining from the returned save state.
package ui

import (
	"fmt"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/internal/skills"
	"bl2save/desktop/native/session"
)

// skillRowView is the pure projection of one skill for rendering.
type skillRowView struct {
	Path, Name string
	Cur, Max   int
	AtMax      bool
}

// skillRows projects the save's current levels onto the tree groups. A
// level above the skill's max is clamped for display (never written back).
func skillRows(groups []skills.TreeGroup, levels map[string]uint64) [][]skillRowView {
	cols := make([][]skillRowView, len(groups))
	for gi, g := range groups {
		rows := make([]skillRowView, 0, len(g.Skills))
		for _, s := range g.Skills {
			cur := int(levels[s.Path])
			atMax := false
			if cur > s.Max {
				cur = s.Max
			}
			if cur == s.Max {
				atMax = true
			}
			rows = append(rows, skillRowView{Path: s.Path, Name: s.Name, Cur: cur, Max: s.Max, AtMax: atMax})
		}
		cols[gi] = rows
	}
	return cols
}

// optionsFor returns selectable point values for a row (0..Max).
func optionsFor(r skillRowView) []string {
	out := make([]string, 0, r.Max+1)
	for i := 0; i <= r.Max; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return out
}

// skillsPanel owns SKILLS tab state.
type skillsPanel struct {
	ses      *session.Session
	filename string
	groups   []skills.TreeGroup
	points   *widget.Label
	status   *widget.Label
	enabled  func() bool
	selects  []*widget.Select
}

// newSkillsTab builds the SKILLS tab content. An unknown/missing class
// shows an explanatory caption instead of panicking.
func newSkillsTab(ses *session.Session, sv *session.SaveView, enabled func() bool) fyne.CanvasObject {
	p := &skillsPanel{
		ses:      ses,
		filename: sv.Filename,
		enabled:  enabled,
	}
	groups, err := skills.Trees(sv.Character.Class)
	if err != nil {
		return container.NewVBox(
			widget.NewLabel("Skills unavailable for class " + strconv.Quote(sv.Character.Class)),
			widget.NewLabel(err.Error()),
		)
	}
	p.groups = groups

	levels, err := p.getSkills()
	if err != nil {
		levels = map[string]uint64{}
	}
	cols := container.NewHBox()
	projected := skillRows(groups, levels)
	for gi, g := range groups {
		name := widget.NewLabel(g.Name)
		name.TextStyle = fyne.TextStyle{Bold: true}
		col := container.NewVBox(name)
		for _, r := range projected[gi] {
			col.Add(p.skillRow(r))
		}
		cols.Add(col)
	}

	p.points = widget.NewLabel(p.pointsText(sv))
	p.points.TextStyle = fyne.TextStyle{Bold: true}
	p.status = widget.NewLabel("Skills ready.")

	root := container.NewBorder(p.points, p.status, nil, nil, container.NewVScroll(cols))
	p.applyGuard(sv)
	return root
}

// applyGuard disables every stepper when edits are barred: game running,
// or no skill points left. The backend enforces no budget, so the UI is
// the spending gate.
func (p *skillsPanel) applyGuard(sv *session.SaveView) {
	dis := sv.Character.SkillPoints == 0
	if p.enabled != nil && !p.enabled() {
		dis = true
	}
	p.setDisabled(dis)
}

// clampLevel bounds a commit level to [0, max] for a known skill path.
// Unknown paths pass through (no tree data to bound them).
func clampLevel(groups []skills.TreeGroup, path string, level int) int {
	max := skills.MaxFor(groups, path)
	if max == 0 {
		return level
	}
	if level < 0 {
		return 0
	}
	if level > max {
		return max
	}
	return level
}

// skillRow builds one skill row: name + points stepper.
func (p *skillsPanel) skillRow(r skillRowView) fyne.CanvasObject {
	sel := widget.NewSelect(optionsFor(r), nil)
	sel.SetSelected(strconv.Itoa(r.Cur))
	sel.OnChanged = func(v string) { p.commit(r.Path, v) }
	p.selects = append(p.selects, sel)
	lbl := widget.NewLabel(r.Name)
	return container.NewHBox(lbl, sel, widget.NewLabel(fmt.Sprintf("/%d", r.Max)))
}

// commit sends one skill change and re-renders points remaining.
func (p *skillsPanel) commit(path, val string) {
	if p.ses == nil {
		return
	}
	level, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return
	}
	level = int64(clampLevel(p.groups, path, int(level)))
	if _, err := p.ses.SetSkills(p.filename, map[string]int64{path: level}); err != nil {
		p.status.SetText("skills: " + err.Error())
		return
	}
	p.refreshPoints()
	p.status.SetText(path + " set to " + val)
}

// getSkills reads the save's current skill levels.
func (p *skillsPanel) getSkills() (map[string]uint64, error) {
	if p.ses == nil {
		return nil, fmt.Errorf("no session")
	}
	return p.ses.GetSkills(p.filename)
}

// refreshPoints re-reads the save to update points-remaining.
func (p *skillsPanel) refreshPoints() {
	if p.ses == nil {
		return
	}
	sv, err := p.ses.OpenSave(p.filename)
	if err != nil {
		return
	}
	p.points.SetText(p.pointsText(sv))
	p.applyGuard(sv)
}

// pointsText renders the points-remaining caption (disabled at 0).
func (p *skillsPanel) pointsText(sv *session.SaveView) string {
	n := sv.Character.SkillPoints
	if n == 0 {
		return "Skill points remaining: 0 (none available)"
	}
	return fmt.Sprintf("Skill points remaining: %d", n)
}

// setDisabled toggles every stepper.
func (p *skillsPanel) setDisabled(dis bool) {
	for _, s := range p.selects {
		if dis {
			s.Disable()
		} else {
			s.Enable()
		}
	}
}
