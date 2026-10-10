// Package ui — INVENTORY tab (Fyne). Sub-tabs (WEAPONS/ITEMS/BANK) hold a
// table each; selecting a row fills the detail pane (estimated stats +
// resolved type/part paths) and fires onSelect for the 3D preview. Actions
// (duplicate/delete/set-level/transfer) call session and fold the returned
// inventory back into every table in one pass.
package ui

import (
	"fmt"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"bl2save/desktop/native/session"
)

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

// swatchToken is the glyph the rarity column renders, colored by
// raritySwatchHex.
const swatchToken = "■"

// invTableData is the pure row projection for one category.
type invTableData struct {
	rows []session.ItemView
	cols int
}

// invTableCols is the column count (swatch, Name, Level, Type, Element).
const invTableCols = 5

// inventoryTableData projects one category's rows. Category matching is
// case-insensitive (session keys are lowercase; the tab labels are not).
func inventoryTableData(sv *session.SaveView, cat string) invTableData {
	d := invTableData{cols: invTableCols}
	if sv == nil {
		return d
	}
	d.rows = sv.Inventory[invCatKey(cat)]
	return d
}

// invCatKey resolves a category label or key to the SaveView map key.
func invCatKey(cat string) string {
	l := nameLower(cat)
	for _, c := range invCats() {
		if c.key == l || nameLower(c.label) == l {
			return c.key
		}
	}
	return l
}

// raritySwatchHex returns the hex color for a row's rarity, falling back
// to the common gray when the item carries no color.
func raritySwatchHex(it session.ItemView) string {
	if h := strings.TrimSpace(it.RarityColor); h != "" {
		return h
	}
	return hexOf(RarityColors["common"])
}

// hexOf renders a color as "#rrggbb".
func hexOf(c color.NRGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// invPanel owns the INVENTORY tab state.
type invPanel struct {
	ses      *session.Session
	filename string
	cats     []catDef
	items    map[string][]session.ItemView
	active   string
	selRow   int
	enabled  func() bool
	onSelect func(session.ItemView)

	sub        *container.AppTabs
	table      *widget.Table
	detail     *widget.Label
	status     *widget.Label
	transferTo *widget.Select

	btnDuplicate *widget.Button
	btnDelete    *widget.Button
	btnLevel     *widget.Button
	btnTransfer  *widget.Button
	levelEd      *widget.Entry
	levelRow     fyne.CanvasObject
}

// newInventoryPanel builds the INVENTORY tab content. onSelect fires on
// every row select (detail update + 3D preview switch in the shell).
func newInventoryPanel(ses *session.Session, sv *session.SaveView, onSelect func(session.ItemView), enabled func() bool) (fyne.CanvasObject, *invPanel) {
	p := &invPanel{
		ses:      ses,
		filename: sv.Filename,
		cats:     invCats(),
		items:    map[string][]session.ItemView{},
		active:   "weapons",
		selRow:   -1,
		enabled:  enabled,
		onSelect: onSelect,
	}
	for _, c := range p.cats {
		p.items[c.key] = append([]session.ItemView{}, sv.Inventory[c.key]...)
	}

	p.table = widget.NewTable(
		func() (int, int) { return len(p.rows()) + 1, invTableCols },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) { p.updateCell(id, o.(*widget.Label)) },
	)
	p.table.OnSelected = func(id widget.TableCellID) {
		if id.Row == 0 {
			return
		}
		p.selectRow(id.Row - 1)
	}

	// Sub-tabs per category.
	subs := make([]*container.TabItem, 0, len(p.cats))
	for _, c := range p.cats {
		c := c
		subs = append(subs, container.NewTabItemWithIcon(c.label, tabIcon(strings.ToUpper(c.label)),
			widget.NewLabel(""))) // content is the shared table below
	}
	p.sub = container.NewAppTabs(subs...)
	p.sub.OnSelected = func(t *container.TabItem) {
		p.active = invCatKey(t.Text)
		p.selRow = -1
		p.table.Refresh()
		p.clearDetail()
	}

	p.detail = widget.NewLabel("Select a row for stats")
	p.detail.Wrapping = fyne.TextWrapWord

	// Action bar.
	p.btnDuplicate = widget.NewButton("Duplicate", p.doDuplicate)
	p.btnDelete = widget.NewButton("Delete", p.doDelete)
	p.levelEd = widget.NewEntry()
	p.levelEd.PlaceHolder = "level"
	p.btnLevel = widget.NewButton("Set Level", p.commitLevel)
	p.levelRow = container.NewHBox(widget.NewLabel("Set level:"), p.levelEd, p.btnLevel)

	labels := make([]string, 0, len(p.cats))
	for _, c := range p.cats {
		labels = append(labels, c.label)
	}
	p.transferTo = widget.NewSelect(labels, nil)
	p.transferTo.SetSelected(labels[0])
	p.btnTransfer = widget.NewButton("Transfer", p.doTransfer)

	bar := container.NewHBox(p.btnDuplicate, p.btnDelete, p.transferTo, p.btnTransfer, p.levelRow)

	p.status = widget.NewLabel("Inventory ready.")

	detail := container.NewVScroll(p.detail)
	right := container.NewBorder(bar, p.status, nil, nil, detail)
	left := container.NewBorder(p.sub, nil, nil, nil, p.table)
	root := container.NewHSplit(left, right)
	root.Offset = 0.62

	if p.enabled != nil && !p.enabled() {
		p.setDisabled(true)
	}
	p.refreshAll()
	return root, p
}

