package assets

import (
	"encoding/json"
	"math"
	"sort"
	"strings"
)

// GetWeaponCategories returns weapon types that have balances.
func (db *DB) GetWeaponCategories() []map[string]any {
	db.ensureLoaded()
	categories := []map[string]any{}
	for wtypePath, wtypeData := range db.weaponTypes {
		if _, has := db.balanceCategories[wtypePath]; !has {
			continue
		}
		name := jsonStr(wtypeData, "name")
		if name == "" {
			name = lastDotSegment(wtypePath)
		}
		typeStr := jsonStr(wtypeData, "type")
		if typeStr == "" {
			typeStr = "Unknown"
		}
		displayType := typeStr
		if d, ok := weaponTypeDisplay[typeStr]; ok {
			displayType = d
		}
		categories = append(categories, map[string]any{
			"path":          wtypePath,
			"name":          name,
			"type":          typeStr,
			"display_type":  displayType,
			"balance_count": len(db.balanceCategories[wtypePath]),
		})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i]["name"].(string) < categories[j]["name"].(string)
	})
	return categories
}

// GetBalancesForType lists balances under one weapon type, sorted by rarity.
func (db *DB) GetBalancesForType(weaponTypePath string) []map[string]any {
	db.ensureLoaded()
	paths := db.balanceCategories[weaponTypePath]
	result := []map[string]any{}
	for _, bp := range paths {
		bd := db.weaponBalance[bp]
		rarity := db.DetectRarity(bp)
		name := balanceDisplayName(bp, []string{"Pistol_", "AR_", "SG_", "SMG_", "SR_", "RL_", "Launcher_"})
		var mfrs []string
		if bd != nil {
			mfrs = jsonStrList(bd, "manufacturers")
		}
		result = append(result, map[string]any{
			"path": bp, "name": name, "rarity": rarity, "manufacturers": mfrs,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		ri := result[i]["rarity"].(rarityInfo)
		rj := result[j]["rarity"].(rarityInfo)
		if ri.Rank != rj.Rank {
			return ri.Rank < rj.Rank
		}
		return result[i]["name"].(string) < result[j]["name"].(string)
	})
	return result
}

func balanceDisplayName(path string, strips []string) string {
	name := lastDotSegment(path)
	for _, strip := range strips {
		if strings.HasPrefix(name, strip) {
			name = name[len(strip):]
			break
		}
	}
	return strings.ReplaceAll(name, "_", " ")
}

// GetPartsForBalance resolves the part list per slot for a weapon balance.
func (db *DB) GetPartsForBalance(balancePath string) map[string][]map[string]string {
	db.ensureLoaded()
	partSlots := map[string][]string{}
	var slotOrder []string
	seen := map[string]bool{}
	var chain []string
	path := balancePath
	for path != "" && !seen[path] {
		seen[path] = true
		chain = append(chain, path)
		bd := db.weaponBalance[path]
		if bd == nil {
			break
		}
		path = jsonStr(bd, "base")
	}
	for i := len(chain) - 1; i >= 0; i-- {
		bd := db.weaponBalance[chain[i]]
		if bd == nil {
			continue
		}
		partsRef := jsonStr(bd, "parts")
		if partsRef == "" {
			continue
		}
		bpl := db.weaponBalanceParts[partsRef]
		mode := "Additive"
		if bpl != nil {
			if m := jsonStr(bpl, "mode"); m != "" {
				mode = m
			}
		}
		for _, slot := range weaponBalanceSlots {
			if bpl == nil {
				continue
			}
			slotVal, ok := bpl[slot]
			if !ok || slotVal == nil {
				continue
			}
			var parts []string
			switch v := slotVal.(type) {
			case string:
				parts = db.weaponPartLists[v]
			case []any:
				parts = stringsFromAny(v)
			case map[string]any:
				switch pv := v["parts"].(type) {
				case string:
					parts = db.weaponPartLists[pv]
				case []any:
					parts = stringsFromAny(pv)
				}
			}
			if len(parts) == 0 {
				continue
			}
			if mode == "Selective" {
				if _, exists := partSlots[slot]; !exists {
					slotOrder = append(slotOrder, slot)
				}
				partSlots[slot] = parts
			} else if _, exists := partSlots[slot]; !exists {
				slotOrder = append(slotOrder, slot)
				partSlots[slot] = parts
			} else {
				existing := map[string]bool{}
				for _, p := range partSlots[slot] {
					existing[p] = true
				}
				for _, p := range parts {
					if !existing[p] {
						partSlots[slot] = append(partSlots[slot], p)
						existing[p] = true
					}
				}
			}
		}
	}
	result := map[string][]map[string]string{}
	for _, slot := range slotOrder {
		parts := partSlots[slot]
		out := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, map[string]string{"path": p, "name": db.CleanPartName(p, slot)})
		}
		result[slot] = out
	}
	return result
}

