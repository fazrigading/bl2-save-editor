package editor

import (
	"encoding/json"
	"math"
	"math/rand"

	"bl2save/desktop/internal/bl2save"
)

// ExtractCharacterInfo builds the character summary sent to the frontend.
func (s *Store) ExtractCharacterInfo(tree bl2save.PBTree) map[string]any {
	charClass := string(getBytes(tree, 1))
	className := charClass
	if known, ok := classNames[charClass]; ok {
		className = known
	}

	currency := currencyValues(tree)
	currencyAt := func(i int) uint64 {
		if len(currency) > i {
			return currency[i]
		}
		return 0
	}

	inventorySize := uint64(12)
	weaponSlots := uint64(2)
	if len(tree[13]) > 0 {
		if slots, ok := subTree(tree[13][0]); ok {
			inventorySize = getUint(slots, 1, 12)
			weaponSlots = getUint(slots, 2, 2)
		}
	}

	name := ""
	appearanceColors := []map[string]any{}
	if len(tree[19]) > 0 {
		if appearance, ok := subTree(tree[19][0]); ok {
			name = latin1(getBytes(appearance, 1))
			for _, colorField := range []int{2, 3, 4} {
				if len(appearance[colorField]) > 0 {
					if colorPb, ok := subTree(appearance[colorField][0]); ok {
						appearanceColors = append(appearanceColors, map[string]any{
							"a": getUint(colorPb, 1, 255),
							"r": getUint(colorPb, 2, 127),
							"g": getUint(colorPb, 3, 127),
							"b": getUint(colorPb, 4, 127),
						})
					} else {
						appearanceColors = append(appearanceColors, defaultColor())
					}
				} else {
					appearanceColors = append(appearanceColors, defaultColor())
				}
			}
		}
	}

	headAsset := ""
	skinAsset := ""
	if entries := tree[35]; len(entries) > 0 {
		if v, ok := entries[0].Value.([]byte); ok {
			headAsset = latin1(v)
		}
		if len(entries) > 4 {
			if v, ok := entries[4].Value.([]byte); ok {
				skinAsset = latin1(v)
			}
		}
	}

	return map[string]any{
		"class":                  charClass,
		"class_name":             className,
		"level":                  getUint(tree, 2, 1),
		"experience":             getUint(tree, 3, 0),
		"skill_points":           getUint(tree, 4, 0),
		"money":                  currencyAt(0),
		"eridium":                currencyAt(1),
		"seraph":                 currencyAt(2),
		"torgue":                 currencyAt(4),
		"golden_keys":            currencyAt(3),
		"inventory_size":         inventorySize,
		"weapon_slots":           weaponSlots,
		"bank_size":              getUint(tree, 56, 6),
		"name":                   name,
		"playthroughs_completed": getUint(tree, 7, 0),
		"time_played":            getUint(tree, 25, 0),
		"save_game_id":           getUint(tree, 20, 0),
		"op_level":               extractOpLevel(tree),
		"appearance_colors":      appearanceColors,
		"head_asset":             headAsset,
		"skin_asset":             skinAsset,
		"skills":                 extractSkills(tree),
		"ammo":                   extractAmmo(tree),
	}
}

func defaultColor() map[string]any {
	return map[string]any{"a": 255, "r": 127, "g": 127, "b": 127}
}

func extractSkills(tree bl2save.PBTree) map[string]uint64 {
	skills := map[string]uint64{}
	for _, entry := range tree[8] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		name := latin1(getBytes(sub, 1))
		skills[name] = getUint(sub, 2, 0)
	}
	return skills
}

// SetSkills applies {skillPath: level} updates, returning the count changed.
func (s *Store) SetSkills(filename string, updates map[string]int64) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	count := 0
	for i, entry := range tree[8] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		name := latin1(getBytes(sub, 1))
		if lvl, ok := updates[name]; ok {
			if lvl < 0 {
				lvl = 0
			}
			setEntry(sub, 2, 0, uint64(lvl))
			tree[8][i].Value, _ = bl2save.WriteProtobuf(sub)
			count++
		}
	}
	if count > 0 {
		if err := s.WriteSave(filename, tree, false); err != nil {
			return 0, err
		}
	}
	return count, nil
}

