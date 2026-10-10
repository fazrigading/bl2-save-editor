package ui

import (
	"testing"

	"bl2save/desktop/native/session"
	"bl2save/desktop/native/setup"
)

// fakeModel returns a model whose tabs can be built without a session.
func fakeModel() *model {
	return &model{st: setup.AssetStatus{Dir: "/tmp/none"}, enabled: func() bool { return true }}
}

func TestRenderSaveSwitchRebuilds(t *testing.T) {
	m := fakeModel()
	a := &session.SaveView{Filename: "Save0001.sav"}
	a.Character.Name = "Alpha"
	b := &session.SaveView{Filename: "Save0002.sav"}
	b.Character.Name = "Bravo"

	ca := buildTabContents(m, a)
	cb := buildTabContents(m, b)
	if len(ca) != 4 || len(cb) != 4 {
		t.Fatalf("want 4 panels per save, got %d / %d", len(ca), len(cb))
	}
	// The two saves must produce distinct widget trees (no shared state).
	if ca[0] == cb[0] {
		t.Fatal("save switch must rebuild CHARACTER content, not reuse it")
	}
	// The CHARACTER panel must show the fresh save's name.
	if !treeContainsLabel(ca[0], "Alpha") {
		t.Fatal("content A should render save A's name")
	}
	if !treeContainsLabel(cb[0], "Bravo") {
		t.Fatal("content B should render save B's name")
	}
	if treeContainsLabel(cb[0], "Alpha") {
		t.Fatal("save B content must not hold save A's data")
	}

	// Nil save (unopened state) shows the placeholder, no panic.
	ph := buildTabContents(m, nil)
	if len(ph) != 1 {
		t.Fatalf("nil save should render one placeholder, got %d", len(ph))
	}
}

func TestGuardDisablesAllTabs(t *testing.T) {
	off := fakeModel()
	off.enabled = func() bool { return false }
	sv := &session.SaveView{Filename: "Save0001.sav"}
	sv.Character.Class = "GD_Soldier"
	panels := buildTabContents(off, sv)
	for i, p := range panels {
		if !allDisabled(p) {
			t.Fatalf("panel %d should have every input disabled", i)
		}
	}
	on := fakeModel()
	panelsOn := buildTabContents(on, sv)
	if allDisabled(panelsOn[0]) {
		t.Fatal("guard on should leave CHARACTER inputs enabled")
	}
}

// treeContainsLabel reports whether any widget.Label in the tree holds text.
func treeContainsLabel(obj any, want string) bool {
	return walkLabels(obj, func(s string) bool { return s == want })
}

func TestSaveSwitchClearsSelection(t *testing.T) {
	m := fakeModel()
	m.curItem = session.ItemView{DisplayName: "Stale Gun"}
	tabs := newAppTabs(m)
	setTabContents(tabs, m, &session.SaveView{Filename: "Save0002.sav"})
	if m.curItem.DisplayName != "" {
		t.Fatalf("save switch must clear stale 3D selection, got %q", m.curItem.DisplayName)
	}
	setTabContents(tabs, m, nil)
	if m.curItem.DisplayName != "" {
		t.Fatalf("clearing selection must clear stale 3D selection, got %q", m.curItem.DisplayName)
	}
}
