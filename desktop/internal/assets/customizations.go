package assets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var classPrefixes = map[string][]string{
	"Axton":    {"Soldier"},
	"Zer0":     {"Assassin"},
	"Maya":     {"Siren"},
	"Salvador": {"Mercenary", "Merc"},
	"Gaige":    {"Mechro"},
	"Krieg":    {"Psycho"},
}

// Customizations returns heads and skins for a character class, parsed from
// the Gibbed Items.json (ported from app.py's /api/assets/customizations).
func (db *DB) Customizations(className string) map[string]any {
	db.ensureLoaded()
	empty := map[string]any{"heads": []map[string]string{}, "skins": []map[string]string{}}
	if db.gibbedDir == "" {
		return empty
	}
	data, err := os.ReadFile(filepath.Join(db.gibbedDir, "Items.json"))
	if err != nil {
		return empty
	}
	var items map[string]map[string]any
	if err := json.Unmarshal(data, &items); err != nil {
		return empty
	}

	prefixes := classPrefixes[className]
	heads := []map[string]string{}
	skins := []map[string]string{}
	for path, info := range items {
		if info == nil {
			continue
		}
		matched := false
		for _, p := range prefixes {
			if p != "" && strings.Contains(path, p) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		name := jsonStr(info, "name")
		if name == "" {
			name = lastDotSegment(path)
		}
		if strings.Contains(path, ".Head_") {
			heads = append(heads, map[string]string{"path": path, "name": name})
		} else if strings.Contains(path, ".Skin_") {
			skins = append(skins, map[string]string{"path": path, "name": name})
		}
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i]["name"] < heads[j]["name"] })
	sort.Slice(skins, func(i, j int) bool { return skins[i]["name"] < skins[j]["name"] })
	return map[string]any{"heads": heads, "skins": skins}
}
