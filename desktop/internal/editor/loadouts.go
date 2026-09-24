package editor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bl2save/desktop/internal/bl2save"
)

type loadoutFile struct {
	Name      string   `json:"name"`
	Character string   `json:"character"`
	Level     int      `json:"level"`
	SaveFile  string   `json:"save_file"`
	Weapons   []string `json:"weapons"`
	Items     []string `json:"items"`
}

var loadoutSafeName = regexp.MustCompile(`[^\w\s\-]`)

func (s *Store) ensureLoadoutDir() error {
	return os.MkdirAll(s.LoadoutDir, 0o755)
}

// SaveLoadout snapshots the current backpack as a named JSON loadout.
func (s *Store) SaveLoadout(filename, name string) (map[string]any, error) {
	if err := s.ensureLoadoutDir(); err != nil {
		return nil, err
	}
	codes, err := s.ExportAllCodes(filename)
	if err != nil {
		return nil, err
	}
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	charInfo := s.ExtractCharacterInfo(tree)

	className := "Unknown"
	if v, ok := charInfo["class_name"].(string); ok {
		className = v
	}
	level := 0
	if v, ok := charInfo["level"].(uint64); ok {
		level = int(v)
	}
	weapons := codes["weapons"]
	items := codes["items"]
	loadout := loadoutFile{
		Name:      name,
		Character: className,
		Level:     level,
		SaveFile:  filename,
		Weapons:   weapons,
		Items:     items,
	}

	safeName := loadoutSafeName.ReplaceAllString(name, "")
	safeName = regexp.MustCompile(`\s+`).ReplaceAllString(safeName, "_")
	safeName = regexp.MustCompile(`^_+|_+$`).ReplaceAllString(safeName, "")
	if safeName == "" {
		safeName = "loadout"
	}
	path := filepath.Join(s.LoadoutDir, safeName+".json")
	counter := 1
	for {
		if _, err := os.Stat(path); err != nil {
			break
		}
		path = filepath.Join(s.LoadoutDir, safeName+"_"+itoa(counter)+".json")
		counter++
	}
	data, err := json.MarshalIndent(loadout, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"file":    filepath.Base(path),
		"name":    name,
		"weapons": len(weapons),
		"items":   len(items),
	}, nil
}

// ListLoadouts enumerates saved loadout files.
func (s *Store) ListLoadouts() ([]map[string]any, error) {
	if err := s.ensureLoadoutDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.LoadoutDir)
	if err != nil {
		return nil, err
	}
	results := []map[string]any{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.LoadoutDir, name))
		if err != nil {
			continue
		}
		var lo loadoutFile
		if err := json.Unmarshal(data, &lo); err != nil {
			continue
		}
		results = append(results, map[string]any{
			"file":      name,
			"name":      orDefault(lo.Name, name),
			"character": orDefault(lo.Character, "?"),
			"level":     lo.Level,
			"weapons":   len(lo.Weapons),
			"items":     len(lo.Items),
		})
	}
	return results, nil
}

// LoadLoadout restores a loadout; replace clears non-fake weapons/items first.
func (s *Store) LoadLoadout(filename, loadoutName string, replace bool) (map[string]any, error) {
	if err := s.ensureLoadoutDir(); err != nil {
		return nil, err
	}
	path := filepath.Join(s.LoadoutDir, filepath.Base(loadoutName))
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("loadout not found: " + loadoutName)
	}
	var lo loadoutFile
	if err := json.Unmarshal(data, &lo); err != nil {
		return nil, err
	}

	allCodes := append(append([]string(nil), lo.Weapons...), lo.Items...)
	if len(allCodes) == 0 {
		return map[string]any{"imported": 0}, nil
	}

	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}

	if replace {
		for _, fieldNum := range []int{54, 53} {
			kept := make([]bl2save.PBEntry, 0, len(tree[fieldNum]))
			for _, entry := range tree[fieldNum] {
				sub, ok := subTree(entry)
				if !ok || len(sub[1]) == 0 {
					kept = append(kept, entry)
					continue
				}
				rawItem, ok := sub[1][0].Value.([]byte)
				if !ok {
					kept = append(kept, entry)
					continue
				}
				isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
				if err == nil && bl2save.IsFakeItem(isWeapon, values) {
					kept = append(kept, entry)
				}
			}
			tree[fieldNum] = kept
		}
	}

	count := 0
	var errMsgs []string
	for lineNum, code := range allCodes {
		code = strings.TrimSpace(code)
		if code == "" || !strings.HasPrefix(code, "BL2(") {
			continue
		}
		codeBytes, err := bl2save.ValidateGibbedCode(code)
		if err != nil {
			errMsgs = append(errMsgs, itoa(lineNum+1)+": "+err.Error())
			continue
		}
		codeBytes, err = bl2save.ReplaceRawItemKey(codeBytes, randomKey())
		if err != nil {
			errMsgs = append(errMsgs, itoa(lineNum+1)+": "+err.Error())
			continue
		}
		if codeBytes[0]&0x80 == 0 {
			entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
				1: {{WireType: 2, Value: codeBytes}},
				2: {{WireType: 0, Value: uint64(1)}},
				3: {{WireType: 0, Value: uint64(0)}},
				4: {{WireType: 0, Value: uint64(1)}},
			})
			tree[53] = append(tree[53], bl2save.PBEntry{WireType: 2, Value: entryBytes})
		} else {
			entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
				1: {{WireType: 2, Value: codeBytes}},
				2: {{WireType: 0, Value: uint64(0)}},
				3: {{WireType: 0, Value: uint64(1)}},
			})
			tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entryBytes})
		}
		count++
	}
	if count > 0 || replace {
		if err := s.WriteSave(filename, tree, false); err != nil {
			return nil, err
		}
	}
	name := lo.Name
	if name == "" {
		name = loadoutName
	}
	return map[string]any{"imported": count, "name": name}, nil
}

// DeleteLoadout removes a loadout file.
func (s *Store) DeleteLoadout(loadoutName string) (bool, error) {
	if err := s.ensureLoadoutDir(); err != nil {
		return false, err
	}
	path := filepath.Join(s.LoadoutDir, filepath.Base(loadoutName))
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}
	if err := os.Remove(path); err != nil {
		return false, err
	}
	return true, nil
}