// rows returns the selected category's rows.
func (p *invPanel) rows() []session.ItemView {
	return p.items[p.active]
}

// updateCell renders one table cell (row 0 = header).
func (p *invPanel) updateCell(id widget.TableCellID, l *widget.Label) {
	headers := []string{swatchToken, "Name", "Level", "Type", "Element"}
	if id.Row == 0 {
		l.TextStyle = fyne.TextStyle{Bold: true}
		l.SetText(headers[id.Col])
		return
	}
	l.TextStyle = fyne.TextStyle{}
	rows := p.rows()
	i := id.Row - 1
	if i < 0 || i >= len(rows) {
		l.SetText("")
		return
	}
	it := rows[i]
	switch id.Col {
	case 0:
		l.SetText(swatchToken)
		l.Importance = widget.MediumImportance
	case 1:
		l.SetText(it.DisplayName)
	case 2:
		l.SetText(strconv.Itoa(it.Level))
	case 3:
		l.SetText(it.Category)
	case 4:
		l.SetText(it.ElementName)
	}
}

// refresh rebuilds the active table in place.
func (p *invPanel) refresh() {
	if p.table != nil {
		p.table.Refresh()
	}
}

// refreshAll repatches every sub-tab (there is one shared table).
func (p *invPanel) refreshAll() {
	p.refresh()
}

// applyInventory swaps in a fresh post-mutation inventory and repatches.
func (p *invPanel) applyInventory(inv map[string][]session.ItemView) {
	for _, c := range p.cats {
		p.items[c.key] = append([]session.ItemView{}, inv[c.key]...)
	}
	p.refreshAll()
}

// selectRow handles a row selection: detail pane + preview switch.
func (p *invPanel) selectRow(row int) {
	rows := p.rows()
	if row < 0 || row >= len(rows) {
		return
	}
	p.selRow = row
	p.showDetail(rows[row])
	p.status.SetText(fmt.Sprintf("%s row %d selected", p.active, row))
}

// clearDetail resets the detail pane.
func (p *invPanel) clearDetail() {
	p.detail.SetText("Select a row for stats")
}

