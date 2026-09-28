package bridge

import (
	"errors"

	"bl2save/desktop/internal/editor"
)

// Editor covers save-scoped reads and mutations: character fields, skills,
// ammo, playthroughs, missions, fast travel and challenges. Mutations return
// the fresh save state merged into their result so the frontend can
// re-render from a single round trip.
type Editor struct{ b *Bridge }

func (s *Editor) UpdateCharacter(filename string, payload map[string]any) (any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	return store.UpdateCharacter(filename, payload)
}

func (s *Editor) SetSkills(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	updates := map[string]int64{}
	if raw, ok := payload["skills"].(map[string]any); ok {
		for k, v := range raw {
			updates[k] = int64(toFloat(v))
		}
	}
	count, err := store.SetSkills(filename, updates)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}

func (s *Editor) SetAmmo(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	updates := map[string]int64{}
	if raw, ok := payload["ammo"].(map[string]any); ok {
		for k, v := range raw {
			updates[k] = int64(toFloat(v))
		}
	}
	ok, err := store.SetAmmo(filename, updates)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": ok}, nil
}

func (s *Editor) FillAmmo(filename string) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	ok, err := store.FillAllAmmo(filename)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": ok}, filename)
}

func (s *Editor) Playthrough(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	switch pStr(payload, "action", "") {
	case "unlock_tvhm":
		if _, err := store.UnlockPlaythrough(filename, "tvhm"); err != nil {
			return nil, err
		}
	case "unlock_uvhm":
		if _, err := store.UnlockPlaythrough(filename, "uvhm"); err != nil {
			return nil, err
		}
	case "set_active":
		if _, err := store.SetActivePlaythrough(filename, pInt(payload, "playthrough", 0)); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("Unknown action")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) UnlockAchievements(filename string) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	result, err := store.UnlockAchievements(filename)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"ok": true}
	for k, v := range result {
		out[k] = v
	}
	return s.b.withState(out, filename)
}

func (s *Editor) SpawnTestWeapons(filename string) (any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	return store.SpawnTestWeapons(filename)
}

func (s *Editor) CompleteAllMissions(filename string, pt int) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	count, err := store.CompleteAllMissions(filename, pt)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}

func (s *Editor) AddMission(filename string, pt int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	mission := pStr(payload, "mission", "")
	status := pInt(payload, "status", 1)
	level := pInt(payload, "level", 1)
	if mission == "" {
		return nil, errors.New("Mission name required")
	}
	if status != 1 && status != 4 {
		return nil, errors.New("Status must be 1 (Active) or 4 (Complete)")
	}
	if level < 1 || level > 127 {
		return nil, errors.New("Level must be 1-127")
	}
	ok, err := store.AddMission(filename, pt, mission, uint64(status), level)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Mission already exists or invalid playthrough")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) RemoveMission(filename string, pt int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	ok, err := store.RemoveMission(filename, pt, pStr(payload, "mission", ""))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Mission not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) AddAllStory(filename string, pt int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	status := pInt(payload, "status", 1)
	if status != 1 && status != 4 {
		return nil, errors.New("Status must be 1 (Active) or 4 (Complete)")
	}
	count, err := store.AddAllStoryMissions(filename, pt, uint64(status))
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}

func (s *Editor) SetActiveMission(filename string, pt int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	ok, err := store.SetActiveMission(filename, pt, pStr(payload, "mission", ""))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Mission not found in this playthrough")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) SetMissionStatus(filename string, pt int, mission string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	newStatus := pInt(payload, "status", 4)
	if newStatus != 1 && newStatus != 4 {
		return nil, errors.New("Status must be 1 (Active) or 4 (Complete)")
	}
	ok, err := store.SetMissionStatus(filename, pt, mission, uint64(newStatus))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Mission not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) MissionDB() (any, error) {
	return editor.MissionDBJSON(), nil
}

func (s *Editor) UpdateFastTravel(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	raw, ok := payload["stations"].([]any)
	if !ok {
		return nil, errors.New("stations must be a list")
	}
	stations := make([]string, 0, len(raw))
	for _, st := range raw {
		if str, ok := st.(string); ok {
			stations = append(stations, str)
		}
	}
	if ok, err := store.UpdateFastTravel(filename, stations); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("update failed")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Editor) UnlockAllFastTravel(filename string) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	count, err := store.UnlockAllFastTravel(filename)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "added": count}, filename)
}

func (s *Editor) AllStations() ([]map[string]any, error) {
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for _, st := range store.AllStations() {
		out = append(out, map[string]any{"name": st, "display_name": store.StationDisplay(st)})
	}
	return out, nil
}

func (s *Editor) GetChallenges(filename string) (any, error) {
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	_, tree, err := store.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	return store.ExtractChallenges(tree), nil
}

func (s *Editor) SetChallenge(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	progress := pInt(payload, "progress", 0)
	completed := pInt(payload, "completed", 0)
	if progress < 0 || progress > 999999 {
		return nil, errors.New("Progress must be 0-999999")
	}
	if completed < 0 || completed > 999 {
		return nil, errors.New("Completed count must be 0-999")
	}
	ok, err := store.SetChallengeProgress(filename, pStr(payload, "path", ""), int64(progress), int64(completed))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Challenge not found")
	}
	return map[string]any{"ok": true}, nil
}

func (s *Editor) CompleteAllChallenges(filename string) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	count, err := store.CompleteAllChallenges(filename)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}

func (s *Editor) ResetAllChallenges(filename string) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	count, err := store.ResetAllChallenges(filename)
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}