func extractAmmo(tree bl2save.PBTree) []map[string]any {
	result := []map[string]any{}
	for _, entry := range tree[11] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		resName := latin1(getBytes(sub, 1))
		short := lastDotSegment(resName)
		qty := float32FromBits(getUint(sub, 3, 0))
		result = append(result, map[string]any{
			"resource":      resName,
			"key":           short,
			"display_name":  orDefault(ammoDisplay[short], short),
			"quantity":      int64(math.Round(float64(qty))),
			"max":           orDefaultInt(ammoMaxes[short], 999),
			"upgrade_level": getUint(sub, 4, 0),
		})
	}
	return result
}

func (s *Store) applyAmmoUpdates(tree bl2save.PBTree, updates map[string]int64) bool {
	changed := false
	for i, entry := range tree[11] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		resName := latin1(getBytes(sub, 1))
		short := lastDotSegment(resName)
		if qty, ok := updates[short]; ok {
			if qty < 0 {
				qty = 0
			}
			packed := math.Float32bits(float32(qty))
			setEntry(sub, 3, 5, uint64(packed))
			tree[11][i].Value, _ = bl2save.WriteProtobuf(sub)
			changed = true
		}
	}
	return changed
}

// SetAmmo sets specific ammo pool quantities.
func (s *Store) SetAmmo(filename string, updates map[string]int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if len(tree[11]) == 0 {
		return false, nil
	}
	if !s.applyAmmoUpdates(tree, updates) {
		return false, nil
	}
	return true, s.WriteSave(filename, tree, false)
}

// FillAllAmmo fills every ammo pool to its max.
func (s *Store) FillAllAmmo(filename string) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if len(tree[11]) == 0 {
		return false, nil
	}
	changed := false
	for _, entry := range tree[11] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		resName := latin1(getBytes(sub, 1))
		short := lastDotSegment(resName)
		maxVal := ammoMaxes[short]
		if maxVal == 0 {
			maxVal = 999
		}
		packed := math.Float32bits(float32(maxVal))
		if getUint(sub, 3, 0) != uint64(packed) {
			setEntry(sub, 3, 5, uint64(packed))
			entry.Value, _ = bl2save.WriteProtobuf(sub)
			changed = true
		}
	}
	if !changed {
		return true, nil
	}
	return true, s.WriteSave(filename, tree, false)
}

func extractOpLevel(tree bl2save.PBTree) uint64 {
	for _, entry := range tree[53] {
		sub, ok := subTree(entry)
		if !ok || len(sub[1]) == 0 || len(sub[2]) == 0 {
			continue
		}
		rawItem, ok := sub[1][0].Value.([]byte)
		if !ok {
			continue
		}
		isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
		if err != nil || !bl2save.IsFakeItem(isWeapon, values) {
			continue
		}
		rawVal := getUint(sub, 2, 0)
		if byte(-rawVal) == 4 {
			return uint64(-int64(rawVal)>>8) & 0x7FFFFF
		}
	}
	return 0
}

// SetOpLevel sets the overpower level via the fake item in field 53.
func (s *Store) SetOpLevel(filename string, opLevel int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if opLevel < 0 {
		opLevel = 0
	}
	if opLevel > 10 {
		opLevel = 10
	}
	for _, entry := range tree[53] {
		sub, ok := subTree(entry)
		if !ok || len(sub[1]) == 0 || len(sub[2]) == 0 {
			continue
		}
		rawItem, ok := sub[1][0].Value.([]byte)
		if !ok {
			continue
		}
		isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
		if err != nil || !bl2save.IsFakeItem(isWeapon, values) {
			continue
		}
		rawVal := getUint(sub, 2, 0)
		if byte(-rawVal) == 4 {
			setEntry(sub, 2, 0, uint64(-int64(uint64(opLevel)<<8|4)))
			entry.Value, _ = bl2save.WriteProtobuf(sub)
			return true, s.WriteSave(filename, tree, false)
		}
	}
	if opLevel > 0 {
		fakeVals := []int64{255}
		for i := 1; i < len(bl2save.ItemSizes[0]); i++ {
			fakeVals = append(fakeVals, 0)
		}
		fakeRaw := bl2save.WrapItem(0, fakeVals, randomKey())
		fakeEntry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
			1: {{WireType: 2, Value: fakeRaw}},
			2: {{WireType: 0, Value: uint64(-int64(uint64(opLevel)<<8 | 4))}},
		})
		tree[53] = append(tree[53], bl2save.PBEntry{WireType: 2, Value: fakeEntry})
		return true, s.WriteSave(filename, tree, false)
	}
	return false, nil
}

