package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGibbedMissingAndInstall(t *testing.T) {
	dir := t.TempDir()
	missing := GibbedMissing(dir)
	if len(missing) != len(GibbedFiles) {
		t.Fatalf("expected all %d files missing, got %d", len(GibbedFiles), len(missing))
	}

	fetched := []string{}
	orig := gibbedFetch
	gibbedFetch = func(name string) ([]byte, error) {
		fetched = append(fetched, name)
		return []byte(`{"ok": true, "pad": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`), nil
	}
	t.Cleanup(func() { gibbedFetch = orig })

	if err := DownloadGibbedData(dir, nil); err != nil {
		t.Fatalf("DownloadGibbedData: %v", err)
	}
	if len(fetched) != len(GibbedFiles) {
		t.Fatalf("expected %d fetches, got %d", len(GibbedFiles), len(fetched))
	}
	if missing = GibbedMissing(dir); len(missing) != 0 {
		t.Fatalf("expected nothing missing after install: %v", missing)
	}

	// Idempotent: nothing to fetch when everything is present.
	fetched = nil
	if err := DownloadGibbedData(dir, nil); err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 0 {
		t.Fatalf("expected 0 fetches on rerun, got %d", len(fetched))
	}

	// Corrupt file is re-fetched.
	if err := os.WriteFile(filepath.Join(dir, "Items.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if missing = GibbedMissing(dir); len(missing) != 1 || missing[0] != "Items.json" {
		t.Fatalf("expected Items.json missing, got %v", missing)
	}
	if err := DownloadGibbedData(dir, nil); err != nil {
		t.Fatal(err)
	}
	if !gibbedFileValid(filepath.Join(dir, "Items.json")) {
		t.Fatal("Items.json not repaired")
	}
}

func TestGibbedFetchErrorPropagates(t *testing.T) {
	orig := gibbedFetch
	gibbedFetch = func(name string) ([]byte, error) {
		return nil, os.ErrPermission
	}
	t.Cleanup(func() { gibbedFetch = orig })
	if err := DownloadGibbedData(t.TempDir(), nil); err == nil {
		t.Fatal("expected error to propagate")
	}
}
