// Package session is the headless core of the native editor: loading,
// opening, and validating saves. It has no GUI dependency so it is fully
// unit-testable; desktop/native/ui renders what it returns.
package session

import (
	"errors"
	"path/filepath"
	"strings"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/editor"
	"bl2save/desktop/internal/platform"
)

// SaveSummary is one row of the save list.
type SaveSummary struct {
	Filename string
	SizeKB   float64
	Modified int64
}

// Color is one ARGB appearance zone.
type Color struct {
	A, R, G, B uint64
}

// CharacterView is the character summary the UI renders.
type CharacterView struct {
	Class, ClassName, Name, HeadAsset, SkinAsset string
	Level                                       uint64
	Colors                                      []Color
}

// SaveView is the opened save the UI renders.
type SaveView struct {
	Filename  string
	Character CharacterView
}

// Session owns the backend state. A zero/invalid config yields a session
// whose methods return setup errors instead of panicking.
type Session struct {
	cfg   *platform.Config
	store *editor.Store
	adb   *assets.DB
}

// New builds a session from an in-memory config.
func New(cfg *platform.Config) *Session {
	s := &Session{cfg: cfg}
	if cfg != nil && cfg.SaveDir != "" {
		gens := cfg.BackupGenerations
		s.store = editor.NewStore(cfg.SaveDir, gens,
			filepath.Join(cfg.SaveDir, "loadouts"))
		s.adb = assets.New(cfg.GibbedDir)
	}
	return s
}

func (s *Session) requireStore() (*editor.Store, error) {
	if s.store == nil {
		return nil, errors.New("editor is not configured — complete setup first")
	}
	return s.store, nil
}

// validSaveFilename mirrors the backend's save-name rule and additionally
// rejects anything that is not a bare filename (path traversal guard).
func validSaveFilename(filename string) bool {
	if filepath.Base(filename) != filename {
		return false
	}
	if len(filename) != 12 || !strings.HasPrefix(filename, "Save") ||
		!strings.HasSuffix(filename, ".sav") {
		return false
	}
	for _, c := range filename[4:8] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// ListSaves enumerates saves in the configured directory.
func (s *Session) ListSaves() ([]SaveSummary, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	raw, err := store.ListSaves()
	if err != nil {
		return nil, err
	}
	out := make([]SaveSummary, 0, len(raw))
	for _, m := range raw {
		name, _ := m["filename"].(string)
		size, _ := m["size_kb"].(float64)
		var modified int64
		switch v := m["modified"].(type) {
		case int64:
			modified = v
		case int:
			modified = int64(v)
		}
		out = append(out, SaveSummary{Filename: name, SizeKB: size, Modified: modified})
	}
	return out, nil
}

func toUint(v any) uint64 {
	switch n := v.(type) {
	case uint64:
		return n
	case uint32:
		return uint64(n)
	case int:
		if n < 0 {
			return 0
		}
		return uint64(n)
	case int64:
		if n < 0 {
			return 0
		}
		return uint64(n)
	}
	return 0
}

// OpenSave reads a save and builds its view-model.
func (s *Session) OpenSave(filename string) (*SaveView, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	if !validSaveFilename(filename) {
		return nil, errors.New("invalid save filename")
	}
	_, tree, err := store.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	info := store.ExtractCharacterInfo(tree)
	class, _ := info["class"].(string)
	className, _ := info["class_name"].(string)
	name, _ := info["name"].(string)
	head, _ := info["head_asset"].(string)
	skin, _ := info["skin_asset"].(string)
	var colors []Color
	if raw, ok := info["appearance_colors"].([]map[string]any); ok {
		for _, c := range raw {
			colors = append(colors, Color{
				A: toUint(c["a"]), R: toUint(c["r"]),
				G: toUint(c["g"]), B: toUint(c["b"]),
			})
		}
	}
	return &SaveView{
		Filename: filename,
		Character: CharacterView{
			Class: class, ClassName: className, Name: name,
			Level:      toUint(info["level"]),
			HeadAsset:  head,
			SkinAsset:  skin,
			Colors:     colors,
		},
	}, nil
}
