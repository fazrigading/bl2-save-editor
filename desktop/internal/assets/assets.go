// Package assets ports asset_db.py: resolving packed BL2 item indices to
// human-readable names from Gibbed JSON data.
package assets

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type bitConfig struct {
	SublibraryBits int
	AssetBits      int
}

var defaultConfigs = map[string]bitConfig{
	"WeaponTypes":   {7, 6},
	"WeaponParts":   {6, 11},
	"ItemTypes":     {9, 8},
	"ItemParts":     {6, 10},
	"Manufacturers": {4, 7},
	"BalanceDefs":   {10, 10},
}

var fieldGroups = map[string]map[string]string{
	"type":         {"weapon": "WeaponTypes", "item": "ItemTypes"},
	"balance":      {"weapon": "BalanceDefs", "item": "BalanceDefs"},
	"manufacturer": {"weapon": "Manufacturers", "item": "Manufacturers"},
}

var partGroup = map[string]string{"weapon": "WeaponParts", "item": "ItemParts"}

var partSlotNames = []string{
	"Body", "Grip", "Barrel", "Sight", "Stock",
	"Element", "Accessory1", "Accessory2",
	"Material", "Prefix", "Title",
}

var weaponBalanceSlots = []string{
	"body", "grip", "barrel", "sight", "stock",
	"elemental", "accessory1", "accessory2",
	"material", "prefix", "title",
}

var itemSlotNames = map[string]string{
	"alpha": "Body", "beta": "Battery/Grip", "gamma": "Capacitor/Barrel",
	"delta": "Accessory", "epsilon": "Accessory 2", "zeta": "Slot 6",
	"eta": "Slot 7", "theta": "Slot 8", "material": "Material",
}

type rarityInfo struct {
	Name  string `json:"name"`
	Color string `json:"color"`
	Rank  int    `json:"rank"`
}

var rarityMap = map[string]rarityInfo{
	"1_Common":      {"Common", "#9d9d9d", 0},
	"2_Uncommon":    {"Uncommon", "#3bbd40", 1},
	"3_Rare":        {"Rare", "#4b8be8", 2},
	"4_VeryRare":    {"Very Rare", "#9b59b6", 3},
	"5_Legendary":   {"Legendary", "#e8a33a", 4},
	"_Legendary":    {"Legendary", "#e8a33a", 4},
	"6_Pearlescent": {"Pearlescent", "#00e5ff", 5},
	"_Seraph":       {"Seraph", "#ff4081", 6},
	"_Unique":       {"Unique", "#9b59b6", 3},
	"_Effervescent": {"Effervescent", "#ff6ed4", 7},
}

var manufacturerNames = map[string]string{
	"Bandit": "Bandit", "Dahl": "Dahl", "Hyperion": "Hyperion",
	"Jakobs": "Jakobs", "Maliwan": "Maliwan", "Tediore": "Tediore",
	"Torgue": "Torgue", "Vladof": "Vladof",
}

var weaponTypeDisplay = map[string]string{
	"Pistol": "Pistol", "AssaultRifle": "Assault Rifle",
	"SMG": "SMG", "Shotgun": "Shotgun",
	"SniperRifle": "Sniper Rifle", "RocketLauncher": "Rocket Launcher",
	"Launcher": "Rocket Launcher",
}

var itemTypeCategories = map[string]string{
	"Shield": "Shield", "GrenadeMod": "Grenade Mod",
	"ClassMod": "Class Mod", "Artifact": "Relic",
	"MissionItem": "Mission Item",
}

var weaponBaseStats = map[string]map[string]float64{
	"Pistol":          {"damage": 18, "fire_rate": 8.5, "reload_speed": 2.5, "mag_size": 14, "accuracy": 90.0, "recoil": 15.0},
	"Assault Rifle":   {"damage": 22, "fire_rate": 8.0, "reload_speed": 3.8, "mag_size": 28, "accuracy": 85.0, "recoil": 18.0},
	"SMG":             {"damage": 16, "fire_rate": 10.0, "reload_speed": 2.8, "mag_size": 30, "accuracy": 82.0, "recoil": 20.0},
	"Shotgun":         {"damage": 80, "fire_rate": 2.0, "reload_speed": 4.0, "mag_size": 8, "accuracy": 55.0, "recoil": 40.0},
	"Sniper Rifle":    {"damage": 120, "fire_rate": 1.5, "reload_speed": 4.5, "mag_size": 6, "accuracy": 96.0, "recoil": 8.0},
	"Rocket Launcher": {"damage": 300, "fire_rate": 1.0, "reload_speed": 5.0, "mag_size": 4, "accuracy": 80.0, "recoil": 50.0},
}

