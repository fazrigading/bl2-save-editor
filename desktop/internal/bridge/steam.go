package bridge

import (
	"errors"

	"bl2save/desktop/internal/editor"
)

// Steam serves the achievements tab. Achievements are written into the save
// file (save-state mode), so the interactive endpoints are stubs that point
// at the save-scoped operations.
type Steam struct{ b *Bridge }

func (s *Steam) Status() map[string]any {
	return map[string]any{
		"available":   true,
		"initialized": true,
		"healthy":     true,
		"mode":        "save-state",
		"message":     "Achievements are written to the save file",
	}
}

func (s *Steam) Init() map[string]any {
	return map[string]any{
		"ok":      true,
		"message": "Save-state mode: achievements are written to the save file",
	}
}

func (s *Steam) Achievements() []map[string]any {
	out := []map[string]any{}
	for _, a := range editor.AchievementList() {
		out = append(out, map[string]any{
			"api_name":      a.APIName,
			"name":          a.Name,
			"description":   a.Desc,
			"category":      a.Category,
			"category_name": categoryDisplay(a.Category),
			"unlocked":      nil,
		})
	}
	return out
}

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

func (s *Steam) Unlock() error {
	return errors.New("Save-state mode: use 'Unlock All' to write achievement state into the loaded save")
}

func (s *Steam) UnlockAll() error {
	return errors.New("Save-state mode: use the MAX CHARACTER button on a loaded save")
}

func (s *Steam) Clear() error {
	return errors.New("Save-state mode: achievements cannot be re-locked from the save")
}
