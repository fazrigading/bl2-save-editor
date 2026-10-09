package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/g3n/engine/window"

	"bl2save/desktop/native/session"
)

// Inventory pane notes (g3n engine v0.2.0, module cache gui/table.go,
// gui/menu.go):
// - gui.NewTable(w,h,cols)(*Table,error); AddRow(map[string]interface{});
//   Clear(); SelectedRows()[]int; SetStatusText(string).
// - Left-click on a row dispatches OnChange; right-click dispatches
//   OnTableClick with a TableClickEvent{Row, Col, MouseEvent}.
//   There is NO programmatic row-selection API (rowCursor is private),
//   so the right-click row is tracked from the event and the highlight
//   follows left-clicks natively; the status line names the action row.
// - Table has no per-cell color API (styles are row-level), so the rarity
//   hex colors the 3D preview material instead of the rarity cell text.
// - Menu is a plain panel: gui.NewMenu()+AddOption(text)*MenuItem, item
//   dispatches OnClick, AddMenu(text,sub) builds the Transfer-to submenu.
//   v0.2.0 has no popup helper and click coords are table-local, so the
//   menu opens at a fixed spot in the pane and hides after each action.
// - Level prompt reuses the character.go commit pattern: commit on Enter
//   (OnKeyDown KeyEnter) or OnFocusLost, revert on unparseable text.

// catDef binds one inventory sub-tab to its session map key and backend
// field number (weapons=54, items=53, bank=41).
type catDef struct {
	key   string
	label string
	field int
}

// invCats lists the sub-tabs in display order.
func invCats() []catDef {
	return []catDef{
		{"weapons", "Weapons", 54},
		{"items", "Items", 53},
		{"bank", "Bank", 41},
	}
}

// elementColors maps DetectElement names (internal/assets/assets.go) to
// flat PBR tints.
func elementColors() map[string]string {
	return map[string]string{
		"Fire":      "#ff6600",
		"Shock":     "#0099ff",
		"Corrosive": "#00dd00",
		"Slag":      "#cc00ff",
		"Explosive": "#ffdd00",
	}
}

// parseHexColor parses "#rrggbb" into a g3n color; bad input yields gray.
func parseHexColor(s string) math32.Color {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return math32.Color{R: 0.6, G: 0.6, B: 0.6}
	}
	n, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return math32.Color{R: 0.6, G: 0.6, B: 0.6}
	}
	return math32.Color{
		R: float32(n>>16&0xff) / 255,
		G: float32(n>>8&0xff) / 255,
		B: float32(n&0xff) / 255,
	}
}

// previewMesh builds the flat-PBR proxy for an item: rarity hex as the
// base Standard color, element hex as emissive. Geometry is a proxy by
// kind (weapons read as box "guns", everything else as compact solids).
// Mask/MIC compositing is explicitly out of scope (slice-1 brief R1).
func previewMesh(view session.ItemView) *graphic.Mesh {
	base := parseHexColor(view.RarityColor)
	mat := material.NewStandard(&base)
	if ec, ok := elementColors()[view.ElementName]; ok {
		em := parseHexColor(ec)
		mat.SetEmissiveColor(&em)
	}
	var mesh *graphic.Mesh
	switch {
	case view.IsWeapon:
		mesh = graphic.NewMesh(geometry.NewBox(0.9, 0.25, 0.25), mat)
	case strings.Contains(view.Category, "Shield"):
		mesh = graphic.NewMesh(geometry.NewBox(0.5, 0.7, 0.12), mat)
	case strings.Contains(view.Category, "Grenade"):
		mesh = graphic.NewMesh(geometry.NewSphere(0.3, 16, 12), mat)
	default:
		mesh = graphic.NewMesh(geometry.NewBox(0.35, 0.35, 0.35), mat)
	}
	return mesh
}

// invPanel owns the INVENTORY tab: sub-tabs with Tables, a stats panel,
// the action menu, and the level prompt. Shell wires onSelect to the 3D
// preview switch and drives stepCarousel from the viewer bar.
type invPanel struct {
	ses      *session.Session
	filename string
	cats     []catDef
	items    map[string][]session.ItemView
	tables   map[string]*gui.Table
	sub      *gui.TabBar
	active   string

	statsTitle *gui.Label
	statsMeta  *gui.Label
	statsLines *gui.Label
	status     *gui.Label

	menu   *gui.Menu
	ctxCat string
	ctxRow int

	levelRow  *gui.Panel
	levelEdit *gui.Edit
	levelCat  string
	levelRowI int

	carouselIdx int
	onSelect    func(session.ItemView)
}

