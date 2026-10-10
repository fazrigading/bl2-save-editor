package ui

import (
	"testing"

	"bl2save/desktop/native/session"
)

func TestCharNumFieldsKeys(t *testing.T) {
	want := []string{
		"level", "skill_points", "money", "eridium", "seraph", "torgue",
		"golden_keys", "inventory_size", "bank_size", "weapon_slots", "op_level",
	}
	fields := charNumFields()
	if len(fields) != len(want) {
		t.Fatalf("want %d fields, got %d", len(want), len(fields))
	}
	for i, f := range fields {
		if f.key != want[i] {
			t.Fatalf("field %d: want %q, got %q", i, want[i], f.key)
		}
	}
}

func TestCharDefaults(t *testing.T) {
	sv := &session.SaveView{Filename: "Save0001.sav"}
	obj := newCharacterPanel(nil, sv, func() bool { return true })
	if obj == nil {
		t.Fatal("nil panel")
	}
}

func TestCommitChangesMap(t *testing.T) {
	fields := charNumFields()
	byKey := map[string]numField{}
	for _, f := range fields {
		byKey[f.key] = f
	}
	m, ok := commitChanges(byKey["level"], "80")
	if !ok || m["level"] != int64(80) {
		t.Fatalf("want {level:80} ok, got %v ok=%v", m, ok)
	}
	if _, ok := commitChanges(byKey["level"], "abc"); ok {
		t.Fatal("non-numeric should fail without mutation")
	}
	if _, ok := commitChanges(byKey["level"], ""); ok {
		t.Fatal("empty should fail")
	}
}

func TestGuardDisablesChar(t *testing.T) {
	sv := &session.SaveView{Filename: "Save0001.sav"}
	obj := newCharacterPanel(nil, sv, func() bool { return false })
	if !allDisabled(obj) {
		t.Fatal("guard off should disable every input")
	}
	obj2 := newCharacterPanel(nil, sv, func() bool { return true })
	if allDisabled(obj2) {
		t.Fatal("guard on should leave inputs enabled")
	}
}

func TestRenderRefreshesAllInputs(t *testing.T) {
	sv := &session.SaveView{Filename: "Save0001.sav"}
	sv.Character.Name = "Axton"
	f := newCharForm(nil, sv, func() bool { return true })
	_ = f.build(sv)
	updated := sv.Character
	updated.Name = "Axton-Renamed"
	updated.HeadAsset = "GD_Soldier_Heads.Head_New"
	updated.SkinAsset = "GD_Soldier_Skins.Skin_New"
	f.render(&updated)
	if f.nameEd.Text != "Axton-Renamed" {
		t.Fatalf("name not re-rendered, got %q", f.nameEd.Text)
	}
	if f.headEd.Text != updated.HeadAsset {
		t.Fatalf("head not re-rendered, got %q", f.headEd.Text)
	}
	if f.skinEd.Text != updated.SkinAsset {
		t.Fatalf("skin not re-rendered, got %q", f.skinEd.Text)
	}
}
