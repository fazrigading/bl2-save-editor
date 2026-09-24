package editor

import (
	"errors"

	"bl2save/desktop/internal/bl2save"
)

func toFloat(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

// RefResolver packs asset paths into (lib, asset) references.
type RefResolver interface {
	PackReference(path, group string, setID int) (int, int, bool)
	GetSetIDForPath(path, group string) int
}

// SpawnTestWeapons clones the first weapon with grade values above the old
// 80 cap, for verifying the exe patch limits.
func (s *Store) SpawnTestWeapons(filename string) (map[string]any, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	if len(tree[54]) == 0 {
		return nil, errors.New("No weapons to use as template")
	}
	sub, ok := subTree(tree[54][0])
	if !ok || len(sub[1]) == 0 {
		return nil, errors.New("No weapons to use as template")
	}
	rawItem, _ := sub[1][0].Value.([]byte)
	isWeapon, baseVals, _, err := bl2save.UnwrapItem(rawItem)
	if err != nil {
		return nil, err
	}
	if bl2save.IsFakeItem(isWeapon, baseVals) {
		return nil, errors.New("Template weapon is empty")
	}

	type test struct {
		stage int
		grade int
		label string
	}
	tests := []test{
		{1, 85, "grade85_stage1"},
		{1, 100, "grade100_stage1"},
		{1, 110, "grade110_stage1"},
		{1, 120, "grade120_stage1"},
		{1, 127, "grade127_stage1"},
	}

	for _, t := range tests {
		vals := append([]int64(nil), baseVals...)
		vals[4] = int64(t.grade)
		vals[5] = int64(t.stage)
		rawNew := bl2save.WrapItem(isWeapon, vals, randomKey())
		entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
			1: {{WireType: 2, Value: rawNew}},
			2: {{WireType: 0, Value: uint64(0)}},
			3: {{WireType: 0, Value: uint64(1)}},
		})
		tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entryBytes})
	}
	if err := s.WriteSave(filename, tree, false); err != nil {
		return nil, err
	}
	testList := []map[string]any{}
	for _, t := range tests {
		testList = append(testList, map[string]any{"stage": t.stage, "grade": t.grade, "label": t.label})
	}
	return map[string]any{"ok": true, "count": len(tests), "tests": testList}, nil
}

// EditItemValues applies the full item editor changes (ported from
// app.py's api_edit_weapon): type/balance/manufacturer/parts/levels.
func (s *Store) EditItemValues(filename string, field, idx int, changes map[string]any, resolver RefResolver) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if idx < 0 || idx >= len(tree[field]) {
		return false, errors.New("Item not found")
	}
	entry := tree[field][idx]
	sub, ok := subTree(entry)
	if !ok || len(sub[1]) == 0 {
		return false, errors.New("Item not found")
	}
	rawItem, _ := sub[1][0].Value.([]byte)
	isWeapon, itemValues, key, err := bl2save.UnwrapItem(rawItem)
	if err != nil {
		return false, err
	}

	clamp127 := func(v int64) int64 {
		if v < 0 {
			v = 0
		}
		if v > 127 {
			v = 127
		}
		return v
	}

	intChange := func(key string) (int64, bool) {
		if v, ok := changes[key]; ok {
			return clamp127(int64(toFloat(v))), true
		}
		return 0, false
	}

	if v, ok := intChange("game_stage"); ok {
		itemValues[5] = v
	}
	if v, ok := intChange("grade_index"); ok {
		itemValues[4] = v
	}
	if v, ok := changes["level"]; ok {
		if _, hasStage := changes["game_stage"]; !hasStage {
			if _, hasGrade := changes["grade_index"]; !hasGrade {
				lvl := clamp127(int64(toFloat(v)))
				itemValues[4] = lvl
				itemValues[5] = lvl
			}
		}
	}

	var partGroup, typeGroup = "ItemParts", "ItemTypes"
	if isWeapon == 1 {
		typeGroup = "WeaponTypes"
		partGroup = "WeaponParts"
	}
	if v, ok := changes["type"].(string); ok && v != "" {
		if refLib, refAsset, refOK := resolver.PackReference(v, typeGroup, 0); refOK {
			itemValues[1] = int64(refLib<<uint(assetBits[typeGroup]) | refAsset)
		}
	}
	if v, ok := changes["balance"].(string); ok && v != "" {
		if refLib, refAsset, refOK := resolver.PackReference(v, "BalanceDefs", 0); refOK {
			itemValues[2] = int64(refLib<<10 | refAsset)
			itemValues[0] = int64(resolver.GetSetIDForPath(v, "BalanceDefs"))
		}
	}
	if v, ok := changes["manufacturer"].(string); ok && v != "" {
		if refLib, refAsset, refOK := resolver.PackReference(v, "Manufacturers", 0); refOK {
			itemValues[3] = int64(refLib<<7 | refAsset)
		}
	}

	partBits := assetBits[partGroup]
	noneVal := (int64(1) << uint(sublibraryBits[partGroup]+partBits)) - 1

	slotOrder := []string{"body", "grip", "barrel", "sight", "stock", "elemental", "accessory1", "accessory2", "material", "prefix", "title"}
	if partsData, ok := changes["parts"].(map[string]any); ok {
		for i, slot := range slotOrder {
			if raw, present := partsData[slot]; present {
				partPath, _ := raw.(string)
				if partPath == "" || partPath == "__keep__" || partPath == "__loading__" {
					continue
				}
				if partPath == "none" {
					itemValues[6+i] = noneVal
					continue
				}
				if refLib, refAsset, refOK := resolver.PackReference(partPath, partGroup, 0); refOK {
					itemValues[6+i] = int64(refLib<<uint(partBits) | refAsset)
				}
			}
		}
	}

	sub[1][0].Value = bl2save.WrapItem(isWeapon, itemValues, key)
	tree[field][idx].Value, _ = bl2save.WriteProtobuf(sub)
	return true, s.WriteSave(filename, tree, true)
}

var assetBits = map[string]int{
	"WeaponTypes": 6, "WeaponParts": 11, "ItemTypes": 8,
	"ItemParts": 10, "Manufacturers": 7, "BalanceDefs": 10,
}

var sublibraryBits = map[string]int{
	"WeaponTypes": 7, "WeaponParts": 6, "ItemTypes": 9,
	"ItemParts": 6, "Manufacturers": 4, "BalanceDefs": 10,
}

// AllStations returns every known fast travel station name.
func (s *Store) AllStations() []string { return AllStations }

// StationDisplay returns the display name for a station.
func (s *Store) StationDisplay(station string) string {
	return orDefault(stationDisplay[station], station)
}

// AchievementList exposes the embedded achievement database.
func AchievementList() []achievementEntry { return AchievementDB }