// newInventoryPanel builds the INVENTORY tab content. onSelect fires on
// every row select (stats update + preview switch in shell).
func newInventoryPanel(ses *session.Session, sv *session.SaveView, onSelect func(session.ItemView)) (*gui.Panel, *invPanel) {
	root := gui.NewPanel(600, 420)
	root.SetLayout(gui.NewVBoxLayout())
	p := &invPanel{
		ses:      ses,
		filename: sv.Filename,
		cats:     invCats(),
		items:    map[string][]session.ItemView{},
		tables:   map[string]*gui.Table{},
		active:   "weapons",
		onSelect: onSelect,
	}
	for _, c := range p.cats {
		p.items[c.key] = append([]session.ItemView{}, sv.Inventory[c.key]...)
	}

	cols := []gui.TableColumn{
		{Id: "name", Header: "Name", Width: 240, Expand: 1},
		{Id: "rarity", Header: "Rarity", Width: 110},
		{Id: "level", Header: "Level", Width: 60},
	}
	p.sub = gui.NewTabBar(580, 260)
	for _, c := range p.cats {
		c := c
		tb, err := gui.NewTable(560, 200, cols)
		if err != nil {
			tb, _ = gui.NewTable(560, 200, cols[:1])
		}
		p.tables[c.key] = tb
		tb.ShowStatus(true)
		tab := p.sub.AddTab(c.label)
		tab.SetPinned(true)
		tab.SetContent(tb)
		tb.Subscribe(gui.OnChange, func(string, interface{}) { p.selectChanged(c.key) })
		tb.Subscribe(gui.OnTableClick, func(_ string, ev interface{}) { p.click(c.key, ev) })
	}
	p.sub.SetSelected(0)
	root.Add(p.sub)

	p.statsTitle = gui.NewLabel("Select a row for stats")
	root.Add(p.statsTitle)
	p.statsMeta = gui.NewLabel("")
	root.Add(p.statsMeta)
	p.statsLines = gui.NewLabel("")
	root.Add(p.statsLines)

	p.levelRow = gui.NewPanel(580, 40)
	p.levelRow.SetLayout(gui.NewHBoxLayout())
	p.levelRow.Add(gui.NewLabel("Set level:"))
	p.levelEdit = gui.NewEdit(80, "level")
	p.levelRow.Add(p.levelEdit)
	p.levelEdit.Subscribe(gui.OnFocusLost, func(string, interface{}) { p.commitLevel() })
	p.levelEdit.Subscribe(gui.OnKeyDown, func(_ string, ev interface{}) {
		if kev, ok := ev.(*window.KeyEvent); ok && kev.Key == window.KeyEnter {
			p.commitLevel()
		}
	})
	p.levelRow.SetVisible(false)
	root.Add(p.levelRow)

	p.menu = gui.NewMenu()
	p.menu.AddOption("Delete").Subscribe(gui.OnClick, func(string, interface{}) { p.doDelete() })
	p.menu.AddOption("Duplicate").Subscribe(gui.OnClick, func(string, interface{}) { p.doDuplicate() })
	transferSub := gui.NewMenu()
	for _, c := range p.cats {
		c := c
		transferSub.AddOption(c.label).Subscribe(gui.OnClick, func(string, interface{}) { p.doTransfer(c.field) })
	}
	p.menu.AddMenu("Transfer to", transferSub)
	p.menu.AddOption("Set level...").Subscribe(gui.OnClick, func(string, interface{}) { p.promptLevel() })
	p.menu.AddOption("Cancel").Subscribe(gui.OnClick, func(string, interface{}) { p.menu.SetVisible(false) })
	p.menu.SetVisible(false)
	root.Add(p.menu)

	p.status = gui.NewLabel("Inventory ready.")
	root.Add(p.status)

	p.refreshAll()
	return root, p
}

// refresh rebuilds one table's rows in place (tables persist).
func (p *invPanel) refresh(cat string) {
	tb := p.tables[cat]
	if tb == nil {
		return
	}
	tb.Clear()
	for _, v := range p.items[cat] {
		tb.AddRow(map[string]interface{}{
			"name":   v.DisplayName,
			"rarity": v.RarityName,
			"level":  v.Level,
		})
	}
	tb.SetStatusText(fmt.Sprintf("%d items", len(p.items[cat])))
}

// refreshAll repatches every sub-tab table in place.
func (p *invPanel) refreshAll() {
	for _, c := range p.cats {
		p.refresh(c.key)
	}
}

// applyInventory swaps in a fresh post-mutation inventory and repatches.
func (p *invPanel) applyInventory(inv map[string][]session.ItemView) {
	for _, c := range p.cats {
		p.items[c.key] = append([]session.ItemView{}, inv[c.key]...)
	}
	p.refreshAll()
}

// selectChanged handles left-click selection: stats + preview switch.
func (p *invPanel) selectChanged(cat string) {
	p.active = cat
	rows := p.tables[cat].SelectedRows()
	if len(rows) == 0 {
		return
	}
	p.showRow(cat, rows[0])
}

// click handles right-click: track the event row and pop the action menu.
func (p *invPanel) click(cat string, ev interface{}) {
	tce, ok := ev.(gui.TableClickEvent)
	if !ok || tce.Button != window.MouseButtonRight || tce.Row < 0 {
		return
	}
	if tce.Row >= len(p.items[cat]) {
		return
	}
	p.active = cat
	p.ctxCat, p.ctxRow = cat, tce.Row
	p.carouselIdx = tce.Row
	p.menu.SetPosition(40, 40)
	p.menu.SetVisible(true)
	p.status.SetText(fmt.Sprintf("%s row %d: menu open", cat, tce.Row))
}

