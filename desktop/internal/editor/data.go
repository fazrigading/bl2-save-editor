package editor

import (
	_ "embed"
	"encoding/json"

	"bl2save/desktop/internal/bl2save"
)

//go:embed data/missions.json
var missionsJSON []byte

//go:embed data/stations.json
var stationsJSON []byte

//go:embed data/achievements.json
var achievementsJSON []byte

type missionEntry struct {
	Path       string
	Name       string
	Objectives int
	IsStory    bool
	Level      int
}

type achievementEntry struct {
	APIName  string
	Name     string
	Desc     string
	Category string
}

var (
	MissionDB       []missionEntry
	StoryMissions   []string
	missionDisplay  map[string]string
	missionObjectiv map[string]int
	missionLevels   map[string]int

	stationDisplay map[string]string
	BaseStations   []string
	DLCStations    map[string][]string
	AllStations    []string

	AchievementDB []achievementEntry
)

func init() {
	var rawMissions [][5]any
	if err := json.Unmarshal(missionsJSON, &rawMissions); err != nil {
		panic(err)
	}
	missionDisplay = map[string]string{}
	missionObjectiv = map[string]int{}
	missionLevels = map[string]int{}
	for _, m := range rawMissions {
		entry := missionEntry{
			Path:       m[0].(string),
			Name:       m[1].(string),
			Objectives: int(m[2].(float64)),
			IsStory:    m[3].(bool),
			Level:      int(m[4].(float64)),
		}
		MissionDB = append(MissionDB, entry)
		missionDisplay[entry.Path] = entry.Name
		missionObjectiv[entry.Path] = entry.Objectives
		missionLevels[entry.Path] = entry.Level
		if entry.IsStory {
			StoryMissions = append(StoryMissions, entry.Path)
		}
	}

	var rawStations struct {
		DisplayNames map[string]string   `json:"display_names"`
		Base         []string            `json:"base"`
		DLC          map[string][]string `json:"dlc"`
	}
	if err := json.Unmarshal(stationsJSON, &rawStations); err != nil {
		panic(err)
	}
	stationDisplay = rawStations.DisplayNames
	BaseStations = rawStations.Base
	DLCStations = rawStations.DLC
	AllStations = append(AllStations, BaseStations...)
	for _, dlc := range rawStations.DLC {
		AllStations = append(AllStations, dlc...)
	}

	var rawAchievements [][4]string
	if err := json.Unmarshal(achievementsJSON, &rawAchievements); err != nil {
		panic(err)
	}
	for _, a := range rawAchievements {
		AchievementDB = append(AchievementDB, achievementEntry{APIName: a[0], Name: a[1], Desc: a[2], Category: a[3]})
	}
}

var requiredXP = []int64{
	0, 358, 1241, 2850, 5376, 8997, 13886, 20208, 28126, 37798,
	49377, 63016, 78861, 97061, 117757, 141092, 167206, 196238, 228322, 263595,
	302190, 344238, 389873, 439222, 492414, 549578, 610840, 676325, 746158, 820463,
	899363, 982980, 1071435, 1164850, 1263343, 1367034, 1476041, 1590483, 1710476, 1836137,
	1967582, 2104926, 2248285, 2397772, 2553501, 2715586, 2884139, 3059273, 3241098, 3429728,
	3625271, 3827840, 4037543, 4254491, 4478792, 4710556, 4949890, 5196902, 5451701, 5714393,
	5985086, 6263885, 6550897, 6846227, 7149982, 7462266, 7783184, 8112840, 8451340, 8798786,
	9155282, 9520931, 9895837, 10280103, 10673830, 11077120, 11490077, 11912801, 12345393, 12787955,
}

var classNames = map[string]string{
	"GD_Soldier.Character.CharClass_Soldier":                    "Axton",
	"GD_Assassin.Character.CharClass_Assassin":                  "Zer0",
	"GD_Siren.Character.CharClass_Siren":                        "Maya",
	"GD_Mercenary.Character.CharClass_Mercenary":                "Salvador",
	"GD_Tulip_Mechromancer.Character.CharClass_Mechromancer":    "Gaige",
	"GD_Lilac_PlayerClass.Character.CharClass_LilacPlayerClass": "Krieg",
}