var manufacturerStatMods = map[string]map[string]float64{
	"Bandit":   {"mag_size": 1.5, "accuracy": 0.85, "reload_speed": 1.3, "damage": 0.95, "recoil": 1.1},
	"Dahl":     {"accuracy": 1.15, "recoil": 0.7, "damage": 0.95, "fire_rate": 1.05},
	"Hyperion": {"accuracy": 1.1, "damage": 1.05, "recoil": 0.85, "fire_rate": 0.9},
	"Jakobs":   {"damage": 1.2, "fire_rate": 0.6, "mag_size": 0.7, "accuracy": 1.05, "reload_speed": 1.1},
	"Maliwan":  {"damage": 0.85, "fire_rate": 0.95, "accuracy": 1.05, "reload_speed": 1.05},
	"Tediore":  {"reload_speed": 0.6, "damage": 0.9, "mag_size": 0.9},
	"Torgue":   {"damage": 1.3, "fire_rate": 0.7, "accuracy": 0.8, "recoil": 1.3, "reload_speed": 1.1},
	"Vladof":   {"fire_rate": 1.3, "damage": 0.85, "recoil": 0.85, "mag_size": 1.2},
}

var partStatMods = map[string]map[string]map[string]float64{
	"Barrel": {
		"Bandit": {"mag_size": 1.2, "accuracy": 0.9}, "Dahl": {"recoil": 0.8, "accuracy": 1.05},
		"Hyperion": {"accuracy": 1.1, "fire_rate": 0.85}, "Jakobs": {"damage": 1.15, "fire_rate": 0.9},
		"Maliwan": {"damage": 0.95, "accuracy": 1.05}, "Tediore": {"reload_speed": 0.9},
		"Torgue": {"damage": 1.2, "fire_rate": 0.8, "accuracy": 0.85}, "Vladof": {"fire_rate": 1.15, "damage": 0.95},
	},
	"Grip": {
		"Bandit": {"mag_size": 1.3, "reload_speed": 1.15}, "Dahl": {"recoil": 0.85, "damage": 0.95},
		"Hyperion": {"accuracy": 1.1, "damage": 0.95}, "Jakobs": {"damage": 1.1, "reload_speed": 0.9, "fire_rate": 0.95},
		"Maliwan": {"damage": 0.95}, "Tediore": {"reload_speed": 0.8},
		"Torgue": {"damage": 1.1, "fire_rate": 0.9}, "Vladof": {"fire_rate": 1.1, "damage": 0.95},
	},
	"Stock": {
		"Dahl": {"recoil": 0.8, "accuracy": 0.95}, "Hyperion": {"accuracy": 1.1, "recoil": 0.9},
		"Jakobs": {"accuracy": 1.05, "recoil": 1.1}, "Torgue": {"recoil": 1.1, "damage": 1.02},
		"Vladof": {"recoil": 0.9, "fire_rate": 1.02},
	},
	"Body": {
		"Bandit": {"damage": 0.95, "mag_size": 1.15}, "Dahl": {"damage": 1.0, "accuracy": 1.05},
		"Hyperion": {"accuracy": 1.1, "damage": 1.0}, "Jakobs": {"damage": 1.1},
		"Torgue": {"damage": 1.1, "fire_rate": 0.95}, "Vladof": {"fire_rate": 1.05},
	},
	"Sight": {
		"Dahl": {"accuracy": 1.02}, "Hyperion": {"accuracy": 1.05},
		"Jakobs": {"accuracy": 1.03}, "Vladof": {"accuracy": 1.01},
	},
}