// showDetail renders stats and resolved part paths for one item.
func (p *invPanel) showDetail(it session.ItemView) {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  [%s]\n", it.DisplayName, it.RarityName)
	fmt.Fprintf(&b, "maker: %s  category: %s  element: %s  level: %d\n",
		it.Manufacturer, it.Category, it.ElementName, it.Level)
	if len(it.Stats) == 0 {
		b.WriteString("(no estimated stats)\n")
	} else {
		keys := make([]string, 0, len(it.Stats))
		for k := range it.Stats {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s: %v\n", k, it.Stats[k])
		}
	}
	types, parts := p.parts(it)
	b.WriteString("Types: " + joinOrDash(types) + "\n")
	b.WriteString("Parts: " + joinOrDash(parts))
	p.detail.SetText(b.String())
	if p.onSelect != nil {
		p.onSelect(it)
	}
}

// parts resolves the item's type/part paths via session.DetailParts,
// returning empties when unavailable (nil session, missing data).
func (p *invPanel) parts(it session.ItemView) (types, parts []string) {
	if p.ses == nil {
		return nil, nil
	}
	types, parts, err := p.ses.DetailParts(p.filename, it.Field, it.Index)
	if err != nil {
		return nil, nil
	}
	return types, parts
}

// joinOrDash joins items or returns "—" for an empty slice.
func joinOrDash(ss []string) string {
	if len(ss) == 0 {
		return "—"
	}
	return strings.Join(ss, ", ")
}

// setDisabled toggles every action control.
func (p *invPanel) setDisabled(dis bool) {
	for _, w := range []fyne.Disableable{p.btnDuplicate, p.btnDelete, p.btnLevel, p.btnTransfer, p.levelEd} {
		if dis {
			w.Disable()
		} else {
			w.Enable()
		}
	}
}

// mutate runs a session item-mutation wrapper, folds the returned map back,
// and surfaces guard/backend errors on the status line.
func (p *invPanel) mutate(what string, op func() (map[string][]session.ItemView, error)) {
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

// selected returns the currently selected row, ok=false when none.
func (p *invPanel) selected() (session.ItemView, bool) {
	rows := p.rows()
	if p.selRow < 0 || p.selRow >= len(rows) {
		return session.ItemView{}, false
	}
	return rows[p.selRow], true
}

// doDelete removes the selected row.
func (p *invPanel) doDelete() {
	it, ok := p.selected()
	if !ok || p.ses == nil {
		return
	}
	p.mutate("delete", func() (map[string][]session.ItemView, error) {
		return p.ses.DeleteItem(p.filename, it.Field, it.Index)
	})
}

// doDuplicate clones the selected row.
func (p *invPanel) doDuplicate() {
	it, ok := p.selected()
	if !ok || p.ses == nil {
		return
	}
	p.mutate("duplicate", func() (map[string][]session.ItemView, error) {
		return p.ses.DuplicateItem(p.filename, it.Field, it.Index)
	})
}

// doTransfer moves the selected row to the chosen category.
func (p *invPanel) doTransfer() {
	it, ok := p.selected()
	if !ok || p.ses == nil || p.transferTo == nil {
		return
	}
	toKey := invCatKey(p.transferTo.Selected)
	var toField int
	for _, c := range p.cats {
		if c.key == toKey {
			toField = c.field
		}
	}
	if toField == 0 {
		p.status.SetText("transfer: unknown target")
		return
	}
	p.mutate("transfer to "+toKey, func() (map[string][]session.ItemView, error) {
		return p.ses.TransferItem(p.filename, it.Field, it.Index, toField)
	})
}

// commitLevel applies the inline level edit (reverts on unparseable text).
func (p *invPanel) commitLevel() {
	it, ok := p.selected()
	if !ok || p.ses == nil {
		return
	}
	val, err := strconv.Atoi(strings.TrimSpace(p.levelEd.Text))
	if err != nil {
		p.levelEd.SetText(strconv.Itoa(it.Level))
		p.status.SetText("invalid level ignored")
		return
	}
	p.mutate("set level", func() (map[string][]session.ItemView, error) {
		return p.ses.SetItemLevel(p.filename, it.Field, it.Index, val)
	})
}
