package session

import (
	"errors"
	"fmt"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/editor"
	"bl2save/desktop/internal/platform"
)

// AchievementRow is one achievements-tab row. Unlocked is nil in save-state
// mode (the DB has no per-save unlock state).
type AchievementRow struct {
	APIName, Name, Desc, Category, CategoryName string
	Unlocked                                    *bool
}

// categoryDisplay maps a DB category slug to a display name
// (parity with bridge/steam.go).
func categoryDisplay(cat string) string {
	names := map[string]string{
		"story": "Story", "exploration": "Exploration", "combat": "Combat",
		"challenge": "Challenge", "misc": "Miscellaneous",
		"scarlett": "Captain Scarlett", "torgue": "Mr. Torgue",
		"hammerlock": "Sir Hammerlock", "tina": "Tiny Tina",
		"dlc": "Fight for Sanctuary",
	}
	if n, ok := names[cat]; ok {
		return n
	}
	return cat
}

// GetSkills returns {skill path: level} for the save.
func (s *Session) GetSkills(filename string) (map[string]uint64, error) {
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
	skills, _ := info["skills"].(map[string]uint64)
	return skills, nil
}

// SetSkills applies {skill path: level} updates and returns the count changed.
func (s *Session) SetSkills(filename string, updates map[string]int64) (int, error) {
	if platform.IsGameRunning() {
		return 0, errors.New("Borderlands 2 is running. Close the game before editing saves.")
	}
	store, err := s.requireStore()
	if err != nil {
		return 0, err
	}
	if !validSaveFilename(filename) {
		return 0, errors.New("invalid save filename")
	}
	return store.SetSkills(filename, updates)
}

// ListAchievements returns the achievement DB rows in save-state mode
// (Unlocked always nil).
func (s *Session) ListAchievements() ([]AchievementRow, error) {
	if s.store == nil {
		return nil, errors.New("editor is not configured — complete setup first")
	}
	out := []AchievementRow{}
	for _, a := range editor.AchievementList() {
		out = append(out, AchievementRow{
			APIName: a.APIName, Name: a.Name, Desc: a.Desc,
			Category: a.Category, CategoryName: categoryDisplay(a.Category),
		})
	}
	return out, nil
}

// EnableAchievements writes save state satisfying all save-trackable
// achievement conditions (MAX CHARACTER) and returns the wrapped result.
func (s *Session) EnableAchievements(filename string) (map[string]any, error) {
	if platform.IsGameRunning() {
		return nil, errors.New("Borderlands 2 is running. Close the game before editing saves.")
	}
	store, err := s.requireStore()
	if err != nil {
		return nil, err
	}
	if !validSaveFilename(filename) {
		return nil, errors.New("invalid save filename")
	}
	return store.UnlockAchievements(filename)
}

// UnlockPlaythrough unlocks TVHM ("tvhm") or UVHM ("uvhm"); returns false
// for unknown targets (no error).
func (s *Session) UnlockPlaythrough(filename, target string) (bool, error) {
	if platform.IsGameRunning() {
		return false, errors.New("Borderlands 2 is running. Close the game before editing saves.")
	}
	store, err := s.requireStore()
	if err != nil {
		return false, err
	}
	if !validSaveFilename(filename) {
		return false, errors.New("invalid save filename")
	}
	return store.UnlockPlaythrough(filename, target)
}

// DetailParts resolves one inventory row's type path and part paths by
// re-reading the save (same extraction buildItemView uses).
func (s *Session) DetailParts(filename string, field, index int) (types, parts []string, err error) {
	store, err := s.requireStore()
	if err != nil {
		return nil, nil, err
	}
	if !validSaveFilename(filename) {
		return nil, nil, errors.New("invalid save filename")
	}
	_, tree, err := store.ReadSave(filename)
	if err != nil {
		return nil, nil, err
	}
	for _, row := range inventoryRows(store.ExtractInventory(tree)) {
		if toInt(row["field"]) != field || toInt(row["index"]) != index {
			continue
		}
		info, ok := row["info"].(map[string]any)
		if !ok {
			// ExtractInventory returns *bl2save.ItemInfo; roundTrip converts
			// it via JSON tags (same as buildItemView).
			info = roundTrip(row["info"])
			if len(info) == 0 {
				return nil, nil, errors.New("item not found")
			}
		}
		resolved := s.adb.ResolveItemParts(info)
		types, parts = assets.TypesAndParts(resolved)
		return types, parts, nil
	}
	return nil, nil, fmt.Errorf("item not found: field %d index %d", field, index)
}

// inventoryRows flattens ExtractInventory output into a row slice.
func inventoryRows(raw map[string]any) []map[string]any {
	var out []map[string]any
	for _, rows := range raw {
		if list, ok := rows.([]map[string]any); ok {
			out = append(out, list...)
		}
	}
	return out
}
