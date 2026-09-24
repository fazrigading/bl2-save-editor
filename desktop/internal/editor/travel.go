package editor

import (
	"bl2save/desktop/internal/bl2save"
)

// ExtractFastTravel returns unlocked stations and last visited.
func (s *Store) ExtractFastTravel(tree bl2save.PBTree) map[string]any {
	stations := []map[string]any{}
	for _, entry := range tree[16] {
		if v, ok := entry.Value.([]byte); ok {
			name := latin1(v)
			stations = append(stations, map[string]any{
				"name":         name,
				"display_name": orDefault(stationDisplay[name], name),
			})
		}
	}
	last := latin1(getBytes(tree, 17))
	return map[string]any{
		"stations":             stations,
		"last_visited":         last,
		"last_visited_display": orDefault(stationDisplay[last], last),
	}
}

// UpdateFastTravel replaces the station list, validated against known stations.
func (s *Store) UpdateFastTravel(filename string, stations []string) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	known := map[string]bool{}
	for _, st := range AllStations {
		known[st] = true
	}
	entries := make([]bl2save.PBEntry, 0, len(stations))
	for _, st := range stations {
		if !known[st] {
			return false, errUnknownStation(st)
		}
		entries = append(entries, bl2save.PBEntry{WireType: 2, Value: []byte(st)})
	}
	tree[16] = entries
	return true, s.WriteSave(filename, tree, false)
}

// UnlockAllFastTravel adds all base-game stations not yet present.
func (s *Store) UnlockAllFastTravel(filename string) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	existing := map[string]bool{}
	for _, entry := range tree[16] {
		if v, ok := entry.Value.([]byte); ok {
			existing[latin1(v)] = true
		}
	}
	added := 0
	for _, station := range BaseStations {
		if !existing[station] {
			tree[16] = append(tree[16], bl2save.PBEntry{WireType: 2, Value: []byte(station)})
			added++
		}
	}
	if added > 0 {
		return added, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}

type unknownStationError string

func (e unknownStationError) Error() string { return "unknown station: " + string(e) }

func errUnknownStation(s string) error { return unknownStationError(s) }
