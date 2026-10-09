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