// showRow renders stats for one row and fires the preview switch.
func (p *invPanel) showRow(cat string, row int) {
	views := p.items[cat]
	if row < 0 || row >= len(views) {
		return
	}
	v := views[row]
	p.active = cat
	p.carouselIdx = row
	p.statsTitle.SetText(v.DisplayName + "  [" + v.RarityName + "]")
	p.statsMeta.SetText(fmt.Sprintf("maker: %s  category: %s  element: %s  level: %d",
		v.Manufacturer, v.Category, v.ElementName, v.Level))
	if len(v.Stats) == 0 {
		p.statsLines.SetText("(no estimated stats)")
	} else {
		keys := make([]string, 0, len(v.Stats))
		for k := range v.Stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		lines := make([]string, 0, len(keys))
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("%s: %v", k, v.Stats[k]))
		}
		p.statsLines.SetText(strings.Join(lines, "\n"))
	}
	p.status.SetText(fmt.Sprintf("%s row %d selected", cat, row))
	if p.onSelect != nil {
		p.onSelect(v)
	}
}

// stepCarousel moves the preview within the active category (viewer bar
// prev/next). The Table highlight cannot follow (no selection API), so the
// status line names the carousel row.
func (p *invPanel) stepCarousel(d int) {
	views := p.items[p.active]
	if len(views) == 0 {
		p.status.SetText(p.active + ": empty")
		return
	}
	p.carouselIdx = (p.carouselIdx + d + len(views)) % len(views)
	p.showRow(p.active, p.carouselIdx)
}

// mutate runs a session item-mutation wrapper, repatches tables in place,
// and surfaces guard/backend errors on the status line.
func (p *invPanel) mutate(what string, op func() (map[string][]session.ItemView, error)) {
	p.menu.SetVisible(false)
	inv, err := op()
	if err != nil {
		p.status.SetText(what + ": " + err.Error())
		return
	}
	p.applyInventory(inv)
	p.status.SetText(what + " applied")
}

// keyOf returns the category key for a backend field number.
func (p *invPanel) keyOf(field int) string {
	for _, c := range p.cats {
		if c.field == field {
			return c.key
		}
	}
	return ""
}

// doDelete removes the menu row.
func (p *invPanel) doDelete() {
	views := p.items[p.ctxCat]
	if p.ctxRow < 0 || p.ctxRow >= len(views) {
		p.menu.SetVisible(false)
		return
	}
	v := views[p.ctxRow]
	p.mutate("delete", func() (map[string][]session.ItemView, error) {
		return p.ses.DeleteItem(p.filename, v.Field, v.Index)
	})
}

// doDuplicate clones the menu row.
func (p *invPanel) doDuplicate() {
	views := p.items[p.ctxCat]
	if p.ctxRow < 0 || p.ctxRow >= len(views) {
		p.menu.SetVisible(false)
		return
	}
	v := views[p.ctxRow]
	p.mutate("duplicate", func() (map[string][]session.ItemView, error) {
		return p.ses.DuplicateItem(p.filename, v.Field, v.Index)
	})
}

// doTransfer moves the menu row to the chosen field.
func (p *invPanel) doTransfer(toField int) {
	views := p.items[p.ctxCat]
	if p.ctxRow < 0 || p.ctxRow >= len(views) {
		p.menu.SetVisible(false)
		return
	}
	v := views[p.ctxRow]
	p.mutate("transfer to "+p.keyOf(toField), func() (map[string][]session.ItemView, error) {
		return p.ses.TransferItem(p.filename, v.Field, v.Index, toField)
	})
}

// promptLevel reveals the level edit for the menu row.
func (p *invPanel) promptLevel() {
	views := p.items[p.ctxCat]
	if p.ctxRow < 0 || p.ctxRow >= len(views) {
		p.menu.SetVisible(false)
		return
	}
	p.menu.SetVisible(false)
	p.levelCat, p.levelRowI = p.ctxCat, p.ctxRow
	p.levelEdit.SetText(strconv.Itoa(views[p.ctxRow].Level))
	p.levelRow.SetVisible(true)
	p.status.SetText(fmt.Sprintf("%s row %d: enter new level", p.levelCat, p.levelRowI))
}

// commitLevel applies the level edit and repatches in place. Unparseable
// text reverts without a backend call (character.go pattern).
func (p *invPanel) commitLevel() {
	if !p.levelRow.Visible() {
		return
	}
	views := p.items[p.levelCat]
	if p.levelRowI < 0 || p.levelRowI >= len(views) {
		p.levelRow.SetVisible(false)
		return
	}
	val, err := strconv.Atoi(strings.TrimSpace(p.levelEdit.Text()))
	if err != nil {
		p.levelEdit.SetText(strconv.Itoa(views[p.levelRowI].Level))
		p.status.SetText("invalid level ignored")
		return
	}
	v := views[p.levelRowI]
	p.levelRow.SetVisible(false)
	p.mutate("set level", func() (map[string][]session.ItemView, error) {
		return p.ses.SetItemLevel(p.filename, v.Field, v.Index, val)
	})
}
