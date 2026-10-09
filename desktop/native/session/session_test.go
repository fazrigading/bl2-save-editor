package session

import (
	"os"
	"path/filepath"
	"testing"

	"bl2save/desktop/internal/platform"
)

func testSession(t *testing.T, dir string) *Session {
	t.Helper()
	if dir == "" {
		dir = t.TempDir()
		data, err := os.ReadFile("../../../tests/Save0001.sav")
		if err != nil {
			t.Skipf("fixture missing: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return New(&platform.Config{SaveDir: dir, BackupGenerations: 5})
}

func TestListSavesFindsOne(t *testing.T) {
	saves, err := testSession(t, "").ListSaves()
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(saves) != 1 {
		t.Fatalf("expected 1 save, got %v", saves)
	}
	if saves[0].Filename != "Save0001.sav" {
		t.Fatalf("unexpected filename: %+v", saves[0])
	}
}

func TestListSavesEmptyDir(t *testing.T) {
	saves, err := testSession(t, t.TempDir()).ListSaves()
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(saves) != 0 {
		t.Fatalf("expected 0 saves, got %v", saves)
	}
}

func TestOpenSaveCharacter(t *testing.T) {
	sv, err := testSession(t, "").OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	if sv.Filename != "Save0001.sav" {
		t.Fatalf("unexpected filename: %+v", sv)
	}
	c := sv.Character
	if c.Class == "" || c.ClassName == "" || c.Name == "" {
		t.Fatalf("missing character identity: %+v", c)
	}
	if c.Level < 1 {
		t.Fatalf("bad level: %+v", c)
	}
	if len(c.Colors) != 3 {
		t.Fatalf("expected 3 appearance colors, got %+v", c.Colors)
	}
}

func TestOpenSaveCharacterEconomy(t *testing.T) {
	s := testSession(t, "")
	_, tree, err := s.store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatalf("ReadSave: %v", err)
	}
	info := s.store.ExtractCharacterInfo(tree)
	sv, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	c := sv.Character
	got := map[string]uint64{
		"experience": c.Experience, "skill_points": c.SkillPoints,
		"money": c.Money, "eridium": c.Eridium, "seraph": c.Seraph,
		"torgue": c.Torgue, "golden_keys": c.GoldenKeys,
		"inventory_size": c.InventorySize, "weapon_slots": c.WeaponSlots,
		"bank_size": c.BankSize,
	}
	for key, val := range got {
		if val != toUint(info[key]) {
			t.Fatalf("field %s: view=%d info=%v", key, val, info[key])
		}
	}
}

func TestOpenSaveRejectsTraversal(t *testing.T) {
	s := testSession(t, "")
	for _, bad := range []string{"evil.sav", "../../etc/passwd", "Save0001.sav/.."} {
		if _, err := s.OpenSave(bad); err == nil {
			t.Fatalf("expected rejection of %q", bad)
		}
	}
}

func TestOpenSaveCorruptReturnsError(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("../../../tests/Save0001.sav")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data[:100], 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := testSession(t, dir).OpenSave("Save0001.sav"); err == nil {
		t.Fatal("expected error for truncated save")
	}
}

func TestNewNilConfigShowsSetup(t *testing.T) {
	s := New(&platform.Config{})
	if _, err := s.ListSaves(); err == nil {
		t.Fatal("expected setup error from unconfigured session")
	}
}

func TestOpenSaveInventoryKeys(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	sv, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	for _, key := range []string{"weapons", "items", "bank"} {
		if _, ok := sv.Inventory[key]; !ok {
			t.Fatalf("missing inventory key %q: %+v", key, sv.Inventory)
		}
	}
	total := len(sv.Inventory["weapons"]) + len(sv.Inventory["items"]) + len(sv.Inventory["bank"])
	if total == 0 {
		t.Fatal("expected total items > 0")
	}
}

func TestOpenSaveItemViewsResolved(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	sv, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	for key, rows := range sv.Inventory {
		for _, it := range rows {
			if it.DisplayName == "" || it.RarityName == "" {
				t.Fatalf("unresolved item in %q: %+v", key, it)
			}
		}
	}
}

func TestOpenSaveInventoryNoGibbed(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("../../../tests/Save0001.sav")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := New(&platform.Config{SaveDir: dir, BackupGenerations: 5, GibbedDir: ""})
	seedTestItems(t, s)
	sv, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave with empty gibbed dir: %v", err)
	}
	total := len(sv.Inventory["weapons"]) + len(sv.Inventory["items"]) + len(sv.Inventory["bank"])
	if total == 0 {
		t.Fatal("expected rows still listed with empty gibbed dir")
	}
}

func TestSetCharacterLevel(t *testing.T) {
	s := testSession(t, "")
	cv, err := s.SetCharacter("Save0001.sav", map[string]any{"level": 20})
	if err != nil {
		t.Fatalf("SetCharacter: %v", err)
	}
	if cv.Level != 20 {
		t.Fatalf("expected level 20, got %+v", cv)
	}
}

func TestSetCharacterClamps(t *testing.T) {
	s := testSession(t, "")
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	cv, err := s.SetCharacter("Save0001.sav", map[string]any{"level": -5, "money": -1})
	if err != nil {
		t.Fatalf("SetCharacter with clamps: %v", err)
	}
	if cv.Level < 1 {
		t.Fatalf("level clamped below 1: %+v", cv)
	}
	if cv.Level != before.Character.Level {
		t.Fatalf("out-of-range level should leave level unchanged: before=%d got=%d", before.Character.Level, cv.Level)
	}
	if cv.Money != 0 {
		t.Fatalf("expected money clamped to 0, got %+v", cv)
	}
}

func TestSetCharacterBadFile(t *testing.T) {
	s := testSession(t, "")
	if _, err := s.SetCharacter("../../etc/passwd", map[string]any{"level": 20}); err == nil {
		t.Fatal("expected rejection of traversal name")
	}
}

func TestDeleteItemShrinks(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	victim := before.Inventory["items"][0]
	after, err := s.DeleteItem("Save0001.sav", 53, victim.Index)
	if err != nil {
		t.Fatalf("DeleteItem: %v", err)
	}
	if got := len(after["items"]); got != len(before.Inventory["items"])-1 {
		t.Fatalf("expected items %d -> %d, got %d", len(before.Inventory["items"]), len(before.Inventory["items"])-1, got)
	}
}

func TestDuplicateItemGrows(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	src := before.Inventory["weapons"][0]
	after, err := s.DuplicateItem("Save0001.sav", 54, src.Index)
	if err != nil {
		t.Fatalf("DuplicateItem: %v", err)
	}
	if got := len(after["weapons"]); got != len(before.Inventory["weapons"])+1 {
		t.Fatalf("expected weapons %d -> %d, got %d", len(before.Inventory["weapons"]), len(before.Inventory["weapons"])+1, got)
	}
}

func TestTransferItemMoves(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	src := before.Inventory["items"][0]
	after, err := s.TransferItem("Save0001.sav", 53, src.Index, 41)
	if err != nil {
		t.Fatalf("TransferItem: %v", err)
	}
	if got := len(after["items"]); got != len(before.Inventory["items"])-1 {
		t.Fatalf("expected items -1: before=%d got=%d", len(before.Inventory["items"]), got)
	}
	if got := len(after["bank"]); got != len(before.Inventory["bank"])+1 {
		t.Fatalf("expected bank +1: before=%d got=%d", len(before.Inventory["bank"]), got)
	}
}

func TestSetItemLevelApplies(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	src := before.Inventory["weapons"][0]
	after, err := s.SetItemLevel("Save0001.sav", 54, src.Index, 50)
	if err != nil {
		t.Fatalf("SetItemLevel: %v", err)
	}
	found := false
	for _, it := range after["weapons"] {
		if it.Index == src.Index {
			found = true
			if it.Level != 50 {
				t.Fatalf("expected level 50, got %+v", it)
			}
		}
	}
	if !found {
		t.Fatalf("mutated weapon index %d missing: %+v", src.Index, after["weapons"])
	}
}

func TestItemMutationsRejectBad(t *testing.T) {
	s := testSession(t, "")
	seedTestItems(t, s)
	if _, err := s.DeleteItem("Save0001.sav", 53, -1); err == nil {
		t.Fatal("expected error for DeleteItem index -1")
	}
	if _, err := s.DuplicateItem("Save0001.sav", 54, -1); err == nil {
		t.Fatal("expected error for DuplicateItem index -1")
	}
	before, err := s.OpenSave("Save0001.sav")
	if err != nil {
		t.Fatalf("OpenSave: %v", err)
	}
	src := before.Inventory["items"][0]
	if _, err := s.TransferItem("Save0001.sav", 53, src.Index, 53); err == nil {
		t.Fatal("expected error for same-field transfer")
	}
	if _, err := s.SetItemLevel("Save0001.sav", 54, -1, 50); err == nil {
		t.Fatal("expected error for SetItemLevel index -1")
	}
}

// seedTestItems plants one synthetic weapon + one synthetic item in the
// TempDir fixture copy (the fixture itself holds only fake placeholder
// entries). Values avoid the fake-item marker (values[0] == 255).
func seedTestItems(t *testing.T, s *Session) {
	t.Helper()
	weapon := []int64{0, 1, 2, 3, 10, 10, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
	if ok, err := s.store.AddWeapon("Save0001.sav", weapon); err != nil || !ok {
		t.Fatalf("AddWeapon: ok=%v err=%v", ok, err)
	}
	item := []int64{0, 1, 2, 3, 10, 10, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
	if ok, err := s.store.AddItem("Save0001.sav", item); err != nil || !ok {
		t.Fatalf("AddItem: ok=%v err=%v", ok, err)
	}
}