func stringsFromAny(raw []any) []string {
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// GetWeaponTypeForBalance resolves weapon_type through the balance chain.
func (db *DB) GetWeaponTypeForBalance(balancePath string) string {
	return db.resolveBalanceWeaponType(balancePath)
}

func (db *DB) resolveBalanceWeaponType(balancePath string) string {
	db.ensureLoaded()
	visited := map[string]bool{}
	path := balancePath
	for path != "" && !visited[path] {
		visited[path] = true
		bd := db.weaponBalance[path]
		if bd == nil {
			return ""
		}
		if wt := jsonStr(bd, "weapon_type"); wt != "" {
			return wt
		}
		path = jsonStr(bd, "base")
	}
	return ""
}

// GetManufacturerForBalance resolves the first manufacturer through the chain.
func (db *DB) GetManufacturerForBalance(balancePath string) string {
	db.ensureLoaded()
	visited := map[string]bool{}
	path := balancePath
	for path != "" && !visited[path] {
		visited[path] = true
		bd := db.weaponBalance[path]
		if bd == nil {
			return ""
		}
		mfrs := jsonStrList(bd, "manufacturers")
		if len(mfrs) > 0 {
			return mfrs[0]
		}
		path = jsonStr(bd, "base")
	}
	return ""
}

// GetAllPartsForSlot lists all weapon parts matching a slot keyword.
func (db *DB) GetAllPartsForSlot(slot string) []map[string]string {
	db.ensureLoaded()
	slotTypeMap := map[string]string{
		"body": "Body", "grip": "Grip", "barrel": "Barrel",
		"sight": "Sight", "stock": "Stock", "elemental": "Elemental",
		"accessory1": "Accessory", "accessory2": "Accessory",
		"material": "Material", "prefix": "Prefix", "title": "Title",
	}
	targetType := slot
	if t, ok := slotTypeMap[slot]; ok {
		targetType = t
	}
	var results []map[string]string
	seenPaths := map[string]bool{}
	seenNames := map[string]bool{}

	for path, data := range db.weaponParts {
		ptype := jsonStr(data, "type")
		short := strings.ToLower(lastDotSegment(path))
		match := false
		switch targetType {
		case "Elemental":
			if ptype == "Elemental" && !strings.Contains(short, "accessory") {
				match = strings.Contains(short, "element") || strings.Contains(short, "fire") ||
					strings.Contains(short, "shock") || strings.Contains(short, "corrosive") ||
					strings.Contains(short, "slag") || strings.Contains(short, "explosive") ||
					strings.Contains(short, "incendiary") || strings.Contains(short, "elemental_none") ||
					strings.HasSuffix(short, "_none")
			}
		case "Accessory":
			if ptype == "Accessory1" || ptype == "Accessory2" {
				match = true
			} else if ptype == "Elemental" && strings.Contains(short, "accessory") {
				match = true
			}
		default:
			match = ptype == targetType
		}
		if !match || seenPaths[path] {
			continue
		}
		name := db.CleanPartName(path, slot)
		if seenNames[name] {
			continue
		}
		seenPaths[path] = true
		seenNames[name] = true
		results = append(results, map[string]string{"path": path, "name": name})
	}

	if len(results) < 5 {
		kw := targetType
		if kw == "Elemental" {
			kw = "Element"
		}
		for _, key := range db.forwardKeys {
			if key.Group != "WeaponParts" {
				continue
			}
			path := db.forward[key]
			if seenPaths[path] {
				continue
			}
			short := lastDotSegment(path)
			if !strings.Contains("_"+short+"_", "_"+kw+"_") && !strings.HasPrefix(short, kw+"_") {
				continue
			}
			name := db.CleanPartName(path, slot)
			if seenNames[name] {
				continue
			}
			seenPaths[path] = true
			seenNames[name] = true
			results = append(results, map[string]string{"path": path, "name": name})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i]["name"] < results[j]["name"] })
	return results
}