// UnlockAchievements writes save state satisfying all save-trackable
// achievement conditions (MAX CHARACTER).
func (s *Store) UnlockAchievements(filename string) (map[string]any, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	changes := map[string]any{}

	maxLevel := len(requiredXP)
	setEntry(tree, 2, 0, uint64(maxLevel))
	setEntry(tree, 3, 0, uint64(requiredXP[maxLevel-1]))
	changes["level"] = maxLevel

	setEntry(tree, 7, 0, uint64(2))
	changes["playthroughs_unlocked"] = 2

	missionsCompleted := 0
	if len(tree[18]) > 0 {
		ptData, ok := subTree(tree[18][0])
		if ok {
			existing := map[string]bool{}
			for i, mEntry := range ptData[3] {
				md, ok := subTree(mEntry)
				if !ok {
					continue
				}
				name := latin1(getBytes(md, 1))
				existing[name] = true
				if getUint(md, 2, 0) != 4 {
					setEntry(md, 2, 0, uint64(4))
					ptData[3][i].Value, _ = bl2save.WriteProtobuf(md)
					missionsCompleted++
				}
			}
			for _, path := range StoryMissions {
				if existing[path] {
					continue
				}
				level := missionLevels[path]
				mData, _ := bl2save.WriteProtobuf(bl2save.PBTree{
					1:  {{WireType: 2, Value: []byte(path)}},
					2:  {{WireType: 0, Value: uint64(4)}},
					3:  {{WireType: 0, Value: uint64(0)}},
					4:  {{WireType: 0, Value: uint64(0)}},
					11: {{WireType: 0, Value: uint64(level)}},
				})
				ptData[3] = append(ptData[3], bl2save.PBEntry{WireType: 2, Value: mData})
				missionsCompleted++
			}
			tree[18][0].Value, _ = bl2save.WriteProtobuf(ptData)
		}
	}
	changes["missions_completed"] = missionsCompleted

	challengesCompleted := 0
	for i, entry := range tree[38] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		if getUint(sub, 3, 0) < 1 {
			setEntry(sub, 2, 0, uint64(99999))
			setEntry(sub, 3, 0, uint64(1))
			tree[38][i].Value, _ = bl2save.WriteProtobuf(sub)
			challengesCompleted++
		}
	}
	changes["challenges_completed"] = challengesCompleted

	existingStations := map[string]bool{}
	for _, entry := range tree[16] {
		if v, ok := entry.Value.([]byte); ok {
			existingStations[latin1(v)] = true
		}
	}
	stationsAdded := 0
	for _, station := range AllStations {
		if !existingStations[station] {
			tree[16] = append(tree[16], bl2save.PBEntry{WireType: 2, Value: []byte(station)})
			existingStations[station] = true
			stationsAdded++
		}
	}
	changes["stations_unlocked"] = stationsAdded

	if len(tree[23]) > 0 {
		if unlocks, ok := tree[23][0].Value.([]byte); ok {
			hasUnlock := false
			for _, b := range unlocks {
				if b == 1 {
					hasUnlock = true
					break
				}
			}
			if !hasUnlock {
				tree[23][0].Value = append(append([]byte(nil), unlocks...), 1)
			}
		}
	} else {
		setEntry(tree, 23, 2, []byte{1})
	}

	return changes, s.WriteSave(filename, tree, false)
}

// UnlockPlaythrough unlocks TVHM ("tvhm") or UVHM ("uvhm").
func (s *Store) UnlockPlaythrough(filename, target string) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	current := getUint(tree, 7, 0)
	switch target {
	case "uvhm":
		if current < 2 {
			setEntry(tree, 7, 0, uint64(2))
		}
	case "tvhm":
		if current < 1 {
			setEntry(tree, 7, 0, uint64(1))
		}
	default:
		return false, nil
	}
	return true, s.WriteSave(filename, tree, false)
}

