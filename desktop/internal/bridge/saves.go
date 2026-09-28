package bridge

import (
	"errors"
	"time"

	"bl2save/desktop/internal/bl2save"
)

// Saves covers save-file lifecycle: listing, previews, load, duplicate,
// delete and backups.
type Saves struct{ b *Bridge }

func getUint64(tree bl2save.PBTree, field int, def uint64) uint64 {
	entries := tree[field]
	if len(entries) == 0 {
		return def
	}
	if v, ok := entries[0].Value.(uint64); ok {
		return v
	}
	return def
}

func classNameFromTree(tree bl2save.PBTree) string {
	entries := tree[1]
	if len(entries) == 0 {
		return "?"
	}
	if v, ok := entries[0].Value.([]byte); ok {
		return string(v)
	}
	return "?"
}

func (s *Saves) ListSaves() ([]map[string]any, error) {
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	return store.ListSaves()
}

func (s *Saves) SavePreviews() (map[string]any, error) {
	defer slowLog("Saves.SavePreviews", time.Now())
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	saves, err := store.ListSaves()
	if err != nil {
		return nil, err
	}
	results := map[string]any{}
	s.b.mu.Lock()
	defer s.b.mu.Unlock()
	for _, sv := range saves {
		fname := sv["filename"].(string)
		mtime := sv["modified"].(int64)
		if cached, ok := s.b.previewCache[fname]; ok && cached.mtime == mtime {
			results[fname] = cached.data
			continue
		}
		preview := map[string]any{"class_name": "?", "level": 0}
		if _, tree, err := store.ReadSave(fname); err == nil {
			preview = map[string]any{
				"class_name": classNameFromTree(tree),
				"level":      getUint64(tree, 2, 1),
			}
		}
		s.b.previewCache[fname] = previewEntry{mtime: mtime, data: preview}
		results[fname] = preview
	}
	return results, nil
}

func (s *Saves) LoadSave(filename string) (map[string]any, error) {
	return s.b.state(filename)
}

func (s *Saves) DuplicateSave(filename string) (map[string]any, error) {
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	newName, err := store.DuplicateSave(filename)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "new_filename": newName}, nil
}

func (s *Saves) DeleteSave(filename string) (map[string]any, error) {
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	ok, err := store.DeleteSave(filename)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Save not found")
	}
	s.b.mu.Lock()
	delete(s.b.previewCache, filename)
	s.b.mu.Unlock()
	return map[string]any{"ok": true}, nil
}

func (s *Saves) ListBackups(filename string) (map[string]any, error) {
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	backups, err := store.ListBackups(filename)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "backups": backups}, nil
}

func (s *Saves) RestoreBackup(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	gen := int(toFloat(payload["generation"]))
	ok, err := store.RestoreBackup(filename, gen)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Backup not found")
	}
	return map[string]any{"ok": true}, nil
}
