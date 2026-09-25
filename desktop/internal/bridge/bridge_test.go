package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
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
	// The handlers gate on a config file existing next to the binary
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

func invoke(t *testing.T, b *Bridge, method, path, body string) (any, *apiError) {
	t.Helper()
	raw, err := b.Invoke(method, path, body)
	if err != nil {
		t.Fatalf("Invoke %s %s: %v", method, path, err)
	}
	var out any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if m, ok := out.(map[string]any); ok {
		if e, hasErr := m["error"]; hasErr {
			return out, &apiError{msg: e.(string)}
		}
	}
	return out, nil
}

func TestRouterListSaves(t *testing.T) {
	b := testBridge(t)
	out, ae := invoke(t, b, "GET", "/api/saves", "")
	if ae != nil {
		t.Fatalf("api error: %s", ae.msg)
	}
	saves, ok := out.([]any)
	if !ok || len(saves) != 1 {
		t.Fatalf("expected 1 save, got %v", out)
	}
	first := saves[0].(map[string]any)
	if first["filename"] != "Save0001.sav" {
		t.Fatalf("unexpected filename: %v", first)
	}
}

func TestRouterLoadSave(t *testing.T) {
	b := testBridge(t)
	out, ae := invoke(t, b, "GET", "/api/save/Save0001.sav", "")
	if ae != nil {
		t.Fatalf("api error: %s", ae.msg)
	}
	m := out.(map[string]any)
	if m["character"] == nil || m["inventory"] == nil || m["missions"] == nil {
		t.Fatalf("missing sections: %v", keysOf(m))
	}
}

func TestRouterRejectsBadFilename(t *testing.T) {
	b := testBridge(t)
	_, ae := invoke(t, b, "GET", "/api/save/evil.sav", "")
	if ae == nil {
		t.Fatal("expected filename rejection")
	}
	_, ae = invoke(t, b, "DELETE", "/api/save/../../etc/passwd/delete", "")
	if ae == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestRouterMissionDB(t *testing.T) {
	b := testBridge(t)
	out, ae := invoke(t, b, "GET", "/api/missions/db", "")
	if ae != nil {
		t.Fatalf("api error: %s", ae.msg)
	}
	if len(out.([]any)) == 0 {
		t.Fatal("empty mission db")
	}
}

func TestRouterUnknownRoute(t *testing.T) {
	b := testBridge(t)
	_, ae := invoke(t, b, "GET", "/api/nonexistent", "")
	if ae == nil {
		t.Fatal("expected 404-equivalent error")
	}
}

func TestRouterGameStatus(t *testing.T) {
	b := testBridge(t)
	out, ae := invoke(t, b, "GET", "/api/game-status", "")
	if ae != nil {
		t.Fatalf("api error: %s", ae.msg)
	}
	m := out.(map[string]any)
	if _, ok := m["running"].(bool); !ok {
		t.Fatalf("missing running flag: %v", out)
	}
}

func keysOf(m map[string]any) []string {
	keys := []string{}
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
