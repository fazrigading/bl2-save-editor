// Package editor ports the app-level save manipulation logic from save_io.py.
package editor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"bl2save/desktop/internal/bl2save"
)

const maxSaveSize = 10 * 1024 * 1024

// Store operates on save files within one save directory.
type Store struct {
	SaveDir           string
	BackupGenerations int
	LoadoutDir        string

	locksMu sync.Mutex
	locks   map[string]*sync.Mutex
}

func NewStore(saveDir string, backupGenerations int, loadoutDir string) *Store {
	if backupGenerations <= 0 {
		backupGenerations = 5
	}
	return &Store{
		SaveDir:           saveDir,
		BackupGenerations: backupGenerations,
		LoadoutDir:        loadoutDir,
		locks:             map[string]*sync.Mutex{},
	}
}

var (
	errSaveNotFound = errors.New("save file not found")
	errSaveTooLarge = errors.New("save file too large")
	errWriteLocked  = errors.New("another operation is in progress")
)

func validSaveName(filename string) bool {
	if len(filename) != 12 || !strings.HasPrefix(filename, "Save") || !strings.HasSuffix(filename, ".sav") {
		return false
	}
	for _, c := range filename[4:8] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Store) savePath(filename string) string {
	return filepath.Join(s.SaveDir, filename)
}

func (s *Store) lock(filename string) *sync.Mutex {
	s.locksMu.Lock()
	defer s.locksMu.Unlock()
	m, ok := s.locks[filename]
	if !ok {
		m = &sync.Mutex{}
		s.locks[filename] = m
	}
	return m
}

// ListSaves enumerates save files with basic metadata.
func (s *Store) ListSaves() ([]map[string]any, error) {
	entries, err := os.ReadDir(s.SaveDir)
	if err != nil {
		return nil, err
	}
	saves := []map[string]any{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "Save") || !strings.HasSuffix(name, ".sav") {
			continue
		}
		if strings.Contains(name, "backup") || strings.Contains(name, "modded") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		saves = append(saves, map[string]any{
			"filename": name,
			"size_kb":  float64(info.Size()) / 1024.0,
			"modified": info.ModTime().Unix(),
		})
	}
	return saves, nil
}

// DuplicateSave copies a save to the next free Save number.
func (s *Store) DuplicateSave(filename string) (string, error) {
	src := s.savePath(filename)
	if _, err := os.Stat(src); err != nil {
		return "", errSaveNotFound
	}
	existing := map[int]bool{}
	entries, err := os.ReadDir(s.SaveDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) == 12 && strings.HasPrefix(name, "Save") && strings.HasSuffix(name, ".sav") {
			var num int
			if _, err := fmt.Sscanf(name[4:8], "%04d", &num); err == nil {
				existing[num] = true
			}
		}
	}
	next := 1
	for existing[next] {
		next++
	}
	if next > 9999 {
		return "", errors.New("no available save slots (Save0001-Save9999 are all used)")
	}
	dst := s.savePath(fmt.Sprintf("Save%04d.sav", next))
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	return filepath.Base(dst), nil
}

// DeleteSave soft-deletes a save by moving it to .deleted.
func (s *Store) DeleteSave(filename string) (bool, error) {
	path := s.savePath(filename)
	if _, err := os.Stat(path); err != nil {
		return false, nil
	}
	if err := os.Rename(path, path+".deleted"); err != nil {
		return false, err
	}
	return true, nil
}

// ListBackups enumerates .bak generations for a save.
func (s *Store) ListBackups(filename string) ([]map[string]any, error) {
	path := s.savePath(filename)
	backups := []map[string]any{}
	add := func(p, name string, generation int) {
		if info, err := os.Stat(p); err == nil {
			backups = append(backups, map[string]any{
				"name": name, "generation": generation,
				"size": info.Size(), "modified": info.ModTime().Unix(),
			})
		}
	}
	add(path+".bak", filename+".bak", 0)
	for i := 1; i <= s.BackupGenerations; i++ {
		add(fmt.Sprintf("%s.bak.%d", path, i), fmt.Sprintf("%s.bak.%d", filename, i), i)
	}
	add(path+".deleted", filename+".deleted", -1)
	return backups, nil
}

