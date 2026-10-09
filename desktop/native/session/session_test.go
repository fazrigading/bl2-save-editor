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