var partEffects = map[string]map[string]string{
	"Barrel": {
		"Bandit": "+Mag Size, -Accuracy", "Dahl": "+Burst Count, +Stability",
		"Hyperion": "+Accuracy", "Jakobs": "+Damage, -Fire Rate",
		"Maliwan": "+Elemental Chance", "Tediore": "+Reload Speed",
		"Torgue": "+Damage, -Fire Rate", "Vladof": "+Fire Rate",
	},
	"Grip": {
		"Bandit": "+Mag Size, -Reload", "Dahl": "+Stability, -Damage",
		"Hyperion": "+Accuracy, -Damage", "Jakobs": "+Damage, +Reload, -Fire Rate",
		"Maliwan": "+Elemental, -Damage", "Tediore": "+Reload Speed",
		"Torgue": "+Damage, -Fire Rate", "Vladof": "+Fire Rate, -Damage",
	},
}

type forwardKey struct {
	Set    int
	Group  string
	Sublib int
	Asset  int
}

type reverseKey struct {
	Path  string
	Group string
}

type reverseVal struct {
	Set    int
	Sublib int
	Asset  int
}

// DB is the loaded asset database.
type DB struct {
	gibbedDir   string
	configs     map[string]bitConfig
	forward     map[forwardKey]string
	forwardKeys []forwardKey
	reverse     map[reverseKey]reverseVal

	weaponTypes        map[string]map[string]any
	weaponParts        map[string]map[string]any
	weaponBalance      map[string]map[string]any
	weaponBalanceParts map[string]map[string]any
	weaponNameParts    map[string]map[string]any
	weaponPartLists    map[string][]string
	itemTypes          map[string]map[string]any
	itemParts          map[string]map[string]any
	itemBalance        map[string]map[string]any

	balanceCategories map[string][]string
	itemPartsByType   map[string][]itemPartEntry
	loaded            bool
}

type itemPartEntry struct {
	path string
	data map[string]any
}

// New creates a DB bound to a Gibbed data directory.
func New(gibbedDir string) *DB {
	return &DB{
		gibbedDir: gibbedDir,
		configs:   map[string]bitConfig{},
		forward:   map[forwardKey]string{},
		reverse:   map[reverseKey]reverseVal{},
	}
}

func (db *DB) loadJSON(filename string) map[string]map[string]any {
	out := map[string]map[string]any{}
	path := filepath.Join(db.gibbedDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return out
	}
	for k, v := range raw {
		var m map[string]any
		if err := json.Unmarshal(v, &m); err == nil {
			out[k] = m
		}
	}
	return out
}

func (db *DB) loadJSONList(filename string) map[string][]string {
	out := map[string][]string{}
	path := filepath.Join(db.gibbedDir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return out
	}
	for k, v := range raw {
		var list []string
		if err := json.Unmarshal(v, &list); err == nil {
			out[k] = list
		}
	}
	return out
}

