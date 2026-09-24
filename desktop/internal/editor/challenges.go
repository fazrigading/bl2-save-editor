package editor

import (
	"strings"

	"bl2save/desktop/internal/bl2save"
)

func formatChallengeName(path string) string {
	name := lastDotSegment(path)
	for _, prefix := range []string{"Challenge_Kill_", "Challenge_", "General_", "Player_", "Enemies_Kill", "Enemies_", "MapECHO_"} {
		if strings.HasPrefix(name, prefix) {
			name = name[len(prefix):]
			break
		}
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

// ExtractChallenges groups challenge data from field 38 by category.
func (s *Store) ExtractChallenges(tree bl2save.PBTree) map[string]any {
	categories := map[string][]map[string]any{}
	for _, entry := range tree[38] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		path := latin1(getBytes(sub, 1))
		parts := strings.Split(path, ".")
		catKey := "Other"
		if len(parts) > 1 {
			catKey = parts[1]
		}
		catDisplay := orDefault(challengeCategories[catKey], catKey)
		categories[catDisplay] = append(categories[catDisplay], map[string]any{
			"path":            path,
			"display_name":    formatChallengeName(path),
			"category":        catDisplay,
			"progress":        getUint(sub, 2, 0),
			"completed_count": getUint(sub, 3, 0),
		})
	}
	return map[string]any{
		"categories": categories,
		"total":      len(tree[38]),
	}
}

// SetChallengeProgress sets one challenge's progress and completion count.
func (s *Store) SetChallengeProgress(filename, challengePath string, progress, completed int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	for i, entry := range tree[38] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		if latin1(getBytes(sub, 1)) == challengePath {
			if progress < 0 {
				progress = 0
			}
			if completed < 0 {
				completed = 0
			}
			setEntry(sub, 2, 0, uint64(progress))
			setEntry(sub, 3, 0, uint64(completed))
			tree[38][i].Value, _ = bl2save.WriteProtobuf(sub)
			return true, s.WriteSave(filename, tree, false)
		}
	}
	return false, nil
}

// CompleteAllChallenges marks all challenges complete with high progress (BUG-32).
func (s *Store) CompleteAllChallenges(filename string) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	count := 0
	for i, entry := range tree[38] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		if getUint(sub, 3, 0) < 1 {
			setEntry(sub, 2, 0, uint64(99999))
			setEntry(sub, 3, 0, uint64(1))
			tree[38][i].Value, _ = bl2save.WriteProtobuf(sub)
			count++
		}
	}
	if count > 0 {
		return count, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}

// ResetAllChallenges zeroes all challenge progress.
func (s *Store) ResetAllChallenges(filename string) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	count := 0
	for i, entry := range tree[38] {
		sub, ok := subTree(entry)
		if !ok {
			continue
		}
		if getUint(sub, 2, 0) > 0 || getUint(sub, 3, 0) > 0 {
			setEntry(sub, 2, 0, uint64(0))
			setEntry(sub, 3, 0, uint64(0))
			tree[38][i].Value, _ = bl2save.WriteProtobuf(sub)
			count++
		}
	}
	if count > 0 {
		return count, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}
