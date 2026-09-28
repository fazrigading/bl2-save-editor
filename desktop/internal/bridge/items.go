package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"bl2save/desktop/internal/assets"
)

// Items covers inventory mutations (weapons/items/bank), Gibbed code
// import/export and loadouts. Mutations return the fresh save state merged
// into their result.
type Items struct{ b *Bridge }

var validItemFields = map[int]bool{41: true, 53: true, 54: true}

var loadoutFileRe = regexp.MustCompile(`^[A-Za-z0-9_\-. ]+\.json$`)

func checkItemField(field int) error {
	if !validItemFields[field] {
		return fmt.Errorf("Invalid item field: %d", field)
	}
	return nil
}

func (s *Items) AddWeapon(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	values, err := buildWeaponValues(s.b.adb, payload)
	if err != nil {
		return nil, err
	}
	if ok, err := store.AddWeapon(filename, values); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("add failed")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func buildWeaponValues(db *assets.DB, obj map[string]any) ([]int64, error) {
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
		return nil, errors.New("Cannot resolve weapon type or manufacturer")
	}
	typeLib, typeAsset, okType := db.PackReference(wtypePath, "WeaponTypes", 0)
	balLib, balAsset, okBal := db.PackReference(balancePath, "BalanceDefs", 0)
	mfrLib, mfrAsset, okMfr := db.PackReference(mfrPath, "Manufacturers", 0)
	if !okType || !okBal || !okMfr {
		return nil, errors.New("Cannot resolve asset references")
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

func (s *Items) AddItem(filename string, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	values, err := buildItemValues(s.b.adb, payload)
	if err != nil {
		return nil, err
	}
	if ok, err := store.AddItem(filename, values); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("add failed")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func buildItemValues(db *assets.DB, obj map[string]any) ([]int64, error) {
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
		return nil, errors.New("Cannot resolve item type")
	}
	mfrPath := db.GetManufacturerForItemBalance(balancePath)

	typeLib, typeAsset, okType := db.PackReference(itemTypePath, "ItemTypes", 0)
	balLib, balAsset, okBal := db.PackReference(balancePath, "BalanceDefs", 0)
	if !okType || !okBal {
		return nil, errors.New("Cannot resolve asset references")
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

func (s *Items) ReorderItem(filename string, field int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	ok, err := store.ReorderItem(filename, field, pInt(payload, "from", -1), pInt(payload, "to", -1))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Invalid indices")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Items) BulkLevel(filename string, field int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	count, err := store.BulkSetLevel(filename, field, pInt(payload, "level", 0))
	if err != nil {
		return nil, err
	}
	return s.b.withState(map[string]any{"ok": true, "count": count}, filename)
}

func (s *Items) DeleteItem(filename string, field, idx int) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	ok, err := store.DeleteItem(filename, field, idx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Item not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Items) SetItemLevel(filename string, field, idx int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	ok, err := store.SetItemLevel(filename, field, idx, pInt(payload, "level", 0))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Item not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Items) DuplicateItem(filename string, field, idx int) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	ok, err := store.DuplicateItem(filename, field, idx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Item not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Items) TransferItem(filename string, field, idx int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, fmt.Errorf("Invalid source field: %d", field)
	}
	toField := pInt(payload, "to_field", 0)
	if err := checkItemField(toField); err != nil {
		return nil, fmt.Errorf("Invalid target field: %d", toField)
	}
	ok, err := store.TransferItem(filename, field, idx, toField)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Transfer failed")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

func (s *Items) EditItem(filename string, field, idx int, payload map[string]any) (map[string]any, error) {
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	if ok, err := store.EditItemValues(filename, field, idx, payload, s.b.adb); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("Item not found")
	}
	return s.b.withState(map[string]any{"ok": true}, filename)
}

// gibbed codes ----------------------------------------------------------------

func (s *Items) PreviewCodes(payload map[string]any) (any, error) {
	defer slowLog("Items.PreviewCodes", time.Now())
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	results := store.PreviewGibbedCodes(pStr(payload, "codes", ""))
	for _, r := range results {
		if info, ok := r["info"]; ok {
			if m, ok := infoToMap(info); ok {
				r["resolved"] = s.b.adb.ResolveItemParts(m)
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

func (s *Items) ImportCodes(filename string, payload map[string]any) (map[string]any, error) {
	defer slowLog("Items.ImportCodes", time.Now())
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	res, err := store.ImportGibbedCodes(filename, pStr(payload, "codes", ""))
	if err != nil {
		return nil, err
	}
	return s.b.withState(res, filename)
}

func (s *Items) ExportCode(filename string, field, idx int) (map[string]any, error) {
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	if err := checkItemField(field); err != nil {
		return nil, err
	}
	code, err := store.ExportGibbedCode(filename, field, idx)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, errors.New("Item not found")
	}
	return map[string]any{"code": code}, nil
}

func (s *Items) ExportAll(filename string) (any, error) {
	defer slowLog("Items.ExportAll", time.Now())
	store, err := s.b.deps(filename)
	if err != nil {
		return nil, err
	}
	return store.ExportAllCodes(filename)
}

// loadouts --------------------------------------------------------------------

func (s *Items) ListLoadouts() (any, error) {
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	return store.ListLoadouts()
}

func (s *Items) SaveLoadout(payload map[string]any) (any, error) {
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	filename := pStr(payload, "filename", "")
	name := strings.TrimSpace(pStr(payload, "name", ""))
	if filename == "" || name == "" {
		return nil, errors.New("filename and name required")
	}
	if err := requireSaveName(filename); err != nil {
		return nil, err
	}
	return store.SaveLoadout(filename, name)
}

func (s *Items) RestoreLoadout(filename string, file string, payload map[string]any) (map[string]any, error) {
	defer slowLog("Items.RestoreLoadout", time.Now())
	store, err := s.b.writeDeps(filename)
	if err != nil {
		return nil, err
	}
	if err := requireSaveName(filename); err != nil {
		return nil, err
	}
	if !loadoutFileRe.MatchString(file) {
		return nil, errors.New("Invalid loadout file")
	}
	replace := false
	if v, ok := payload["replace"].(bool); ok {
		replace = v
	}
	res, err := store.LoadLoadout(filename, file, replace)
	if err != nil {
		return nil, err
	}
	return s.b.withState(res, filename)
}

func (s *Items) DeleteLoadout(file string) (map[string]any, error) {
	store, err := s.b.requireStore()
	if err != nil {
		return nil, err
	}
	if !loadoutFileRe.MatchString(file) {
		return nil, errors.New("Invalid loadout file")
	}
	ok, err := store.DeleteLoadout(file)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("Loadout not found")
	}
	return map[string]any{"ok": true}, nil
}
