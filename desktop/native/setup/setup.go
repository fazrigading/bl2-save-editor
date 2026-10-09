// Package setup covers first-launch concerns of the native editor:
// where 3D assets live and whether the required set is present.
// Slice 3 refines ProbeAssetDir with a pipeline manifest; slice 0 only
// distinguishes present/missing/empty.
package setup

import (
	"os"
	"path/filepath"
	"strings"

	"bl2save/desktop/internal/platform"
)

// AssetStatus describes the 3D asset directory state.
type AssetStatus struct {
	Dir                          string
	ModelCount, TextureCount, MaterialCount int
	OK                           bool
	Missing                      []string
}

// DefaultAssetDir is the asset location: $BL2_NATIVE_ASSETS override,
// else "bl2models" beside config.json.
func DefaultAssetDir() string {
	if env := os.Getenv("BL2_NATIVE_ASSETS"); env != "" {
		return env
	}
	return filepath.Join(filepath.Dir(platform.ConfigPath()), "bl2models")
}

// ProbeAssetDir counts models (.gltf/.glb), textures (.png/.jpg) and
// material sidecars (.json) recursively. Slice-0 OK rule: dir exists and
// at least one model file is present.
func ProbeAssetDir(dir string) AssetStatus {
	st := AssetStatus{Dir: dir}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".gltf", ".glb":
			st.ModelCount++
		case ".png", ".jpg", ".jpeg":
			st.TextureCount++
		case ".json":
			st.MaterialCount++
		}
		return nil
	})
	if err != nil {
		st.Missing = []string{"models", "textures", "materials"}
		return st
	}
	if st.ModelCount == 0 {
		st.Missing = append(st.Missing, "models")
	}
	if st.TextureCount == 0 {
		st.Missing = append(st.Missing, "textures")
	}
	if st.MaterialCount == 0 {
		st.Missing = append(st.Missing, "materials")
	}
	st.OK = st.ModelCount > 0
	return st
}
