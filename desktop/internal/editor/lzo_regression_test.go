package editor

import (
	"os"
	"path/filepath"
	"testing"

	"bl2save/desktop/internal/bl2save"
)

// Regression: item key 36 with a seeded weapon present produced an
// LZO-undecodable save ("save verification failed: lzo1x data out of
// range"). The compressor emitted a zero terminator in extended-length
// sequences whenever the remainder was a multiple of 255, desyncing the
// decoder. Deterministic — no randomness involved.
func TestWriteSaveKey36Regression(t *testing.T) {
	dir := t.TempDir()
	data, err := os.ReadFile("../../../tests/Save0001.sav")
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(dir, 5, filepath.Join(dir, "loadouts"))
	_, tree, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	addEntry := func(field int, isWeapon int, key int64, f2, f3 uint64, extra ...bl2save.PBEntry) {
		t.Helper()
		raw := bl2save.WrapItem(isWeapon, synthValsForTest, key)
		fields := bl2save.PBTree{
			1: {{WireType: 2, Value: raw}},
			2: {{WireType: 0, Value: f2}},
			3: {{WireType: 0, Value: f3}},
		}
		for i, e := range extra {
			fields[4+i] = []bl2save.PBEntry{e}
		}
		entry, err := bl2save.WriteProtobuf(fields)
		if err != nil {
			t.Fatal(err)
		}
		tree[field] = append(tree[field], bl2save.PBEntry{WireType: 2, Value: entry})
	}
	addEntry(54, 1, 12345, 0, 1)
	addEntry(53, 0, 36, 1, 0, bl2save.PBEntry{WireType: 0, Value: uint64(1)})
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatalf("WriteSave with item key 36: %v", err)
	}
	if _, _, err := store.ReadSave("Save0001.sav"); err != nil {
		t.Fatalf("readback: %v", err)
	}
}

var synthValsForTest = []int64{0, 1, 2, 3, 10, 10, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14}
