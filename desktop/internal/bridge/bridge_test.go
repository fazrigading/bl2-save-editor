package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/editor"
	"bl2save/desktop/internal/platform"
)

func testBridge(t *testing.T) *Bridge {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile("../../../tests/Save0001.sav")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	// The services gate on a config file existing next to the binary
	// (Python parity), so materialize one for the test run.
	if err := os.WriteFile(platform.ConfigPath(), []byte(`{"save_dir":"`+dir+`","backup_generations":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	b := New()
	b.cfg = &platform.Config{SaveDir: dir, BackupGenerations: 5}
	b.store = editor.NewStore(dir, 5, filepath.Join(dir, "loadouts"))
	b.adb = assets.New("")
	return b
}

func TestSavesListSaves(t *testing.T) {
	sv := &Saves{testBridge(t)}
	saves, err := sv.ListSaves()
	if err != nil {
		t.Fatalf("ListSaves: %v", err)
	}
	if len(saves) != 1 {
		t.Fatalf("expected 1 save, got %v", saves)
	}
	if saves[0]["filename"] != "Save0001.sav" {
		t.Fatalf("unexpected filename: %v", saves[0])
	}
}

func TestSavesLoadSave(t *testing.T) {
	sv := &Saves{testBridge(t)}
	state, err := sv.LoadSave("Save0001.sav")
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	if state["character"] == nil || state["inventory"] == nil || state["missions"] == nil {
		t.Fatalf("missing sections: %v", keysOf(state))
	}
}

func TestSavesRejectsBadFilename(t *testing.T) {
	sv := &Saves{testBridge(t)}
	if _, err := sv.LoadSave("evil.sav"); err == nil {
		t.Fatal("expected filename rejection")
	}
	if _, err := sv.DeleteSave("../../etc/passwd"); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestEditorMissionDB(t *testing.T) {
	ed := &Editor{testBridge(t)}
	db, err := ed.MissionDB()
	if err != nil {
		t.Fatalf("MissionDB: %v", err)
	}
	missions, ok := db.([]map[string]any)
	if !ok || len(missions) == 0 {
		t.Fatalf("empty mission db: %T", db)
	}
}

func TestAppConfigPaths(t *testing.T) {
	ap := &App{testBridge(t)}
	paths := ap.ConfigPaths()
	if paths["save_dir"] == "" || paths["config_path"] == "" {
		t.Fatalf("missing path fields: %v", paths)
	}
	if cp, ok := paths["config_path"].(string); !ok || !strings.Contains(cp, "config.json") {
		t.Fatalf("unexpected config_path: %v", paths["config_path"])
	}
}

func TestAppGameStatus(t *testing.T) {
	ap := &App{testBridge(t)}
	status := ap.GameStatus()
	if _, ok := status["running"].(bool); !ok {
		t.Fatalf("missing running flag: %v", status)
	}
}

func keysOf(m map[string]any) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
