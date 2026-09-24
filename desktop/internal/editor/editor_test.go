package editor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bl2save/desktop/internal/bl2save"
)

const fixtureSave = "../../../tests/Save0001.sav"

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	data, err := os.ReadFile(fixtureSave)
	if err != nil {
		t.Skipf("fixture missing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Save0001.sav"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return NewStore(dir, 5, filepath.Join(dir, "loadouts")), dir
}

func TestReadSaveFixture(t *testing.T) {
	store, _ := newTestStore(t)
	raw, tree, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatalf("ReadSave: %v", err)
	}
	if len(raw) == 0 || len(tree) == 0 {
		t.Fatal("empty save")
	}
	if got := string(getBytes(tree, 1)); got != "GD_Soldier.Character.CharClass_Soldier" {
		t.Fatalf("unexpected class: %q", got)
	}
	if getUint(tree, 2, 0) == 0 {
		t.Fatal("level missing")
	}
}

func TestWriteRoundTripUnchanged(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatalf("WriteSave: %v", err)
	}
	_, tree2, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != len(tree2) {
		t.Fatal("field count changed after round trip")
	}
}

func TestBackupRotation(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
			t.Fatal(err)
		}
	}
	backups, _ := store.ListBackups("Save0001.sav")
	generations := map[int]bool{}
	for _, b := range backups {
		generations[b["generation"].(int)] = true
	}
	if !generations[0] || !generations[1] || !generations[5] {
		t.Fatalf("expected .bak and .bak.1..5, got %v", generations)
	}
	if generations[6] {
		t.Fatal("generation 6 should have rotated away")
	}
}

func TestRestoreBackup(t *testing.T) {
	store, dir := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	setEntry(tree, 2, 0, uint64(42))
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatal(err)
	}
	ok, err := store.RestoreBackup("Save0001.sav", 0)
	if err != nil || !ok {
		t.Fatalf("restore: %v %v", ok, err)
	}
	_, restored, err := store.ReadSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	if getUint(restored, 2, 0) == 42 {
		t.Fatal("backup restore did not revert level")
	}
	_ = dir
}

func TestValidateItemsRejectsCorrupt(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	// Inject a truncated item: unpacking yields all-None critical fields.
	values := make([]int64, len(bl2save.ItemSizes[1]))
	for i := range values {
		values[i] = 1
	}
	raw := bl2save.WrapItem(1, values, 0)
	entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1: {{WireType: 2, Value: raw[:6]}},
		2: {{WireType: 0, Value: uint64(0)}},
		3: {{WireType: 0, Value: uint64(1)}},
	})
	tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entryBytes})

	if err := store.WriteSave("Save0001.sav", tree, false); err == nil {
		t.Fatal("expected validation error")
	}
	if err := store.WriteSave("Save0001.sav", tree, true); err != nil {
		t.Fatalf("warn-only write should pass: %v", err)
	}
}