// GetAllItemPartsForSlot lists all item parts for a Greek-letter slot.
func (db *DB) GetAllItemPartsForSlot(slot string) []map[string]string {
	db.ensureLoaded()
	slotTypeMap := map[string]string{
		"alpha": "Alpha", "beta": "Beta", "gamma": "Gamma",
		"delta": "Delta", "epsilon": "Epsilon", "zeta": "Zeta",
		"eta": "Eta", "theta": "Theta", "material": "Material",
	}
	targetType := slot
	if t, ok := slotTypeMap[slot]; ok {
		targetType = t
	}
	var results []map[string]string
	seenPaths := map[string]bool{}
	seenNames := map[string]bool{}

	for path, data := range db.itemParts {
		if jsonStr(data, "type") != targetType || seenPaths[path] {
			continue
		}
		name := db.CleanPartName(path, slot)
		if seenNames[name] {
			continue
		}
		seenPaths[path] = true
		seenNames[name] = true
		results = append(results, map[string]string{"path": path, "name": name})
	}

	if len(results) < 3 {
		for _, key := range db.forwardKeys {
			if key.Group != "ItemParts" {
				continue
			}
			path := db.forward[key]
			if seenPaths[path] {
				continue
			}
			short := lastDotSegment(path)
			if !strings.Contains(strings.ToLower(short), strings.ToLower(targetType)) {
				continue
			}
			name := db.CleanPartName(path, slot)
			if seenNames[name] {
				continue
			}
			seenPaths[path] = true
			seenNames[name] = true
			results = append(results, map[string]string{"path": path, "name": name})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i]["name"] < results[j]["name"] })
	return results
}

// GetItemCategories returns item categories with balance counts.
func (db *DB) GetItemCategories() []map[string]any {
	db.ensureLoaded()
	categories := map[string]*int{}
	keys := []string{}
	for balPath, balData := range db.itemBalance {
		itemRef := jsonStr(balData, "item")
		cat := ""
		for key := range itemTypeCategories {
			if strings.Contains(balPath, key) || strings.Contains(itemRef, key) {
				cat = key
				break
			}
		}
		if cat == "" || cat == "MissionItem" {
			continue
		}
		if _, ok := categories[cat]; !ok {
			n := 0
			categories[cat] = &n
			keys = append(keys, cat)
		}
		*categories[cat]++
	}
	out := []map[string]any{}
	for _, cat := range keys {
		out = append(out, map[string]any{
			"key": cat, "name": itemTypeCategories[cat], "count": *categories[cat],
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["name"].(string) < out[j]["name"].(string) })
	return out
}

// GetItemBalancesForCategory lists balances within one item category.
func (db *DB) GetItemBalancesForCategory(category string) []map[string]any {
	db.ensureLoaded()
	var results []map[string]any
	for balPath, balData := range db.itemBalance {
		itemRef := jsonStr(balData, "item")
		if !strings.Contains(balPath, category) && !strings.Contains(itemRef, category) {
			continue
		}
		name := balanceDisplayName(balPath, []string{"Shield_", "GrenadeMod_", "ClassMod_", "Artifact_", "ItemGrade_Gear_", "ItemGrade_"})
		rarity := db.DetectRarity(balPath)
		mfrs := jsonStrList(balData, "manufacturers")
		results = append(results, map[string]any{
			"path": balPath, "name": name, "rarity": rarity, "manufacturers": mfrs,
		})
	}
	sort.Slice(results, func(i, j int) bool {
		ri := results[i]["rarity"].(rarityInfo)
		rj := results[j]["rarity"].(rarityInfo)
		if ri.Rank != rj.Rank {
			return ri.Rank < rj.Rank
		}
		return results[i]["name"].(string) < results[j]["name"].(string)
	})
	return results
}

// GetItemPartsForBalance resolves available parts for an item balance.
func (db *DB) GetItemPartsForBalance(balancePath string) map[string][]map[string]string {
	db.ensureLoaded()
	result := map[string][]map[string]string{}
	var displayOrder []string

	balData := db.itemBalance[balancePath]
	if balData == nil {
		return result
	}
	itemRef := jsonStr(balData, "item")
	itemData := db.itemTypes[itemRef]

	addParts := func(displayName string, parts []string) {
		if len(parts) == 0 {
			return
		}
		if _, ok := result[displayName]; !ok {
			displayOrder = append(displayOrder, displayName)
		}
		out := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			out = append(out, map[string]string{"path": p, "name": db.CleanPartName(p, displayName)})
		}
		result[displayName] = out
	}

	// deterministic slot order
	slotKeys := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "material"}
	for _, slotKey := range slotKeys {
		displayName := itemSlotNames[slotKey]
		partsRef := jsonStr(itemData, slotKey+"_parts")
		if partsRef == "" {
			continue
		}
		addParts(displayName, db.resolveItemPartList(partsRef, slotKey))
	}

	if balPartsRef := jsonStr(balData, "parts"); balPartsRef != "" {
		for _, slotKey := range slotKeys {
			displayName := itemSlotNames[slotKey]
			if _, exists := result[displayName]; exists {
				continue
			}
			addParts(displayName, db.resolveItemPartList(balPartsRef, slotKey))
		}
	}

	ordered := map[string][]map[string]string{}
	for _, name := range displayOrder {
		ordered[name] = result[name]
	}
	return ordered
}