// SetActivePlaythrough sets the active playthrough index (0-2).
func (s *Store) SetActivePlaythrough(filename string, playthrough int) (bool, error) {
	if playthrough < 0 || playthrough > 2 {
		return false, nil
	}
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	setEntry(tree, 49, 0, uint64(playthrough))
	return true, s.WriteSave(filename, tree, false)
}

// UpdateCharacter applies partial character changes and returns fresh info.
func (s *Store) UpdateCharacter(filename string, changes map[string]any) (map[string]any, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}

	if v, ok := changes["level"]; ok {
		newLevel := int(toInt64(v))
		if newLevel >= 1 && newLevel <= len(requiredXP) {
			setEntry(tree, 2, 0, uint64(newLevel))
			setEntry(tree, 3, 0, uint64(requiredXP[newLevel-1]))
		}
	}

	if v, ok := changes["skill_points"]; ok {
		sp := toInt64(v)
		if sp < 0 {
			sp = 0
		}
		setEntry(tree, 4, 0, uint64(sp))
	}

	if anyKey(changes, "money", "eridium", "seraph", "torgue", "golden_keys") && len(tree[6]) > 0 {
		values := currencyValues(tree)
		for len(values) < 5 {
			values = append(values, 0)
		}
		setCurrency := func(key string, idx int) {
			if v, ok := changes[key]; ok {
				n := toInt64(v)
				if n < 0 {
					n = 0
				}
				values[idx] = uint64(n)
			}
		}
		setCurrency("money", 0)
		setCurrency("eridium", 1)
		setCurrency("seraph", 2)
		setCurrency("golden_keys", 3)
		setCurrency("torgue", 4)
		tree[6][0] = bl2save.PBEntry{WireType: 0, Value: values}
	}

	if v, ok := changes["name"]; ok && len(tree[19]) > 0 {
		if appearance, ok := subTree(tree[19][0]); ok {
			setEntry(appearance, 1, 2, []byte(toString(v)))
			tree[19][0].Value, _ = bl2save.WriteProtobuf(appearance)
		}
	}

	if v, ok := changes["inventory_size"]; ok && len(tree[13]) > 0 && len(tree[36]) > 0 {
		size := toInt64(v)
		if size < 12 {
			size = 12
		}
		if size > 39 {
			size = 39
		}
		sduSize := (size - 12 + 2) / 3
		actual := 12 + sduSize*3
		if slots, ok := subTree(tree[13][0]); ok {
			if len(slots[1]) > 0 {
				slots[1][0].Value = uint64(actual)
			}
			tree[13][0].Value, _ = bl2save.WriteProtobuf(slots)
		}
		s := readRepeatedVarints(getBytes(tree, 36))
		if len(s) >= 8 {
			newS := append(append([]uint64(nil), s[:7]...), uint64(sduSize))
			newS = append(newS, s[8:]...)
			tree[36][0].Value = newS
		}
	}

	if v, ok := changes["bank_size"]; ok && len(tree[36]) > 0 {
		size := toInt64(v)
		if size < 6 {
			size = 6
		}
		if size > 24 {
			size = 24
		}
		sduSize := (size - 6 + 1) / 2
		if sduSize > 255 {
			sduSize = 255
		}
		actual := 6 + sduSize*2
		setEntry(tree, 56, 0, uint64(actual))
		s := readRepeatedVarints(getBytes(tree, 36))
		for len(s) < 9 {
			s = append(s, 0)
		}
		newS := append(append([]uint64(nil), s[:8]...), uint64(sduSize))
		newS = append(newS, s[9:]...)
		tree[36][0].Value = newS
	}

	if v, ok := changes["weapon_slots"]; ok && len(tree[13]) > 0 {
		n := toInt64(v)
		if n < 2 {
			n = 2
		}
		if n > 4 {
			n = 4
		}
		if slots, ok := subTree(tree[13][0]); ok {
			if len(slots[2]) > 0 {
				slots[2][0].Value = uint64(n)
			}
			if len(slots[3]) > 0 {
				if getUint(slots, 3, 0) > uint64(n-2) {
					slots[3][0].Value = uint64(n - 2)
				}
			}
			tree[13][0].Value, _ = bl2save.WriteProtobuf(slots)
		}
	}

	if v, ok := changes["head_asset"]; ok {
		if head := toString(v); head != "" {
			ensureEntries(tree, 35, 1)
			tree[35][0] = bl2save.PBEntry{WireType: 2, Value: []byte(head)}
		}
	}
	if v, ok := changes["skin_asset"]; ok {
		if skin := toString(v); skin != "" {
			ensureEntries(tree, 35, 5)
			tree[35][4] = bl2save.PBEntry{WireType: 2, Value: []byte(skin)}
		}
	}

	if rawColors, ok := changes["appearance_colors"].([]any); ok && len(rawColors) >= 3 && len(tree[19]) > 0 {
		if appearance, ok := subTree(tree[19][0]); ok {
			for i, colorField := range []int{2, 3, 4} {
				c, _ := rawColors[i].(map[string]any)
				color := func(key string, def int64) uint64 {
					v, present := c[key]
					if !present {
						return uint64(def)
					}
					n := toInt64(v)
					if n < 0 {
						n = 0
					}
					if n > 255 {
						n = 255
					}
					return uint64(n)
				}
				colorBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
					1: {{WireType: 0, Value: color("a", 255)}},
					2: {{WireType: 0, Value: color("r", 127)}},
					3: {{WireType: 0, Value: color("g", 127)}},
					4: {{WireType: 0, Value: color("b", 127)}},
				})
				appearance[colorField] = []bl2save.PBEntry{{WireType: 2, Value: colorBytes}}
			}
			tree[19][0].Value, _ = bl2save.WriteProtobuf(appearance)
		}
	}

	if v, ok := changes["op_level"]; ok {
		op := toInt64(v)
		if op < 0 {
			op = 0
		}
		if op > 10 {
			op = 10
		}
		foundOp := false
		for _, entry := range tree[53] {
			sub, ok := subTree(entry)
			if !ok || len(sub[1]) == 0 || len(sub[2]) == 0 {
				continue
			}
			rawItem, ok := sub[1][0].Value.([]byte)
			if !ok {
				continue
			}
			isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
			if err != nil || !bl2save.IsFakeItem(isWeapon, values) {
				continue
			}
			rawVal := getUint(sub, 2, 0)
			if byte(-rawVal) == 4 {
				setEntry(sub, 2, 0, uint64(-int64(uint64(op)<<8|4)))
				entry.Value, _ = bl2save.WriteProtobuf(sub)
				foundOp = true
				break
			}
		}
		if !foundOp && op > 0 {
			fakeVals := []int64{255}
			for i := 1; i < len(bl2save.ItemSizes[0]); i++ {
				fakeVals = append(fakeVals, 0)
			}
			fakeRaw := bl2save.WrapItem(0, fakeVals, randomKey())
			fakeEntry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
				1: {{WireType: 2, Value: fakeRaw}},
				2: {{WireType: 0, Value: uint64(-int64(uint64(op)<<8 | 4))}},
			})
			tree[53] = append(tree[53], bl2save.PBEntry{WireType: 2, Value: fakeEntry})
		}
	}

	if rawAmmo, ok := changes["ammo"].(map[string]any); ok && len(tree[11]) > 0 {
		updates := map[string]int64{}
		for k, v := range rawAmmo {
			updates[k] = toInt64(v)
		}
		s.applyAmmoUpdates(tree, updates)
	}

	if err := s.WriteSave(filename, tree, false); err != nil {
		return nil, err
	}
	return s.ExtractCharacterInfo(tree), nil
}

// SaveAchievementState is not needed in save-state mode: achievement state is
// written via UnlockAchievements.

func ensureEntries(tree bl2save.PBTree, field, count int) {
	for len(tree[field]) < count {
		tree[field] = append(tree[field], bl2save.PBEntry{WireType: 2, Value: []byte("")})
	}
}

func anyKey(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if _, ok := m[k]; ok {
			return true
		}
	}
	return false
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case uint64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func clamp255(v int64) uint64 {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint64(v)
}

func float32FromBits(bits uint64) float32 {
	return math.Float32frombits(uint32(bits))
}

func lastDotSegment(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orDefaultInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func randomKey() int64 {
	return int64(rand.Uint32()) - 0x80000000
}