func TestItemOperations(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")

	values := make([]int64, len(bl2save.ItemSizes[1]))
	for i := range values {
		values[i] = int64(i + 1)
	}
	raw := bl2save.WrapItem(1, values, 0)
	entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1: {{WireType: 2, Value: raw}},
		2: {{WireType: 0, Value: uint64(0)}},
		3: {{WireType: 0, Value: uint64(1)}},
	})
	tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entryBytes})
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatal(err)
	}

	ok, err := store.SetItemLevel("Save0001.sav", 54, 0, 72)
	if err != nil || !ok {
		t.Fatalf("SetItemLevel: %v %v", ok, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	sub, _ := subTree(tree[54][0])
	rawItem := sub[1][0].Value.([]byte)
	_, itemValues, _, err := bl2save.UnwrapItem(rawItem)
	if err != nil {
		t.Fatal(err)
	}
	if itemValues[4] != 72 || itemValues[5] != 72 {
		t.Fatalf("level not applied: %v", itemValues[4:6])
	}

	ok, err = store.TransferItem("Save0001.sav", 54, 0, 41)
	if err != nil || !ok {
		t.Fatalf("TransferItem: %v %v", ok, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	if len(tree[54]) != 0 || len(tree[41]) != 1 {
		t.Fatalf("transfer failed: weapons=%d bank=%d", len(tree[54]), len(tree[41]))
	}

	ok, err = store.DeleteItem("Save0001.sav", 41, 0)
	if err != nil || !ok {
		t.Fatalf("DeleteItem: %v %v", ok, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	if len(tree[41]) != 0 {
		t.Fatal("delete failed")
	}
}

func TestCharacterUpdate(t *testing.T) {
	store, _ := newTestStore(t)
	info, err := store.UpdateCharacter("Save0001.sav", map[string]any{
		"level":        float64(50),
		"money":        float64(12345),
		"name":         "TestName",
		"skill_points": float64(99),
	})
	if err != nil {
		t.Fatalf("UpdateCharacter: %v", err)
	}
	if info["level"].(uint64) != 50 {
		t.Fatalf("level not applied: %v", info["level"])
	}
	if info["money"].(uint64) != 12345 {
		t.Fatalf("money not applied: %v", info["money"])
	}
	if info["name"].(string) != "TestName" {
		t.Fatalf("name not applied: %v", info["name"])
	}
	// XP must match the required table for level 50
	if info["experience"].(uint64) != uint64(requiredXP[49]) {
		t.Fatalf("xp mismatch: %v", info["experience"])
	}
}

func TestSetSkills(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	skills := extractSkills(tree)
	if len(skills) == 0 {
		t.Skip("fixture has no skills")
	}
	name := ""
	for k := range skills {
		name = k
		break
	}
	count, err := store.SetSkills("Save0001.sav", map[string]int64{name: 5})
	if err != nil || count == 0 {
		t.Fatalf("SetSkills: %v %v", count, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	if extractSkills(tree)[name] != 5 {
		t.Fatal("skill level not applied")
	}
}

func TestUnlockPlaythroughAndActive(t *testing.T) {
	store, _ := newTestStore(t)
	if ok, err := store.UnlockPlaythrough("Save0001.sav", "uvhm"); err != nil || !ok {
		t.Fatalf("UnlockPlaythrough: %v %v", ok, err)
	}
	_, tree, _ := store.ReadSave("Save0001.sav")
	if getUint(tree, 7, 0) != 2 {
		t.Fatalf("playthroughs = %d", getUint(tree, 7, 0))
	}
	if ok, err := store.SetActivePlaythrough("Save0001.sav", 2); err != nil || !ok {
		t.Fatalf("SetActivePlaythrough: %v %v", ok, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	if getUint(tree, 49, 0) != 2 {
		t.Fatal("active playthrough not set")
	}
}

func TestGibbedImportExport(t *testing.T) {
	store, _ := newTestStore(t)
	code := "BL2(hwAAAABL7AFkQAYADoBEDgBQAPA=)" // from golden synthetic item (key 0)
	res, err := store.ImportGibbedCodes("Save0001.sav", code+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if res["imported"].(int) != 1 {
		t.Fatalf("import count: %v (%v)", res["imported"], res["errors"])
	}
	exported, err := store.ExportGibbedCode("Save0001.sav", 54, 0)
	if err != nil || exported == "" {
		t.Fatalf("ExportGibbedCode: %q %v", exported, err)
	}
	sections, err := store.ExportAllCodes("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range sections["weapons"] {
		if c == code {
			found = true
		}
	}
	if !found {
		t.Fatalf("exported codes missing original: %v", sections["weapons"])
	}

	preview := store.PreviewGibbedCodes(code)
	if len(preview) != 1 || preview[0]["error"] != nil {
		t.Fatalf("preview failed: %v", preview)
	}
	if preview[0]["is_weapon"] != true {
		t.Fatal("expected weapon")
	}
}

func TestInvalidFilenameRejected(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	if err := store.WriteSave("../evil.sav", tree, false); err == nil {
		t.Fatal("expected filename rejection")
	}
	if err := store.WriteSave("SaveNNNN.sav", tree, false); err == nil {
		t.Fatal("expected filename rejection")
	}
}

func TestLoadoutSaveRestore(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	code := "BL2(hwAAAABL7AFkQAYADoBEDgBQAPA=)"
	codeBytes, err := bl2save.ValidateGibbedCode(code)
	if err != nil {
		t.Fatal(err)
	}
	rekeyed, _ := bl2save.ReplaceRawItemKey(codeBytes, 12345)
	entryBytes, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1: {{WireType: 2, Value: rekeyed}},
		2: {{WireType: 0, Value: uint64(0)}},
		3: {{WireType: 0, Value: uint64(1)}},
	})
	tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entryBytes})
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatal(err)
	}

	res, err := store.SaveLoadout("Save0001.sav", "My Loadout!")
	if err != nil {
		t.Fatal(err)
	}
	if res["weapons"].(int) != 1 {
		t.Fatalf("loadout weapons: %v", res)
	}
	list, err := store.ListLoadouts()
	if err != nil || len(list) != 1 {
		t.Fatalf("ListLoadouts: %v %v", list, err)
	}
	// Delete the weapon, then restore the loadout
	_, tree, _ = store.ReadSave("Save0001.sav")
	tree[54] = nil
	if err := store.WriteSave("Save0001.sav", tree, false); err != nil {
		t.Fatal(err)
	}
	loadRes, err := store.LoadLoadout("Save0001.sav", res["file"].(string), false)
	if err != nil {
		t.Fatal(err)
	}
	if loadRes["imported"].(int) != 1 {
		t.Fatalf("loadout restore: %v", loadRes)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	if len(tree[54]) != 1 {
		t.Fatal("weapon not restored")
	}
}

func TestMissionsExtractAndComplete(t *testing.T) {
	store, _ := newTestStore(t)
	_, tree, _ := store.ReadSave("Save0001.sav")
	missions := store.ExtractMissions(tree)
	if missions["active_playthrough"] == nil {
		t.Fatal("missing active playthrough")
	}
	count, err := store.CompleteAllMissions("Save0001.sav", 0)
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Log("fixture had no incomplete missions")
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	missions = store.ExtractMissions(tree)
	pts := missions["playthroughs"].([]map[string]any)
	if len(pts) > 0 {
		ms := pts[0]["missions"].([]map[string]any)
		for _, m := range ms {
			if m["status"].(uint64) != 4 {
				t.Fatalf("mission %v not complete", m["name"])
			}
		}
	}
}

func TestFastTravelUpdate(t *testing.T) {
	store, _ := newTestStore(t)
	added, err := store.UnlockAllFastTravel("Save0001.sav")
	if err != nil || added == 0 {
		t.Fatalf("UnlockAllFastTravel: %v %v", added, err)
	}
	_, tree, _ := store.ReadSave("Save0001.sav")
	ft := store.ExtractFastTravel(tree)
	stations := ft["stations"].([]map[string]any)
	if len(stations) < len(BaseStations) {
		t.Fatalf("expected at least %d stations, got %d", len(BaseStations), len(stations))
	}
	if ok, err := store.UpdateFastTravel("Save0001.sav", []string{"NotAStation"}); err == nil || ok {
		t.Fatal("expected unknown station rejection")
	}
	if ok, err := store.UpdateFastTravel("Save0001.sav", []string{"Sanctuary"}); err != nil || !ok {
		t.Fatalf("UpdateFastTravel: %v %v", ok, err)
	}
	_, tree, _ = store.ReadSave("Save0001.sav")
	ft = store.ExtractFastTravel(tree)
	if len(ft["stations"].([]map[string]any)) != 1 {
		t.Fatal("station replacement failed")
	}
}

func TestChallenges(t *testing.T) {
	store, _ := newTestStore(t)
	count, err := store.CompleteAllChallenges("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	if count > 0 {
		_, tree, _ := store.ReadSave("Save0001.sav")
		ch := store.ExtractChallenges(tree)
		cats := ch["categories"].(map[string][]map[string]any)
		for _, list := range cats {
			for _, c := range list {
				if c["completed_count"].(uint64) < 1 {
					t.Fatalf("challenge %v not completed", c["path"])
				}
			}
		}
	}
	reset, err := store.ResetAllChallenges("Save0001.sav")
	if err != nil || reset < count {
		t.Fatalf("reset %v vs complete %v", reset, count)
	}
}

func TestDuplicateSave(t *testing.T) {
	store, dir := newTestStore(t)
	newName, err := store.DuplicateSave("Save0001.sav")
	if err != nil {
		t.Fatal(err)
	}
	if newName != "Save0002.sav" {
		t.Fatalf("unexpected duplicate name: %s", newName)
	}
	if _, err := os.Stat(filepath.Join(dir, newName)); err != nil {
		t.Fatal("duplicate file missing")
	}
	if _, err := store.DuplicateSave("Save9999.sav"); err == nil {
		t.Fatal("expected missing save error")
	}
	if !strings.HasPrefix(newName, "Save") {
		t.Fatal("bad name")
	}
}
