package viewer

import (
	"strings"
	"testing"

	"bl2save/desktop/native/setup"
)

// windowTitle mirrors the shell chrome string chain without opening a GL
// window (g3n windows can't open headless).
func TestWindowTitleAssetsMissing(t *testing.T) {
	title := windowTitle(setup.AssetStatus{Dir: "/assets"})
	if !strings.Contains(title, "assets") {
		t.Fatalf("missing-assets title should mention assets: %q", title)
	}
}

func TestWindowTitleNoModels(t *testing.T) {
	title := windowTitle(setup.AssetStatus{Dir: "/assets", OK: true})
	if !strings.Contains(title, "No .gltf") {
		t.Fatalf("no-models title should mention No .gltf: %q", title)
	}
}

func TestWindowTitleOK(t *testing.T) {
	title := windowTitle(setup.AssetStatus{Dir: "/assets", OK: true, ModelCount: 3})
	if strings.Contains(title, "assets") || strings.Contains(title, "No .gltf") {
		t.Fatalf("OK status should give a plain title: %q", title)
	}
}
