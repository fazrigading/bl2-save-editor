package bridge

import (
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"bl2save/desktop/internal/assets"
	"bl2save/desktop/internal/bl2save"
	"bl2save/desktop/internal/editor"
)

// payload helpers -------------------------------------------------------------

func bodyMap(payload any) (map[string]any, *apiError) {
	obj, ok := payload.(map[string]any)
	if !ok {
		return nil, errBad(400, "expected JSON object")
	}
	return obj, nil
}

func pStr(obj map[string]any, key, def string) string {
	if v, ok := obj[key].(string); ok && v != "" {
		return v
	}
	return def
}

func pInt(obj map[string]any, key string, def int) int {
	if v, ok := obj[key].(float64); ok {
		return int(v)
	}
	return def
}

func toFloat(v any) float64 {
	if n, ok := v.(float64); ok {
		return n
	}
	return 0
}

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

// character / skills / ammo ---------------------------------------------------

func (b *Bridge) hUpdateCharacter(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	changes, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	info, err := store.UpdateCharacter(p["filename"], changes)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return info, nil
}

func (b *Bridge) hSetSkills(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	updates := map[string]int64{}
	if raw, ok := obj["skills"].(map[string]any); ok {
		for k, v := range raw {
			updates[k] = int64(toFloat(v))
		}
	}
	count, err := store.SetSkills(p["filename"], updates)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

func (b *Bridge) hSetAmmo(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	updates := map[string]int64{}
	if raw, ok := obj["ammo"].(map[string]any); ok {
		for k, v := range raw {
			updates[k] = int64(toFloat(v))
		}
	}
	ok, err := store.SetAmmo(p["filename"], updates)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": ok}, nil
}

func (b *Bridge) hFillAmmo(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	ok, err := store.FillAllAmmo(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": ok}, nil
}

func (b *Bridge) hPlaythrough(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	filename := p["filename"]
	switch pStr(obj, "action", "") {
	case "unlock_tvhm":
		if _, err := store.UnlockPlaythrough(filename, "tvhm"); err != nil {
			return nil, errBad(500, "%v", err)
		}
	case "unlock_uvhm":
		if _, err := store.UnlockPlaythrough(filename, "uvhm"); err != nil {
			return nil, errBad(500, "%v", err)
		}
	case "set_active":
		pt := pInt(obj, "playthrough", 0)
		if _, err := store.SetActivePlaythrough(filename, pt); err != nil {
			return nil, errBad(500, "%v", err)
		}
	default:
		return nil, errBad(400, "Unknown action")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hUnlockAchievements(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	result, err := store.UnlockAchievements(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	out := map[string]any{"ok": true}
	for k, v := range result {
		out[k] = v
	}
	return out, nil
}

func (b *Bridge) hSpawnTestWeapons(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	res, err := store.SpawnTestWeapons(p["filename"])
	if err != nil {
		return nil, errBad(errStatus(err), "%v", err)
	}
	return res, nil
}

func errStatus(err error) int {
	if strings.Contains(err.Error(), "No weapons") || strings.Contains(err.Error(), "empty") {
		return 400
	}
	return 500
}

// missions --------------------------------------------------------------------

func (b *Bridge) hCompleteAllMissions(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	pt, ae := p.int("pt")
	if ae != nil {
		return nil, ae
	}
	count, err := store.CompleteAllMissions(p["filename"], pt)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

func (b *Bridge) hAddMission(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	mission := pStr(obj, "mission", "")
	status := pInt(obj, "status", 1)
	level := pInt(obj, "level", 1)
	if mission == "" {
		return nil, errBad(400, "Mission name required")
	}
	if status != 1 && status != 4 {
		return nil, errBad(400, "Status must be 1 (Active) or 4 (Complete)")
	}
	if level < 1 || level > 127 {
		return nil, errBad(400, "Level must be 1-127")
	}
	pt, pe := p.int("pt")
	if pe != nil {
		return nil, pe
	}
	ok, err := store.AddMission(p["filename"], pt, mission, uint64(status), level)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(400, "Mission already exists or invalid playthrough")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hRemoveMission(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	pt, pe := p.int("pt")
	if pe != nil {
		return nil, pe
	}
	ok, err := store.RemoveMission(p["filename"], pt, pStr(obj, "mission", ""))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Mission not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hAddAllStory(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	status := 1
	if obj, ok := payload.(map[string]any); ok {
		status = pInt(obj, "status", 1)
	}
	if status != 1 && status != 4 {
		return nil, errBad(400, "Status must be 1 (Active) or 4 (Complete)")
	}
	pt, pe := p.int("pt")
	if pe != nil {
		return nil, pe
	}
	count, err := store.AddAllStoryMissions(p["filename"], pt, uint64(status))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

func (b *Bridge) hSetActiveMission(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	pt, pe := p.int("pt")
	if pe != nil {
		return nil, pe
	}
	ok, err := store.SetActiveMission(p["filename"], pt, pStr(obj, "mission", ""))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Mission not found in this playthrough")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hSetMissionStatus(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	newStatus := pInt(obj, "status", 4)
	if newStatus != 1 && newStatus != 4 {
		return nil, errBad(400, "Status must be 1 (Active) or 4 (Complete)")
	}
	pt, pe := p.int("pt")
	if pe != nil {
		return nil, pe
	}
	ok, err := store.SetMissionStatus(p["filename"], pt, p["mission"], uint64(newStatus))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Mission not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hMissionDB(_ params, _ any, _ url.Values) (any, *apiError) {
	return editor.MissionDBJSON(), nil
}

// fast travel -----------------------------------------------------------------

func (b *Bridge) hUpdateFastTravel(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	raw, ok := obj["stations"].([]any)
	if !ok {
		return nil, errBad(400, "stations must be a list")
	}
	stations := make([]string, 0, len(raw))
	for _, s := range raw {
		if str, ok := s.(string); ok {
			stations = append(stations, str)
		}
	}
	if ok, err := store.UpdateFastTravel(p["filename"], stations); err != nil {
		var unknown editor.UnknownStationError
		if errors.As(err, &unknown) {
			return nil, errBad(400, "%v", err)
		}
		return nil, errBad(500, "%v", err)
	} else if !ok {
		return nil, errBad(500, "update failed")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hUnlockAllFastTravel(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	count, err := store.UnlockAllFastTravel(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "added": count}, nil
}

func (b *Bridge) hAllStations(_ params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	out := []map[string]any{}
	for _, s := range store.AllStations() {
		out = append(out, map[string]any{"name": s, "display_name": store.StationDisplay(s)})
	}
	return out, nil
}

// challenges ------------------------------------------------------------------

func (b *Bridge) hGetChallenges(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	_, tree, err := store.ReadSave(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return store.ExtractChallenges(tree), nil
}

func (b *Bridge) hSetChallenge(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	progress := pInt(obj, "progress", 0)
	completed := pInt(obj, "completed", 0)
	if progress < 0 || progress > 999999 {
		return nil, errBad(400, "Progress must be 0-999999")
	}
	if completed < 0 || completed > 999 {
		return nil, errBad(400, "Completed count must be 0-999")
	}
	ok, err := store.SetChallengeProgress(p["filename"], pStr(obj, "path", ""), int64(progress), int64(completed))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Challenge not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hCompleteAllChallenges(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	count, err := store.CompleteAllChallenges(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

func (b *Bridge) hResetAllChallenges(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	count, err := store.ResetAllChallenges(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

// items -----------------------------------------------------------------------

func (b *Bridge) hDeleteItem(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	ok, err := store.DeleteItem(p["filename"], field, idx)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Item not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hReorderItem(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	ok, err := store.ReorderItem(p["filename"], field, pInt(obj, "from", -1), pInt(obj, "to", -1))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(400, "Invalid indices")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hSetItemLevel(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	ok, err := store.SetItemLevel(p["filename"], field, idx, pInt(obj, "level", 0))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Item not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hDuplicateItem(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	ok, err := store.DuplicateItem(p["filename"], field, idx)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Item not found")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hBulkLevel(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	count, err := store.BulkSetLevel(p["filename"], field, pInt(obj, "level", 0))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return map[string]any{"ok": true, "count": count}, nil
}

func (b *Bridge) hTransferItem(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid source field: %d", field)
	}
	toField := pInt(obj, "to_field", 0)
	if !validItemFields[toField] {
		return nil, errBad(400, "Invalid target field: %d", toField)
	}
	ok, err := store.TransferItem(p["filename"], field, idx, toField)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(400, "Transfer failed")
	}
	return map[string]any{"ok": true}, nil
}

func (b *Bridge) hAddWeapon(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	values, ae := buildWeaponValues(b.adb, obj)
	if ae != nil {
		return nil, ae
	}
	if ok, err := store.AddWeapon(p["filename"], values); err != nil {
		return nil, errBad(500, "%v", err)
	} else if !ok {
		return nil, errBad(500, "add failed")
	}
	return map[string]any{"ok": true}, nil
}

func buildWeaponValues(db *assets.DB, obj map[string]any) ([]int64, *apiError) {
	balancePath := pStr(obj, "balance", "")
	level := pInt(obj, "level", 1)
	if level < 0 {
		level = 0
	}
	if level > 127 {
		level = 127
	}
	selectedParts, _ := obj["parts"].(map[string]any)

	wtypePath := db.GetWeaponTypeForBalance(balancePath)
	mfrPath := db.GetManufacturerForBalance(balancePath)
	if wtypePath == "" || mfrPath == "" {
		return nil, errBad(400, "Cannot resolve weapon type or manufacturer")
	}
	typeLib, typeAsset, okType := db.PackReference(wtypePath, "WeaponTypes", 0)
	balLib, balAsset, okBal := db.PackReference(balancePath, "BalanceDefs", 0)
	mfrLib, mfrAsset, okMfr := db.PackReference(mfrPath, "Manufacturers", 0)
	if !okType || !okBal || !okMfr {
		return nil, errBad(400, "Cannot resolve asset references")
	}
	setID := db.GetSetIDForPath(balancePath, "BalanceDefs")

	typePacked := typeLib<<6 | typeAsset
	balPacked := balLib<<10 | balAsset
	mfrPacked := mfrLib<<7 | mfrAsset

	const none17 = (1 << 17) - 1
	slotOrder := []string{"body", "grip", "barrel", "sight", "stock", "elemental", "accessory1", "accessory2", "material", "prefix", "title"}
	values := []int64{int64(setID), int64(typePacked), int64(balPacked), int64(mfrPacked), int64(level), int64(level)}
	for _, slot := range slotOrder {
		partPath := pStr(selectedParts, slot, "")
		if partPath != "" {
			if lib, asset, ok := db.PackReference(partPath, "WeaponParts", 0); ok {
				values = append(values, int64(lib<<11|asset))
				continue
			}
		}
		values = append(values, none17)
	}
	return values, nil
}

func (b *Bridge) hAddItem(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	values, ae := buildItemValues(b.adb, obj)
	if ae != nil {
		return nil, ae
	}
	if ok, err := store.AddItem(p["filename"], values); err != nil {
		return nil, errBad(500, "%v", err)
	} else if !ok {
		return nil, errBad(500, "add failed")
	}
	return map[string]any{"ok": true}, nil
}

func buildItemValues(db *assets.DB, obj map[string]any) ([]int64, *apiError) {
	balancePath := pStr(obj, "balance", "")
	level := pInt(obj, "level", 1)
	if level < 0 {
		level = 0
	}
	if level > 127 {
		level = 127
	}
	selectedParts, _ := obj["parts"].(map[string]any)

	itemTypePath := db.GetItemTypeForBalance(balancePath)
	if itemTypePath == "" {
		return nil, errBad(400, "Cannot resolve item type")
	}
	mfrPath := db.GetManufacturerForItemBalance(balancePath)

	typeLib, typeAsset, okType := db.PackReference(itemTypePath, "ItemTypes", 0)
	balLib, balAsset, okBal := db.PackReference(balancePath, "BalanceDefs", 0)
	if !okType || !okBal {
		return nil, errBad(400, "Cannot resolve asset references")
	}
	typePacked := typeLib<<8 | typeAsset
	balPacked := balLib<<10 | balAsset
	mfrPacked := 0
	if mfrPath != "" {
		if lib, asset, ok := db.PackReference(mfrPath, "Manufacturers", 0); ok {
			mfrPacked = lib<<7 | asset
		}
	}
	setID := db.GetSetIDForPath(balancePath, "BalanceDefs")

	const none16 = (1 << 16) - 1
	slotOrder := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "material", "prefix", "title"}
	values := []int64{int64(setID), int64(typePacked), int64(balPacked), int64(mfrPacked), int64(level), int64(level)}
	for _, slot := range slotOrder {
		partPath := pStr(selectedParts, slot, "")
		if partPath != "" {
			if lib, asset, ok := db.PackReference(partPath, "ItemParts", 0); ok {
				values = append(values, int64(lib<<10|asset))
				continue
			}
		}
		values = append(values, none16)
	}
	return values, nil
}

func (b *Bridge) hEditItem(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	if ok, err := store.EditItemValues(p["filename"], field, idx, obj, b.adb); err != nil {
		return nil, errBad(errStatus(err), "%v", err)
	} else if !ok {
		return nil, errBad(404, "Item not found")
	}
	return map[string]any{"ok": true}, nil
}

// gibbed codes ----------------------------------------------------------------

func (b *Bridge) hPreviewCodes(_ params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	results := store.PreviewGibbedCodes(pStr(obj, "codes", ""))
	for _, r := range results {
		if info, ok := r["info"]; ok {
			if m, ok := infoToMap(info); ok {
				r["resolved"] = b.adb.ResolveItemParts(m)
			}
		}
	}
	return results, nil
}

func infoToMap(v any) (map[string]any, bool) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, false
	}
	return m, true
}

func (b *Bridge) hImportCodes(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	res, err := store.ImportGibbedCodes(p["filename"], pStr(obj, "codes", ""))
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return res, nil
}

func (b *Bridge) hExportCode(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	field, ae := p.int("field")
	if ae != nil {
		return nil, ae
	}
	idx, ae := p.int("idx")
	if ae != nil {
		return nil, ae
	}
	if !validItemFields[field] {
		return nil, errBad(400, "Invalid item field: %d", field)
	}
	code, err := store.ExportGibbedCode(p["filename"], field, idx)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if code == "" {
		return nil, errBad(404, "Item not found")
	}
	return map[string]any{"code": code}, nil
}

func (b *Bridge) hExportAll(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	sections, err := store.ExportAllCodes(p["filename"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return sections, nil
}

// assets ----------------------------------------------------------------------

func (b *Bridge) hWeaponTypes(_ params, _ any, _ url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetWeaponCategories(), nil
}

func (b *Bridge) hBalances(_ params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetBalancesForType(q.Get("type")), nil
}

func (b *Bridge) hParts(_ params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetPartsForBalance(q.Get("balance")), nil
}

func (b *Bridge) hItemCategories(_ params, _ any, _ url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetItemCategories(), nil
}

func (b *Bridge) hItemBalances(_ params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetItemBalancesForCategory(q.Get("category")), nil
}

func (b *Bridge) hItemParts(_ params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetItemPartsForBalance(q.Get("balance")), nil
}

func (b *Bridge) hAllPartsForSlot(p params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	if q.Get("kind") == "item" {
		return b.adb.GetAllItemPartsForSlot(p["slot"]), nil
	}
	return b.adb.GetAllPartsForSlot(p["slot"]), nil
}

func (b *Bridge) hAllPartsBatch(_ params, _ any, q url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	kind := q.Get("kind")
	slotsStr := q.Get("slots")
	if slotsStr == "" {
		return nil, errBad(400, "slots required")
	}
	result := map[string]any{}
	for _, slot := range strings.Split(slotsStr, ",") {
		slot = strings.TrimSpace(slot)
		if slot == "" {
			continue
		}
		if kind == "item" {
			result[slot] = b.adb.GetAllItemPartsForSlot(slot)
		} else {
			result[slot] = b.adb.GetAllPartsForSlot(slot)
		}
	}
	return result, nil
}

func (b *Bridge) hAllBalances(_ params, _ any, _ url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetAllBalances(), nil
}

func (b *Bridge) hManufacturers(_ params, _ any, _ url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.GetAllManufacturers(), nil
}

func (b *Bridge) hCustomizations(p params, _ any, _ url.Values) (any, *apiError) {
	b.adb.EnsureLoaded()
	return b.adb.Customizations(p["class"]), nil
}

// loadouts --------------------------------------------------------------------

var loadoutFileRe = regexp.MustCompile(`^[A-Za-z0-9_\-. ]+\.json$`)

func (b *Bridge) hListLoadouts(_ params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	list, err := store.ListLoadouts()
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return list, nil
}

func (b *Bridge) hSaveLoadout(_ params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	filename := pStr(obj, "filename", "")
	name := strings.TrimSpace(pStr(obj, "name", ""))
	if filename == "" || name == "" {
		return nil, errBad(400, "filename and name required")
	}
	if !validSaveName(filename) {
		return nil, errBad(400, "Invalid save filename")
	}
	res, err := store.SaveLoadout(filename, name)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return res, nil
}

func (b *Bridge) hRestoreLoadout(p params, payload any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	obj, ae := bodyMap(payload)
	if ae != nil {
		return nil, ae
	}
	filename := pStr(obj, "filename", "")
	if filename == "" {
		return nil, errBad(400, "filename required")
	}
	if !validSaveName(filename) {
		return nil, errBad(400, "Invalid save filename")
	}
	if !loadoutFileRe.MatchString(p["file"]) {
		return nil, errBad(400, "Invalid loadout file")
	}
	replace := false
	if v, ok := obj["replace"].(bool); ok {
		replace = v
	}
	res, err := store.LoadLoadout(filename, p["file"], replace)
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	return res, nil
}

func (b *Bridge) hDeleteLoadout(p params, _ any, _ url.Values) (any, *apiError) {
	store, e := b.requireStore()
	if e != nil {
		return nil, e
	}
	if !loadoutFileRe.MatchString(p["file"]) {
		return nil, errBad(400, "Invalid loadout file")
	}
	ok, err := store.DeleteLoadout(p["file"])
	if err != nil {
		return nil, errBad(500, "%v", err)
	}
	if !ok {
		return nil, errBad(404, "Loadout not found")
	}
	return map[string]any{"ok": true}, nil
}

// steam (save-state mode) ------------------------------------------------------

func (b *Bridge) hSteamStatus(_ params, _ any, _ url.Values) (any, *apiError) {
	return map[string]any{
		"available":   true,
		"initialized": true,
		"healthy":     true,
		"mode":        "save-state",
		"message":     "Achievements are written to the save file",
	}, nil
}

func (b *Bridge) hSteamInit(_ params, _ any, _ url.Values) (any, *apiError) {
	return map[string]any{
		"ok":      true,
		"message": "Save-state mode: achievements are written to the save file",
	}, nil
}

func (b *Bridge) hSteamAchievements(_ params, _ any, _ url.Values) (any, *apiError) {
	out := []map[string]any{}
	for _, a := range editor.AchievementList() {
		out = append(out, map[string]any{
			"api_name":      a.APIName,
			"name":          a.Name,
			"description":   a.Desc,
			"category":      a.Category,
			"category_name": categoryDisplay(a.Category),
			"unlocked":      nil,
		})
	}
	return out, nil
}

func categoryDisplay(cat string) string {
	names := map[string]string{
		"story": "Story", "exploration": "Exploration", "combat": "Combat",
		"challenge": "Challenge", "misc": "Miscellaneous",
		"scarlett": "Captain Scarlett", "torgue": "Mr. Torgue",
		"hammerlock": "Sir Hammerlock", "tina": "Tiny Tina",
		"dlc": "Fight for Sanctuary",
	}
	if n, ok := names[cat]; ok {
		return n
	}
	return cat
}

func (b *Bridge) hSteamUnlock(_ params, payload any, _ url.Values) (any, *apiError) {
	return nil, errBad(400, "Save-state mode: use 'Unlock All' to write achievement state into the loaded save")
}

func (b *Bridge) hSteamUnlockAll(_ params, _ any, _ url.Values) (any, *apiError) {
	return nil, errBad(400, "Save-state mode: use the MAX CHARACTER button on a loaded save")
}

func (b *Bridge) hSteamClear(_ params, payload any, _ url.Values) (any, *apiError) {
	return nil, errBad(400, "Save-state mode: achievements cannot be re-locked from the save")
}
