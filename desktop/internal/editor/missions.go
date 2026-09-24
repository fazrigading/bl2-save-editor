package editor

import (
	"strings"

	"bl2save/desktop/internal/bl2save"
)

func formatMissionName(path string) string {
	if path == "" {
		return ""
	}
	if name, ok := missionDisplay[path]; ok {
		return name
	}
	name := lastDotSegment(path)
	if strings.HasPrefix(name, "M_") {
		name = name[2:]
	}
	if idx := epPrefixLen(name); idx > 0 {
		name = name[idx:]
	}
	var b strings.Builder
	for i := 0; i < len(name); i++ {
		if i > 0 && isLower(name[i-1]) && isUpper(name[i]) {
			b.WriteByte(' ')
		}
		b.WriteByte(name[i])
	}
	return strings.ReplaceAll(b.String(), "_", " ")
}

func isLower(c byte) bool { return c >= 'a' && c <= 'z' }
func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

func epPrefixLen(name string) int {
	if len(name) < 3 || name[0] != 'E' || name[1] != 'p' {
		return 0
	}
	i := 2
	digits := 0
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
		digits++
	}
	if digits == 0 {
		return 0
	}
	if i < len(name) && name[i] >= 'a' && name[i] <= 'z' {
		i++
	}
	if i < len(name) && name[i] == '_' {
		return i + 1
	}
	return 0
}

// ExtractMissions returns mission data grouped per playthrough.
func (s *Store) ExtractMissions(tree bl2save.PBTree) map[string]any {
	activePt := getUint(tree, 49, 0)
	playthroughs := []map[string]any{}
	for ptIdx, ptEntry := range tree[18] {
		ptData, ok := subTree(ptEntry)
		if !ok {
			continue
		}
		activeMission := latin1(getBytes(ptData, 2))
		missions := []map[string]any{}
		for _, mEntry := range ptData[3] {
			md, ok := subTree(mEntry)
			if !ok {
				continue
			}
			name := latin1(getBytes(md, 1))
			status := getUint(md, 2, 0)
			statusText := missionStatusNames[status]
			if statusText == "" {
				statusText = "Unknown"
			}
			missions = append(missions, map[string]any{
				"name":         name,
				"display_name": formatMissionName(name),
				"status":       status,
				"status_text":  statusText,
				"level":        getUint(md, 11, 0),
				"is_dlc":       getUint(md, 3, 0) != 0,
				"dlc_id":       getUint(md, 4, 0),
			})
		}
		label := "PT1"
		if ptIdx < 3 {
			label = playthroughLabels[ptIdx]
		} else {
			label = "PT" + itoa(ptIdx+1)
		}
		playthroughs = append(playthroughs, map[string]any{
			"index":                  ptIdx,
			"label":                  label,
			"active_mission":         activeMission,
			"active_mission_display": formatMissionName(activeMission),
			"missions":               missions,
		})
	}
	return map[string]any{
		"active_playthrough": activePt,
		"playthroughs":       playthroughs,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func (s *Store) playthroughTree(tree bl2save.PBTree, playthrough int) (bl2save.PBTree, bool) {
	if playthrough < 0 || playthrough >= len(tree[18]) {
		return nil, false
	}
	return subTree(tree[18][playthrough])
}

// SetMissionStatus changes one mission's status (1=Active, 4=Complete).
func (s *Store) SetMissionStatus(filename string, playthrough int, missionName string, newStatus uint64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return false, nil
	}
	found := false
	for i, mEntry := range ptData[3] {
		md, ok := subTree(mEntry)
		if !ok {
			continue
		}
		if latin1(getBytes(md, 1)) == missionName {
			setEntry(md, 2, 0, newStatus)
			ptData[3][i].Value, _ = bl2save.WriteProtobuf(md)
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
	return true, s.WriteSave(filename, tree, false)
}

// CompleteAllMissions marks every mission in a playthrough complete.
func (s *Store) CompleteAllMissions(filename string, playthrough int) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return 0, nil
	}
	count := 0
	for i, mEntry := range ptData[3] {
		md, ok := subTree(mEntry)
		if !ok {
			continue
		}
		if getUint(md, 2, 0) != 4 {
			setEntry(md, 2, 0, uint64(4))
			ptData[3][i].Value, _ = bl2save.WriteProtobuf(md)
			count++
		}
	}
	if count > 0 {
		tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
		return count, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}

// AddMission adds a mission to a playthrough (rejects duplicates).
func (s *Store) AddMission(filename string, playthrough int, missionName string, status uint64, level int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return false, nil
	}
	for _, mEntry := range ptData[3] {
		if md, ok := subTree(mEntry); ok && latin1(getBytes(md, 1)) == missionName {
			return false, nil
		}
	}
	numObj, ok := missionObjectiv[missionName]
	if !ok {
		numObj = 5
	}
	objBytes := make([]byte, numObj)
	if status == 4 {
		for i := range objBytes {
			objBytes[i] = 1
		}
	}
	count := numObj
	if status != 4 {
		count = 0
	}
	missionPb, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1:  {{WireType: 2, Value: []byte(missionName)}},
		2:  {{WireType: 0, Value: status}},
		3:  {{WireType: 0, Value: uint64(0)}},
		4:  {{WireType: 0, Value: uint64(0)}},
		5:  {{WireType: 2, Value: objBytes}},
		6:  {{WireType: 0, Value: uint64(count)}},
		8:  {{WireType: 0, Value: uint64(0)}},
		10: {{WireType: 0, Value: uint64(1)}},
		11: {{WireType: 0, Value: uint64(level)}},
	})
	ptData[3] = append(ptData[3], bl2save.PBEntry{WireType: 2, Value: missionPb})
	tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
	return true, s.WriteSave(filename, tree, false)
}

