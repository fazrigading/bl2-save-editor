package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProbeMissingDir(t *testing.T) {
	st := ProbeAssetDir(filepath.Join(t.TempDir(), "nope"))
	if st.OK {
		t.Fatalf("expected not-OK: %+v", st)
	}
	if !hasMissing(st.Missing, "models") {
		t.Fatalf("expected models in missing: %+v", st)
	}
}

func TestProbeEmptyDir(t *testing.T) {
	st := ProbeAssetDir(t.TempDir())
	if st.OK {
		t.Fatalf("expected not-OK for empty dir: %+v", st)
	}
}

func TestProbeFindsModel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "w.gltf"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "t.png"), []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	st := ProbeAssetDir(dir)
	if !st.OK {
		t.Fatalf("expected OK: %+v", st)
	}
	if st.ModelCount != 1 || st.TextureCount != 1 {
		t.Fatalf("bad counts: %+v", st)
	}
}

func TestDefaultAssetDirRespectsEnv(t *testing.T) {
	t.Setenv("BL2_NATIVE_ASSETS", "/tmp/custom-assets")
	if got := DefaultAssetDir(); got != "/tmp/custom-assets" {
		t.Fatalf("env override ignored: %q", got)
	}
}

func hasMissing(m []string, want string) bool {
	for _, s := range m {
		if strings.Contains(s, want) {
			return true
		}
	}
	return false
}