func (db *DB) resolveItemPartList(partsRef, slotKey string) []string {
	candidates := db.itemPartsByType[strings.ToLower(slotKey)]
	if len(candidates) == 0 {
		candidates = db.itemPartsByType[strings.ToLower(capitalize(slotKey))]
	}
	refPrefix := partsRef
	if idx := strings.LastIndex(partsRef, "."); idx >= 0 {
		refPrefix = partsRef[:idx]
	}
	refSuffix := strings.ReplaceAll(lastDotSegment(partsRef), "PartsList_", "")
	var parts []string
	for _, c := range candidates {
		if strings.Contains(c.path, refPrefix) || strings.Contains(c.path, refSuffix) {
			parts = append(parts, c.path)
		}
	}
	if len(parts) == 0 {
		refShort := strings.ToLower(lastDotSegment(partsRef))
		var keywords []string
		for _, kw := range strings.Split(strings.ReplaceAll(refShort, "partslist_", ""), "_") {
			if len(kw) > 3 {
				keywords = append(keywords, kw)
			}
		}
		for _, c := range candidates {
			lower := strings.ToLower(c.path)
			for _, kw := range keywords {
				if strings.Contains(lower, kw) {
					parts = append(parts, c.path)
					break
				}
			}
		}
	}
	return parts
}

// GetItemTypeForBalance returns the item type path of a balance.
func (db *DB) GetItemTypeForBalance(balancePath string) string {
	db.ensureLoaded()
	if balData := db.itemBalance[balancePath]; balData != nil {
		return jsonStr(balData, "item")
	}
	return ""
}

// GetManufacturerForItemBalance returns the first manufacturer of an item balance.
func (db *DB) GetManufacturerForItemBalance(balancePath string) string {
	db.ensureLoaded()
	if balData := db.itemBalance[balancePath]; balData != nil {
		if mfrs := jsonStrList(balData, "manufacturers"); len(mfrs) > 0 {
			return mfrs[0]
		}
	}
	return ""
}

// GetAllBalances lists every weapon balance with category and rarity.
func (db *DB) GetAllBalances() []map[string]any {
	db.ensureLoaded()
	var results []map[string]any
	for bp := range db.weaponBalance {
		rarity := db.DetectRarity(bp)
		name := balanceDisplayName(bp, []string{"Pistol_", "AR_", "SG_", "SMG_", "SR_", "RL_", "Launcher_"})
		wtype := db.resolveBalanceWeaponType(bp)
		cat := "Unknown"
		if wtype != "" {
			cat = db.DetectWeaponCategory(wtype)
		}
		results = append(results, map[string]any{"path": bp, "name": name, "rarity": rarity, "category": cat})
	}
	sort.Slice(results, func(i, j int) bool {
		ci, _ := results[i]["category"].(string)
		cj, _ := results[j]["category"].(string)
		if ci != cj {
			return ci < cj
		}
		ri := results[i]["rarity"].(rarityInfo)
		rj := results[j]["rarity"].(rarityInfo)
		if ri.Rank != rj.Rank {
			return ri.Rank < rj.Rank
		}
		return results[i]["name"].(string) < results[j]["name"].(string)
	})
	return results
}

// GetAllManufacturers lists distinct manufacturer paths.
func (db *DB) GetAllManufacturers() []map[string]string {
	db.ensureLoaded()
	var results []map[string]string
	seen := map[string]bool{}
	for _, key := range db.forwardKeys {
		if key.Group != "Manufacturers" {
			continue
		}
		path := db.forward[key]
		if seen[path] {
			continue
		}
		seen[path] = true
		results = append(results, map[string]string{"path": path, "name": db.GetManufacturerName(path)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i]["name"] < results[j]["name"] })
	return results
}

// helpers -------------------------------------------------------------------

func jsonStr(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func jsonStrList(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func libAsset(v any) (int, int) {
	m, ok := v.(map[string]any)
	if !ok {
		return 0, 0
	}
	return int(toInt64(m["lib"])), int(toInt64(m["asset"]))
}

func resolvedParts(resolved map[string]any) []map[string]any {
	parts, _ := resolved["resolved_parts"].([]map[string]any)
	return parts
}

func lastDotSegment(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case int:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func round1(v float64) float64 {
	return math.RoundToEven(v*10) / 10
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