// RestoreBackup restores a save from a backup generation (0=.bak, -1=.deleted).
// Unlike the Python implementation, the current save is preserved as .bak
// without shifting the numbered generations, so the restored generation
// matches what ListBackups reported.
func (s *Store) RestoreBackup(filename string, generation int) (bool, error) {
	path := s.savePath(filename)
	var src string
	switch generation {
	case 0:
		src = path + ".bak"
	case -1:
		src = path + ".deleted"
	default:
		src = fmt.Sprintf("%s.bak.%d", path, generation)
	}
	if _, err := os.Stat(src); err != nil {
		return false, nil
	}
	srcData, err := os.ReadFile(src)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err == nil {
		copyFile(path, path+".bak")
	}
	if err := os.WriteFile(path, srcData, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) rotateBackups(path string) {
	for i := s.BackupGenerations - 1; i > 0; i-- {
		src := fmt.Sprintf("%s.bak.%d", path, i)
		dst := fmt.Sprintf("%s.bak.%d", path, i+1)
		if _, err := os.Stat(src); err == nil {
			os.Remove(dst)
			os.Rename(src, dst)
		}
	}
	bak := path + ".bak"
	if _, err := os.Stat(bak); err == nil {
		dst := path + ".bak.1"
		os.Remove(dst)
		os.Rename(bak, dst)
	}
	if _, err := os.Stat(path); err == nil {
		copyFile(path, bak)
	}
}

// ReadSave loads and parses a save file.
func (s *Store) ReadSave(filename string) ([]byte, bl2save.PBTree, error) {
	path := s.savePath(filename)
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, errSaveNotFound
	}
	if info.Size() > maxSaveSize {
		return nil, nil, fmt.Errorf("%w (%dKB)", errSaveTooLarge, info.Size()/1024)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	playerBytes, err := bl2save.UnwrapPlayerData(raw)
	if err != nil {
		return nil, nil, err
	}
	tree, err := bl2save.ReadProtobuf(playerBytes)
	if err != nil {
		return nil, nil, err
	}
	return raw, tree, nil
}

// WriteSave validates, backs up, and atomically writes a player tree back to
// disk, verifying the result by reading it back.
func (s *Store) WriteSave(filename string, tree bl2save.PBTree, warnOnly bool) error {
	if !validSaveName(filename) {
		return errors.New("invalid save filename")
	}
	m := s.lock(filename)
	if !m.TryLock() {
		return errWriteLocked
	}
	defer m.Unlock()
	return s.writeSaveLocked(filename, tree, warnOnly)
}

func (s *Store) writeSaveLocked(filename string, tree bl2save.PBTree, warnOnly bool) error {
	if err := s.validateItems(tree, warnOnly); err != nil {
		return err
	}
	path := s.savePath(filename)
	s.rotateBackups(path)

	playerBytes, err := bl2save.WriteProtobuf(tree)
	if err != nil {
		return err
	}
	saveBytes := bl2save.WrapPlayerData(playerBytes)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, saveBytes, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	written, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	readback, err := bl2save.UnwrapPlayerData(written)
	if err != nil {
		return fmt.Errorf("save verification failed: %w", err)
	}
	rbTree, err := bl2save.ReadProtobuf(readback)
	if err != nil {
		return fmt.Errorf("save verification failed: %w", err)
	}
	for _, field := range []int{54, 53, 41} {
		if len(rbTree[field]) != len(tree[field]) {
			return fmt.Errorf("save verification failed: field %d had %d items, readback has %d",
				field, len(tree[field]), len(rbTree[field]))
		}
	}
	return nil
}

// validateItems mirrors save_io.validate_items: structural checks on every item.
func (s *Store) validateItems(tree bl2save.PBTree, warnOnly bool) error {
	var problems []string
	for _, fc := range []struct {
		field    int
		category string
	}{{54, "weapons"}, {53, "items"}, {41, "bank"}} {
		for idx, entry := range tree[fc.field] {
			subBytes, ok := entry.Value.([]byte)
			if !ok {
				continue
			}
			sub, err := bl2save.ReadProtobuf(subBytes)
			if err != nil {
				continue
			}
			if len(sub[1]) == 0 {
				continue
			}
			rawItem, ok := sub[1][0].Value.([]byte)
			if !ok {
				continue
			}
			isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
			if err != nil {
				problems = append(problems, fmt.Sprintf("%s[%d]: failed to unpack: %v", fc.category, idx, err))
				continue
			}
			if bl2save.IsFakeItem(isWeapon, values) {
				continue
			}
			if values[1] == bl2save.ItemNone && values[2] == bl2save.ItemNone && values[3] == bl2save.ItemNone {
				problems = append(problems, fmt.Sprintf("%s[%d]: corrupted item (all critical fields are None)", fc.category, idx))
				continue
			}
			sizes := bl2save.ItemSizes[isWeapon]
			for i, val := range values {
				if i >= len(sizes) || val == bl2save.ItemNone {
					continue
				}
				maxVal := int64(1)<<uint(sizes[i]) - 1
				if val < 0 || val > maxVal {
					problems = append(problems, fmt.Sprintf("%s[%d] field %d: value %d exceeds %d-bit max (%d)",
						fc.category, idx, i, val, sizes[i], maxVal))
				}
			}
			grade := int64(0)
			if len(values) > 4 && values[4] != bl2save.ItemNone {
				grade = values[4]
			}
			stage := int64(0)
			if len(values) > 5 && values[5] != bl2save.ItemNone {
				stage = values[5]
			}
			if grade > 127 {
				problems = append(problems, fmt.Sprintf("%s[%d]: grade_index %d exceeds max 127", fc.category, idx, grade))
			}
			if stage > 127 {
				problems = append(problems, fmt.Sprintf("%s[%d]: game_stage %d exceeds max 127", fc.category, idx, stage))
			}
		}
	}
	if len(problems) > 0 {
		msg := fmt.Sprintf("Item validation (%d errors): ", len(problems)) + strings.Join(problems[:min(5, len(problems))], "; ")
		if warnOnly {
			return nil
		}
		return errors.New(msg)
	}
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
