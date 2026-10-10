// Package session is the headless core of the native editor: loading,
// opening, and validating saves. It has no GUI dependency so it is fully
// unit-testable; desktop/native/ui renders what it returns.
package session

import (
	"encoding/json"
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
	Level                                        uint64
	Experience, SkillPoints                      uint64
	Money, Eridium, Seraph, Torgue, GoldenKeys   uint64
	InventorySize, WeaponSlots, BankSize         uint64
	OpLevel                                      uint64 // Mayhem/OP level, 0-10
	PlaythroughsCompleted                        uint64 // read-only badge
	TimePlayed                                   uint64 // read-only badge (seconds)
	Colors                                       []Color
}

// ItemView is one inventory row the UI renders.
type ItemView struct {
	Index, Field                                      int
	DisplayName, RarityName, RarityColor, ElementName string
	Level                                             int
	Manufacturer, Category                            string
	IsWeapon                                          bool
	Stats                                             map[string]any // estimated_stats for weapons, nil otherwise
}

// SaveView is the opened save the UI renders.
type SaveView struct {
	Filename  string
	Character CharacterView
	Inventory map[string][]ItemView
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

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case uint32:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// roundTrip maps a struct through JSON so map access uses json tags.
func roundTrip(v any) map[string]any {
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func strMap(m map[string]any, key string) map[string]any {
	sub, _ := m[key].(map[string]any)
	return sub
}

// buildItemView resolves one raw inventory entry; missing keys yield zero
// values, never an error.
func (s *Session) buildItemView(raw map[string]any) ItemView {
	view := ItemView{
		Index: toInt(raw["index"]),
		Field: toInt(raw["field"]),
	}
	info := roundTrip(raw["info"])
	resolved := s.adb.ResolveItemParts(info)
	r := roundTrip(resolved)
	view.DisplayName, _ = r["display_name"].(string)
	rarity := strMap(r, "rarity")
	view.RarityName, _ = rarity["name"].(string)
	view.RarityColor, _ = rarity["color"].(string)
	if el := strMap(r, "element"); el != nil {
		view.ElementName, _ = el["name"].(string)
	}
	if lv, ok := info["level"].([]any); ok && len(lv) > 0 {
		view.Level = toInt(lv[0])
	}
	view.Manufacturer, _ = r["manufacturer_name"].(string)
	view.Category, _ = r["category"].(string)
	view.IsWeapon = toInt(info["is_weapon"]) != 0
	if view.IsWeapon {
		view.Stats = strMap(r, "estimated_stats")
	}
	return view
}

// buildInventory maps raw ExtractInventory output to view-models.
func (s *Session) buildInventory(raw map[string]any) map[string][]ItemView {
	inventory := map[string][]ItemView{}
	for key, rows := range raw {
		views := []ItemView{}
		if list, ok := rows.([]map[string]any); ok {
			for _, item := range list {
				views = append(views, s.buildItemView(item))
			}
		}
		inventory[key] = views
	}
	return inventory
}

// openInventory re-reads a save and rebuilds its inventory views.
func (s *Session) openInventory(filename string) (map[string][]ItemView, error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	_, tree, err := store.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	return s.buildInventory(store.ExtractInventory(tree)), nil
}

// mutate is the shared item-mutation path: guard → validate → call →
// re-extract. A store rejection (false) becomes errMsg, never a silent
// no-op.
func (s *Session) mutate(filename string, op func(*editor.Store) (bool, error), errMsg string) (map[string][]ItemView, error) {
	if platform.IsGameRunning() {
		return nil, errors.New("Borderlands 2 is running. Close the game before editing saves.")
	}
	if !validSaveFilename(filename) {
		return nil, errors.New("invalid save filename")
	}
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	ok, err := op(store)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New(errMsg)
	}
	return s.openInventory(filename)
}

// DeleteItem removes an item and returns the fresh inventory.
func (s *Session) DeleteItem(filename string, field, index int) (map[string][]ItemView, error) {
	return s.mutate(filename, func(store *editor.Store) (bool, error) {
		return store.DeleteItem(filename, field, index)
	}, "Item not found")
}

// DuplicateItem clones an item and returns the fresh inventory.
func (s *Session) DuplicateItem(filename string, field, index int) (map[string][]ItemView, error) {
	return s.mutate(filename, func(store *editor.Store) (bool, error) {
		return store.DuplicateItem(filename, field, index)
	}, "Item not found")
}

// TransferItem moves an item between sections and returns the fresh inventory.
func (s *Session) TransferItem(filename string, fromField, index, toField int) (map[string][]ItemView, error) {
	return s.mutate(filename, func(store *editor.Store) (bool, error) {
		return store.TransferItem(filename, fromField, index, toField)
	}, "Transfer failed")
}

// SetItemLevel rewrites an item's level and returns the fresh inventory.
func (s *Session) SetItemLevel(filename string, field, index, level int) (map[string][]ItemView, error) {
	return s.mutate(filename, func(store *editor.Store) (bool, error) {
		return store.SetItemLevel(filename, field, index, level)
	}, "Item not found")
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
	inventory := s.buildInventory(store.ExtractInventory(tree))
	return &SaveView{
		Filename: filename,
		Character: CharacterView{
			Class: class, ClassName: className, Name: name,
			Level:                 toUint(info["level"]),
			Experience:            toUint(info["experience"]),
			SkillPoints:           toUint(info["skill_points"]),
			Money:                 toUint(info["money"]),
			Eridium:               toUint(info["eridium"]),
			Seraph:                toUint(info["seraph"]),
			Torgue:                toUint(info["torgue"]),
			GoldenKeys:            toUint(info["golden_keys"]),
			InventorySize:         toUint(info["inventory_size"]),
			WeaponSlots:           toUint(info["weapon_slots"]),
			BankSize:              toUint(info["bank_size"]),
			OpLevel:               toUint(info["op_level"]),
			PlaythroughsCompleted: toUint(info["playthroughs_completed"]),
			TimePlayed:            toUint(info["time_played"]),
			HeadAsset:             head,
			SkinAsset:             skin,
			Colors:                colors,
		},
		Inventory: inventory,
	}, nil
}

// SetCharacter applies character changes and returns the fresh view.
func (s *Session) SetCharacter(filename string, changes map[string]any) (*CharacterView, error) {
	if platform.IsGameRunning() {
		return nil, errors.New("Borderlands 2 is running. Close the game before editing saves.")
	}
	if !validSaveFilename(filename) {
		return nil, errors.New("invalid save filename")
	}
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	if _, err := store.UpdateCharacter(filename, changes); err != nil {
		return nil, err
	}
	sv, err := s.OpenSave(filename)
	if err != nil {
		return nil, err
	}
	return &sv.Character, nil
}