// RemoveMission removes a mission, clearing the active pointer if needed.
func (s *Store) RemoveMission(filename string, playthrough int, missionName string) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return false, nil
	}
	var kept []bl2save.PBEntry
	found := false
	for _, mEntry := range ptData[3] {
		if md, ok := subTree(mEntry); ok && latin1(getBytes(md, 1)) == missionName {
			found = true
			continue
		}
		kept = append(kept, mEntry)
	}
	if !found {
		return false, nil
	}
	ptData[3] = kept
	if latin1(getBytes(ptData, 2)) == missionName {
		setEntry(ptData, 2, 2, []byte(""))
	}
	tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
	return true, s.WriteSave(filename, tree, false)
}

// AddAllStoryMissions adds every story mission not yet present.
func (s *Store) AddAllStoryMissions(filename string, playthrough int, status uint64) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return 0, nil
	}
	existing := map[string]bool{}
	for _, mEntry := range ptData[3] {
		if md, ok := subTree(mEntry); ok {
			existing[latin1(getBytes(md, 1))] = true
		}
	}
	count := 0
	for _, path := range StoryMissions {
		if existing[path] {
			continue
		}
		numObj := missionObjectiv[path]
		level := missionLevels[path]
		objBytes := make([]byte, numObj)
		if status == 4 {
			for i := range objBytes {
				objBytes[i] = 1
			}
		}
		countField := uint64(0)
		if status == 4 {
			countField = uint64(numObj)
		}
		missionPb, _ := bl2save.WriteProtobuf(bl2save.PBTree{
			1:  {{WireType: 2, Value: []byte(path)}},
			2:  {{WireType: 0, Value: status}},
			3:  {{WireType: 0, Value: uint64(0)}},
			4:  {{WireType: 0, Value: uint64(0)}},
			5:  {{WireType: 2, Value: objBytes}},
			6:  {{WireType: 0, Value: countField}},
			8:  {{WireType: 0, Value: uint64(0)}},
			10: {{WireType: 0, Value: uint64(1)}},
			11: {{WireType: 0, Value: uint64(level)}},
		})
		ptData[3] = append(ptData[3], bl2save.PBEntry{WireType: 2, Value: missionPb})
		count++
	}
	if count > 0 {
		tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
		return count, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}

// SetActiveMission sets the tracked mission, verifying it exists (BUG-34).
func (s *Store) SetActiveMission(filename string, playthrough int, missionName string) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	ptData, ok := s.playthroughTree(tree, playthrough)
	if !ok {
		return false, nil
	}
	found := false
	for _, mEntry := range ptData[3] {
		if md, ok := subTree(mEntry); ok && latin1(getBytes(md, 1)) == missionName {
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	setEntry(ptData, 2, 2, []byte(missionName))
	tree[18][playthrough].Value, _ = bl2save.WriteProtobuf(ptData)
	return true, s.WriteSave(filename, tree, false)
}

// MissionDBJSON returns the mission database for the frontend.
func MissionDBJSON() []map[string]any {
	out := make([]map[string]any, 0, len(MissionDB))
	for _, m := range MissionDB {
		out = append(out, map[string]any{
			"path": m.Path, "name": m.Name, "objectives": m.Objectives,
			"is_story": m.IsStory, "level": m.Level,
		})
	}
	return out
}