var ammoPools = []struct {
	Key     string
	Display string
	Max     int
}{
	{"Ammo_Combat_Rifle", "Assault Rifle", 280 * 4},
	{"Ammo_Combat_Shotgun", "Shotgun", 80 * 4},
	{"Ammo_Grenade_Protean", "Grenades", 10},
	{"Ammo_Combat_Launcher", "Rocket Launcher", 12 * 4},
	{"Ammo_Patrol_SMG", "SMG", 360 * 4},
	{"Ammo_Repeater_Pistol", "Pistol", 200 * 4},
	{"Ammo_Sniper_Rifle", "Sniper Rifle", 48 * 4},
}

var (
	ammoDisplay map[string]string
	ammoMaxes   map[string]int
)

var challengeCategories = map[string]string{
	"Weapons":             "Weapons",
	"GeneralCombat":       "General Combat",
	"Grenades":            "Grenades",
	"Shields":             "Shields",
	"elemental":           "Elemental",
	"enemies":             "Enemies",
	"Vehicles":            "Vehicles",
	"Player":              "Player",
	"Melee":               "Melee",
	"Economy":             "Economy",
	"Loot":                "Loot",
	"Pickups":             "Pickups",
	"Miscellaneous":       "Miscellaneous",
	"Dueling":             "Dueling",
	"Challenges":          "Challenges",
	"LevelChallenges":     "Area Challenges",
	"LevelECHOChallenges": "ECHO Logs",
}

var playthroughLabels = []string{"Normal", "TVHM", "UVHM"}

var missionStatusNames = map[uint64]string{1: "Active", 4: "Complete"}

func init() {
	ammoDisplay = map[string]string{}
	ammoMaxes = map[string]int{}
	for _, p := range ammoPools {
		ammoDisplay[p.Key] = p.Display
		ammoMaxes[p.Key] = p.Max
	}
}

// itemEntryPayloads enumerates the item sections with their category names.
var itemSections = []struct {
	Field    int
	Category string
}{
	{54, "weapons"},
	{53, "items"},
	{41, "bank"},
}

// pb helpers -----------------------------------------------------------------

func subTree(entry bl2save.PBEntry) (bl2save.PBTree, bool) {
	b, ok := entry.Value.([]byte)
	if !ok {
		return nil, false
	}
	t, err := bl2save.ReadProtobuf(b)
	if err != nil {
		return nil, false
	}
	return t, true
}

func getUint(tree bl2save.PBTree, field int, def uint64) uint64 {
	entries := tree[field]
	if len(entries) == 0 {
		return def
	}
	if v, ok := entries[0].Value.(uint64); ok {
		return v
	}
	return def
}

func getBytes(tree bl2save.PBTree, field int) []byte {
	entries := tree[field]
	if len(entries) == 0 {
		return nil
	}
	if v, ok := entries[0].Value.([]byte); ok {
		return v
	}
	return nil
}

func setEntry(tree bl2save.PBTree, field, wireType int, value any) {
	tree[field] = []bl2save.PBEntry{{WireType: wireType, Value: value}}
}

func latin1(b []byte) string {
	return string(b)
}

func deepCopyTree(tree bl2save.PBTree) bl2save.PBTree {
	out := make(bl2save.PBTree, len(tree))
	for k, entries := range tree {
		c := make([]bl2save.PBEntry, len(entries))
		for i, e := range entries {
			ne := e
			switch v := e.Value.(type) {
			case []byte:
				ne.Value = append([]byte(nil), v...)
			case *bl2save.PBTree:
				sub := deepCopyTree(*v)
				ne.Value = &sub
			case []uint64:
				ne.Value = append([]uint64(nil), v...)
			}
			c[i] = ne
		}
		out[k] = c
	}
	return out
}

// readRepeatedVarints mirrors read_repeated_protobuf_value for wire type 0.
func readRepeatedVarints(data []byte) []uint64 {
	var out []uint64
	pos := 0
	for pos < len(data) {
		var value uint64
		var offset uint
		for {
			if pos >= len(data) {
				return out
			}
			b := data[pos]
			pos++
			value |= uint64(b&0x7F) << offset
			if b&0x80 == 0 {
				break
			}
			offset += 7
		}
		out = append(out, value)
	}
	return out
}

// currencyValues reads the currency list from field 6, handling both packed
// bytes (fresh read) and []uint64 (after in-place updates).
func currencyValues(tree bl2save.PBTree) []uint64 {
	entries := tree[6]
	if len(entries) == 0 {
		return nil
	}
	switch v := entries[0].Value.(type) {
	case []uint64:
		return v
	case []byte:
		return readRepeatedVarints(v)
	default:
		return nil
	}
}