// ensureLoaded lazily loads the Gibbed data.
func (db *DB) ensureLoaded() {
	if db.loaded {
		return
	}
	db.loaded = true
	for group, cfg := range defaultConfigs {
		db.configs[group] = cfg
	}

	var alm struct {
		Configs map[string]struct {
			SublibraryBits *int `json:"sublibrary_bits"`
			AssetBits      *int `json:"asset_bits"`
		} `json:"configs"`
		Sets []struct {
			Libraries map[string]struct {
				Sublibraries []struct {
					Package string   `json:"package"`
					Assets  []string `json:"assets"`
				} `json:"sublibraries"`
			} `json:"libraries"`
		} `json:"sets"`
	}
	if data, err := os.ReadFile(filepath.Join(db.gibbedDir, "Asset Library Manager.json")); err == nil {
		if err := json.Unmarshal(data, &alm); err == nil {
			for group, cfg := range alm.Configs {
				if base, ok := db.configs[group]; ok {
					if cfg.SublibraryBits != nil {
						base.SublibraryBits = *cfg.SublibraryBits
					}
					if cfg.AssetBits != nil {
						base.AssetBits = *cfg.AssetBits
					}
					db.configs[group] = base
				}
			}
			for setIdx, s := range alm.Sets {
				for groupName, libData := range s.Libraries {
					for sublibIdx, sublib := range libData.Sublibraries {
						if sublib.Package == "" || len(sublib.Assets) == 0 {
							continue
						}
						for assetIdx, assetName := range sublib.Assets {
							fullPath := sublib.Package + "." + assetName
							key := forwardKey{Set: setIdx, Group: groupName, Sublib: sublibIdx, Asset: assetIdx}
							db.forward[key] = fullPath
							db.forwardKeys = append(db.forwardKeys, key)
							db.reverse[reverseKey{Path: fullPath, Group: groupName}] =
								reverseVal{Set: setIdx, Sublib: sublibIdx, Asset: assetIdx}
						}
					}
				}
			}
		}
	}

	db.weaponTypes = db.loadJSON("Weapon Types.json")
	db.weaponParts = db.loadJSON("Weapon Parts.json")
	db.weaponBalance = db.loadJSON("Weapon Balance.json")
	db.weaponBalanceParts = db.loadJSON("Weapon Balance Part Lists.json")
	db.weaponNameParts = db.loadJSON("Weapon Name Parts.json")
	db.weaponPartLists = db.loadJSONList("Weapon Part Lists.json")
	db.itemTypes = db.loadJSON("Items.json")
	db.itemParts = db.loadJSON("Item Parts.json")
	db.itemBalance = db.loadJSON("Item Balance.json")

	db.balanceCategories = map[string][]string{}
	for balPath := range db.weaponBalance {
		wtype := db.resolveBalanceWeaponType(balPath)
		if wtype != "" {
			db.balanceCategories[wtype] = append(db.balanceCategories[wtype], balPath)
		}
	}
	for _, list := range db.balanceCategories {
		sort.Strings(list)
	}

	db.itemPartsByType = map[string][]itemPartEntry{}
	for path, data := range db.itemParts {
		ptype := strings.ToLower(jsonStr(data, "type"))
		if ptype != "" {
			db.itemPartsByType[ptype] = append(db.itemPartsByType[ptype], itemPartEntry{path: path, data: data})
		}
	}
}

// ResolvePath resolves a (set, group, lib, asset) reference to a full path.
func (db *DB) ResolvePath(setID int, group string, lib, asset int) string {
	db.ensureLoaded()
	cfg, ok := db.configs[group]
	if !ok {
		return ""
	}
	useSetMask := 1 << uint(cfg.SublibraryBits-1)
	useSetID := lib&useSetMask != 0
	actualSublib := lib & (useSetMask - 1)
	actualSet := 0
	if useSetID {
		actualSet = setID
	}
	maxVal := (1 << uint(cfg.SublibraryBits+cfg.AssetBits)) - 1
	packed := lib<<uint(cfg.AssetBits) | asset
	if packed == maxVal {
		return ""
	}
	return db.forward[forwardKey{Set: actualSet, Group: group, Sublib: actualSublib, Asset: asset}]
}

// PackReference packs a full path back into (lib, asset) values.
func (db *DB) PackReference(fullPath, group string, setID int) (int, int, bool) {
	db.ensureLoaded()
	rv, ok := db.reverse[reverseKey{Path: fullPath, Group: group}]
	if !ok {
		return 0, 0, false
	}
	cfg := db.configs[group]
	useSetMask := 1 << uint(cfg.SublibraryBits-1)
	lib := rv.Sublib
	if rv.Set != 0 {
		lib |= useSetMask
	}
	return lib, rv.Asset, true
}

// GetSetIDForPath returns the library set index for an asset path.
func (db *DB) GetSetIDForPath(fullPath, group string) int {
	db.ensureLoaded()
	if rv, ok := db.reverse[reverseKey{Path: fullPath, Group: group}]; ok {
		return rv.Set
	}
	return 0
}

// DetectRarity determines rarity from a balance path (longest keyword wins,
// ties broken by higher rank).
func (db *DB) DetectRarity(balancePath string) rarityInfo {
	common := rarityInfo{Name: "Common", Color: "#9d9d9d", Rank: 0}
	if balancePath == "" {
		return common
	}
	type kr struct {
		key  string
		info rarityInfo
	}
	entries := make([]kr, 0, len(rarityMap))
	for k, v := range rarityMap {
		entries = append(entries, kr{k, v})
	}
	sort.Slice(entries, func(i, j int) bool {
		if len(entries[i].key) != len(entries[j].key) {
			return len(entries[i].key) > len(entries[j].key)
		}
		return entries[i].info.Rank > entries[j].info.Rank
	})
	for _, e := range entries {
		if strings.Contains(balancePath, e.key) {
			return e.info
		}
	}
	return common
}

// DetectWeaponCategory maps a type path to a display category.
func (db *DB) DetectWeaponCategory(typePath string) string {
	if typePath == "" {
		return "Unknown"
	}
	lower := strings.ToLower(typePath)
	for key, display := range weaponTypeDisplay {
		if strings.Contains(lower, strings.ToLower(key)) {
			return display
		}
	}
	return "Weapon"
}

// DetectItemCategory maps type/balance paths to an item display category.
func (db *DB) DetectItemCategory(typePath, balancePath string) string {
	check := strings.ToLower(typePath + balancePath)
	type kv struct {
		key, display string
	}
	entries := make([]kv, 0, len(itemTypeCategories))
	for k, d := range itemTypeCategories {
		entries = append(entries, kv{k, d})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	for _, e := range entries {
		if strings.Contains(check, strings.ToLower(e.key)) {
			return e.display
		}
	}
	return "Item"
}

// DetectElement inspects resolved parts for an element.
func (db *DB) DetectElement(parts []map[string]any) map[string]any {
	for _, p := range parts {
		slot, _ := p["slot"].(string)
		path, _ := p["path"].(string)
		if slot != "Element" || path == "" {
			continue
		}
		lower := strings.ToLower(path)
		switch {
		case strings.Contains(lower, "incendiary"), strings.Contains(lower, "fire"):
			return map[string]any{"name": "Fire", "color": "#ff6600"}
		case strings.Contains(lower, "shock"):
			return map[string]any{"name": "Shock", "color": "#0099ff"}
		case strings.Contains(lower, "corrosive"):
			return map[string]any{"name": "Corrosive", "color": "#00dd00"}
		case strings.Contains(lower, "slag"):
			return map[string]any{"name": "Slag", "color": "#cc00ff"}
		case strings.Contains(lower, "explosive"):
			return map[string]any{"name": "Explosive", "color": "#ffdd00"}
		}
	}
	return nil
}

// GetManufacturerName shortens a manufacturer path.
func (db *DB) GetManufacturerName(mfrPath string) string {
	if mfrPath == "" {
		return "Unknown"
	}
	short := lastDotSegment(mfrPath)
	if name, ok := manufacturerNames[short]; ok {
		return name
	}
	return short
}

// GetPartEffect returns a display hint for a part.
func (db *DB) GetPartEffect(slot, partPath string) string {
	if partPath == "" {
		return ""
	}
	effects, ok := partEffects[slot]
	if !ok {
		return ""
	}
	lower := strings.ToLower(partPath)
	for mfr, effect := range effects {
		if strings.Contains(lower, strings.ToLower(mfr)) {
			return effect
		}
	}
	return ""
}

// ResolveItemParts resolves every part of an item to display metadata.
func (db *DB) ResolveItemParts(itemInfo map[string]any) map[string]any {
	db.ensureLoaded()
	isWeapon := int(toInt64(itemInfo["is_weapon"]))
	setID := int(toInt64(itemInfo["set"]))
	kind := "item"
	if isWeapon != 0 {
		kind = "weapon"
	}
	result := map[string]any{}
	for k, v := range itemInfo {
		result[k] = v
	}

	typeGroup := fieldGroups["type"][kind]
	typeLib, typeAsset := libAsset(itemInfo["type"])
	typePath := db.ResolvePath(setID, typeGroup, typeLib, typeAsset)
	result["type_path"] = typePath

	balGroup := fieldGroups["balance"][kind]
	balLib, balAsset := libAsset(itemInfo["balance"])
	balPath := db.ResolvePath(setID, balGroup, balLib, balAsset)
	result["balance_path"] = balPath

	mfrGroup := fieldGroups["manufacturer"][kind]
	mfrLib, mfrAsset := libAsset(itemInfo["manufacturer"])
	mfrPath := db.ResolvePath(setID, mfrGroup, mfrLib, mfrAsset)
	result["manufacturer_path"] = mfrPath
	result["manufacturer_name"] = db.GetManufacturerName(mfrPath)

	rarity := db.DetectRarity(balPath)
	result["rarity"] = rarity

	if isWeapon != 0 {
		result["category"] = db.DetectWeaponCategory(typePath)
	} else {
		result["category"] = db.DetectItemCategory(typePath, balPath)
	}

	partGrp := partGroup[kind]
	rawParts, _ := itemInfo["parts"].([]any)
	resolvedParts := []map[string]any{}
	for i, p := range rawParts {
		slotName := "Part" + itoa(i)
		if i < len(partSlotNames) {
			slotName = partSlotNames[i]
		}
		if p == nil {
			resolvedParts = append(resolvedParts, map[string]any{
				"slot": slotName, "path": nil, "name": "None", "lib": 0, "asset": 0, "effect": "",
			})
			continue
		}
		lib, asset := libAsset(p)
		path := db.ResolvePath(setID, partGrp, lib, asset)
		if path == "" {
			resolvedParts = append(resolvedParts, map[string]any{
				"slot": slotName, "path": nil, "name": "None", "lib": lib, "asset": asset, "effect": "",
			})
			continue
		}
		resolvedParts = append(resolvedParts, map[string]any{
			"slot": slotName, "path": path,
			"name": db.CleanPartName(path, slotName), "lib": lib, "asset": asset,
			"effect": db.GetPartEffect(slotName, path),
		})
	}
	result["resolved_parts"] = resolvedParts
	result["element"] = db.DetectElement(resolvedParts)
	result["display_name"] = db.BuildDisplayName(result)
	if isWeapon != 0 {
		result["estimated_stats"] = db.EstimateWeaponStats(result)
	}
	return result
}

// CleanPartName makes a part path human-readable.
func (db *DB) CleanPartName(path, slot string) string {
	if path == "" {
		return "None"
	}
	short := lastDotSegment(path)
	prefixes := []string{
		"Pistol_Barrel_", "Pistol_Grip_", "Pistol_Body_", "Pistol_Sight_",
		"AR_Barrel_", "AR_Grip_", "AR_Body_", "AR_Sight_", "AR_Stock_",
		"SG_Barrel_", "SG_Grip_", "SG_Body_", "SG_Sight_", "SG_Stock_",
		"SMG_Barrel_", "SMG_Grip_", "SMG_Body_", "SMG_Sight_", "SMG_Stock_",
		"SR_Barrel_", "SR_Grip_", "SR_Body_", "SR_Sight_", "SR_Stock_",
		"RL_Barrel_", "RL_Grip_", "RL_Body_", "RL_Sight_", "RL_Stock_",
		"Mat_", "Accessory_", "Elemental_",
		"Prefix_Barrel_", "Prefix_Grip_", "Prefix_",
		"Title_Legendary_", "Title_Unique_", "Title__", "Title_",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(short, p) {
			short = short[len(p):]
			break
		}
	}
	return strings.ReplaceAll(short, "_", " ")
}

// BuildDisplayName builds the human-readable item name.
func (db *DB) BuildDisplayName(resolved map[string]any) string {
	parts, _ := resolved["resolved_parts"].([]map[string]any)
	titlePath := ""
	prefixPath := ""
	for _, p := range parts {
		slot, _ := p["slot"].(string)
		path, _ := p["path"].(string)
		if slot == "Title" && path != "" {
			titlePath = path
		}
		if slot == "Prefix" && path != "" {
			prefixPath = path
		}
	}

	titleText := ""
	if titlePath != "" {
		if npi, ok := db.weaponNameParts[titlePath]; ok {
			titleText = jsonStr(npi, "name")
		}
		if titleText == "" {
			titleText = db.CleanPartName(titlePath, "Title")
		}
	}
	prefixText := ""
	if prefixPath != "" {
		if npi, ok := db.weaponNameParts[prefixPath]; ok {
			prefixText = jsonStr(npi, "name")
		}
	}

	if prefixText != "" && titleText != "" {
		return prefixText + " " + titleText
	}
	if titleText != "" {
		return titleText
	}
	if prefixText != "" {
		return prefixText
	}

	balPath, _ := resolved["balance_path"].(string)
	if balPath != "" {
		name := lastDotSegment(balPath)
		for _, strip := range []string{"Pistol_", "AR_", "SG_", "SMG_", "SR_", "RL_", "Launcher_", "Shield_", "GrenadeMod_", "ClassMod_", "Artifact_"} {
			if strings.HasPrefix(name, strip) {
				name = name[len(strip):]
				break
			}
		}
		return strings.ReplaceAll(name, "_", " ")
	}
	if cat, ok := resolved["category"].(string); ok && cat != "" {
		return cat
	}
	return "Unknown Item"
}

// LevelDamageScale mirrors the piecewise exponential damage curve.
func LevelDamageScale(level int) float64 {
	if level <= 1 {
		return 1.0
	}
	scale := 1.0
	for lv := 2; lv <= minInt(level, 31); lv++ {
		scale *= 1.13
	}
	for lv := 31; lv <= minInt(level, 50); lv++ {
		scale *= 1.10
	}
	for lv := 51; lv <= minInt(level, 61); lv++ {
		scale *= 1.09
	}
	for lv := 62; lv <= minInt(level, 72); lv++ {
		scale *= 1.08
	}
	for lv := 73; lv <= level; lv++ {
		scale *= 1.11
	}
	return scale
}

// EstimateWeaponStats estimates displayed weapon stats.
func (db *DB) EstimateWeaponStats(resolved map[string]any) map[string]any {
	category, _ := resolved["category"].(string)
	base, ok := weaponBaseStats[category]
	if !ok {
		base = weaponBaseStats["Pistol"]
	}
	stats := map[string]float64{}
	for k, v := range base {
		stats[k] = v
	}

	gradeIndex := 1.0
	gameStage := 1.0
	if level, ok := resolved["level"].([]any); ok && len(level) > 0 {
		if len(level) > 0 {
			if v := toInt64(level[0]); v != 0 {
				gradeIndex = float64(v)
			}
		}
		if len(level) > 1 {
			if v := toInt64(level[1]); v != 0 {
				gameStage = float64(v)
			}
		}
	}

	stats["damage"] = stats["damage"] * LevelDamageScale(int(gameStage))
	if gradeIndex > gameStage {
		gradeDiff := gradeIndex - gameStage
		stats["damage"] *= 1.0 + gradeDiff*0.03
		stats["fire_rate"] *= 1.0 + gradeDiff*0.004
		stats["mag_size"] *= 1.0 + gradeDiff*0.006
		stats["accuracy"] *= 1.0 + gradeDiff*0.002
		rel := 1.0 - gradeDiff*0.003
		if rel < 0.85 {
			rel = 0.85
		}
		stats["reload_speed"] *= rel
	}

	mfr, _ := resolved["manufacturer_name"].(string)
	for stat, mult := range manufacturerStatMods[mfr] {
		if _, ok := stats[stat]; ok {
			stats[stat] *= mult
		}
	}

	for _, p := range resolvedParts(resolved) {
		slot, _ := p["slot"].(string)
		slotMods, ok := partStatMods[slot]
		if !ok {
			continue
		}
		path, _ := p["path"].(string)
		if path == "" {
			continue
		}
		lower := strings.ToLower(path)
		for mfrKey, mods := range slotMods {
			if strings.Contains(lower, strings.ToLower(mfrKey)) {
				for stat, mult := range mods {
					if _, ok := stats[stat]; ok {
						stats[stat] *= mult
					}
				}
				break
			}
		}
	}

	return map[string]any{
		"damage":       int64(math.RoundToEven(stats["damage"])),
		"fire_rate":    round1(stats["fire_rate"]),
		"reload_speed": round1(stats["reload_speed"]),
		"mag_size":     maxInt64(1, int64(math.RoundToEven(stats["mag_size"]))),
		"accuracy":     minFloat(100, maxFloat(0, round1(stats["accuracy"]))),
		"recoil":       maxFloat(0, round1(stats["recoil"])),
	}
}
