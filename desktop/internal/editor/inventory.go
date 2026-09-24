package editor

import (
	"strings"

	"bl2save/desktop/internal/bl2save"
)

// ExtractInventory decodes all items with lib/asset info.
func (s *Store) ExtractInventory(tree bl2save.PBTree) map[string]any {
	result := map[string]any{"weapons": []any{}, "items": []any{}, "bank": []any{}}
	for _, section := range itemSections {
		items := []map[string]any{}
		for idx, entry := range tree[section.Field] {
			sub, ok := subTree(entry)
			if !ok || len(sub[1]) == 0 {
				continue
			}
			rawItem, ok := sub[1][0].Value.([]byte)
			if !ok {
				continue
			}
			isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
			if err != nil || bl2save.IsFakeItem(isWeapon, values) {
				continue
			}
			info, err := bl2save.UnwrapItemInfo(rawItem)
			if err != nil {
				continue
			}
			item := map[string]any{
				"index": idx,
				"field": section.Field,
				"info":  info,
			}
			switch section.Field {
			case 54:
				item["slot"] = getUint(sub, 2, 0)
				item["star"] = getUint(sub, 3, 0)
			case 53:
				item["is_equipped"] = getUint(sub, 3, 0)
				item["star"] = getUint(sub, 4, 0)
			}
			items = append(items, item)
		}
		result[section.Category] = items
	}
	return result
}

// DeleteItem removes an item from a section.
func (s *Store) DeleteItem(filename string, field, index int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if index < 0 || index >= len(tree[field]) {
		return false, nil
	}
	tree[field] = append(tree[field][:index], tree[field][index+1:]...)
	return true, s.WriteSave(filename, tree, false)
}

// TransferItem moves an item between sections, rejecting fake items.
func (s *Store) TransferItem(filename string, fromField, index, toField int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if index < 0 || index >= len(tree[fromField]) || fromField == toField {
		return false, nil
	}
	entry := tree[fromField][index]
	if sub, ok := subTree(entry); ok && len(sub[1]) > 0 {
		if rawItem, ok := sub[1][0].Value.([]byte); ok {
			if isWeapon, values, _, err := bl2save.UnwrapItem(rawItem); err == nil && bl2save.IsFakeItem(isWeapon, values) {
				return false, nil
			}
		}
	}
	tree[fromField] = append(tree[fromField][:index], tree[fromField][index+1:]...)
	tree[toField] = append(tree[toField], entry)
	return true, s.WriteSave(filename, tree, false)
}

// ReorderItem moves an item within a section.
func (s *Store) ReorderItem(filename string, field, fromIndex, toIndex int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	items := tree[field]
	if fromIndex < 0 || fromIndex >= len(items) || toIndex < 0 || toIndex >= len(items) {
		return false, nil
	}
	if fromIndex == toIndex {
		return true, nil
	}
	item := items[fromIndex]
	items = append(items[:fromIndex], items[fromIndex+1:]...)
	items = append(items[:toIndex], append([]bl2save.PBEntry{item}, items[toIndex:]...)...)
	tree[field] = items
	return true, s.WriteSave(filename, tree, false)
}

// SetItemLevel rewrites grade_index and game_stage for one item.
func (s *Store) SetItemLevel(filename string, field, index, newLevel int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if index < 0 || index >= len(tree[field]) {
		return false, nil
	}
	entry := tree[field][index]
	sub, ok := subTree(entry)
	if !ok || len(sub[1]) == 0 {
		return false, nil
	}
	rawItem, ok := sub[1][0].Value.([]byte)
	if !ok {
		return false, nil
	}
	isWeapon, values, key, err := bl2save.UnwrapItem(rawItem)
	if err != nil || bl2save.IsFakeItem(isWeapon, values) {
		return false, nil
	}
	if newLevel < 0 {
		newLevel = 0
	}
	if newLevel > 127 {
		newLevel = 127
	}
	values[4] = int64(newLevel)
	values[5] = int64(newLevel)
	sub[1][0].Value = bl2save.WrapItem(isWeapon, values, key)
	entry.Value, _ = bl2save.WriteProtobuf(sub)
	tree[field][index] = entry
	return true, s.WriteSave(filename, tree, false)
}

// DuplicateItem clones an item within its section with a fresh key.
func (s *Store) DuplicateItem(filename string, field, index int) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if index < 0 || index >= len(tree[field]) {
		return false, nil
	}
	orig := tree[field][index]
	subBytes, ok := orig.Value.([]byte)
	if !ok {
		return false, nil
	}
	cloneBytes := append([]byte(nil), subBytes...)
	sub, err := bl2save.ReadProtobuf(cloneBytes)
	if err == nil && len(sub[1]) > 0 {
		if rawItem, ok := sub[1][0].Value.([]byte); ok {
			if isWeapon, values, _, err := bl2save.UnwrapItem(rawItem); err == nil {
				sub[1][0].Value = bl2save.WrapItem(isWeapon, values, randomKey())
				if nb, err := bl2save.WriteProtobuf(sub); err == nil {
					cloneBytes = nb
				}
			}
		}
	}
	tree[field] = append(tree[field], bl2save.PBEntry{WireType: 2, Value: cloneBytes})
	return true, s.WriteSave(filename, tree, false)
}

// BulkSetLevel sets every item in a section to the given level.
func (s *Store) BulkSetLevel(filename string, field, newLevel int) (int, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return 0, err
	}
	if newLevel < 0 {
		newLevel = 0
	}
	if newLevel > 127 {
		newLevel = 127
	}
	count := 0
	for idx, entry := range tree[field] {
		sub, ok := subTree(entry)
		if !ok || len(sub[1]) == 0 {
			continue
		}
		rawItem, ok := sub[1][0].Value.([]byte)
		if !ok {
			continue
		}
		isWeapon, values, key, err := bl2save.UnwrapItem(rawItem)
		if err != nil || bl2save.IsFakeItem(isWeapon, values) {
			continue
		}
		values[4] = int64(newLevel)
		values[5] = int64(newLevel)
		sub[1][0].Value = bl2save.WrapItem(isWeapon, values, key)
		entry.Value, _ = bl2save.WriteProtobuf(sub)
		tree[field][idx] = entry
		count++
	}
	if count > 0 {
		return count, s.WriteSave(filename, tree, false)
	}
	return 0, nil
}

// AddWeapon appends a weapon from a packed values list.
func (s *Store) AddWeapon(filename string, valuesList []int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	raw := bl2save.WrapItem(1, valuesList, randomKey())
	entry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1: {{WireType: 2, Value: raw}},
		2: {{WireType: 0, Value: uint64(0)}},
		3: {{WireType: 0, Value: uint64(1)}},
	})
	tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entry})
	return true, s.WriteSave(filename, tree, false)
}

// AddItem appends a non-weapon item from a packed values list.
func (s *Store) AddItem(filename string, valuesList []int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	raw := bl2save.WrapItem(0, valuesList, randomKey())
	entry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
		1: {{WireType: 2, Value: raw}},
		2: {{WireType: 0, Value: uint64(1)}},
		3: {{WireType: 0, Value: uint64(0)}},
		4: {{WireType: 0, Value: uint64(1)}},
	})
	tree[53] = append(tree[53], bl2save.PBEntry{WireType: 2, Value: entry})
	return true, s.WriteSave(filename, tree, true)
}

// EditItem replaces an item's packed values in place (full item editor).
func (s *Store) EditItem(filename string, field, index int, valuesList []int64, newKey *int64) (bool, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return false, err
	}
	if index < 0 || index >= len(tree[field]) {
		return false, nil
	}
	entry := tree[field][index]
	sub, ok := subTree(entry)
	if !ok || len(sub[1]) == 0 {
		return false, nil
	}
	rawItem, ok := sub[1][0].Value.([]byte)
	if !ok {
		return false, nil
	}
	isWeapon, values, key, err := bl2save.UnwrapItem(rawItem)
	if err != nil || bl2save.IsFakeItem(isWeapon, values) {
		return false, nil
	}
	_ = values
	if newKey != nil {
		key = *newKey
	}
	sub[1][0].Value = bl2save.WrapItem(isWeapon, valuesList, key)
	entry.Value, _ = bl2save.WriteProtobuf(sub)
	tree[field][index] = entry
	return true, s.WriteSave(filename, tree, false)
}

// ImportGibbedCodes validates and appends BL2() codes to the save.
func (s *Store) ImportGibbedCodes(filename, codesText string) (map[string]any, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	count := 0
	var errMsgs []string
	for lineNum, line := range strings.Split(strings.TrimSpace(codesText), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "BL2(") {
			continue
		}
		codeBytes, err := bl2save.ValidateGibbedCode(line)
		if err != nil {
			errMsgs = append(errMsgs, itoa(lineNum+1)+": "+err.Error())
			continue
		}
		key := randomKey()
		codeBytes, err = bl2save.ReplaceRawItemKey(codeBytes, key)
		if err != nil {
			errMsgs = append(errMsgs, itoa(lineNum+1)+": "+err.Error())
			continue
		}
		if codeBytes[0]&0x80 == 0 {
			entry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
				1: {{WireType: 2, Value: codeBytes}},
				2: {{WireType: 0, Value: uint64(1)}},
				3: {{WireType: 0, Value: uint64(0)}},
				4: {{WireType: 0, Value: uint64(1)}},
			})
			tree[53] = append(tree[53], bl2save.PBEntry{WireType: 2, Value: entry})
		} else {
			entry, _ := bl2save.WriteProtobuf(bl2save.PBTree{
				1: {{WireType: 2, Value: codeBytes}},
				2: {{WireType: 0, Value: uint64(0)}},
				3: {{WireType: 0, Value: uint64(1)}},
			})
			tree[54] = append(tree[54], bl2save.PBEntry{WireType: 2, Value: entry})
		}
		count++
	}
	if count > 0 {
		if err := s.WriteSave(filename, tree, false); err != nil {
			return nil, err
		}
	}
	return map[string]any{"imported": count, "errors": errMsgs}, nil
}

// PreviewGibbedCodes decodes codes without importing.
func (s *Store) PreviewGibbedCodes(codesText string) []map[string]any {
	results := []map[string]any{}
	for lineNum, line := range strings.Split(strings.TrimSpace(codesText), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "BL2(") {
			continue
		}
		codeBytes, err := bl2save.ValidateGibbedCode(line)
		if err != nil {
			results = append(results, map[string]any{"line": lineNum + 1, "error": err.Error()})
			continue
		}
		info, err := bl2save.UnwrapItemInfo(codeBytes)
		if err != nil {
			results = append(results, map[string]any{"line": lineNum + 1, "error": err.Error()})
			continue
		}
		results = append(results, map[string]any{
			"line":      lineNum + 1,
			"info":      info,
			"is_weapon": codeBytes[0]&0x80 != 0,
			"code":      line,
		})
	}
	return results
}

// ExportGibbedCode exports one item with a zeroed key.
func (s *Store) ExportGibbedCode(filename string, field, index int) (string, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return "", err
	}
	if index < 0 || index >= len(tree[field]) {
		return "", nil
	}
	sub, ok := subTree(tree[field][index])
	if !ok || len(sub[1]) == 0 {
		return "", nil
	}
	rawItem, ok := sub[1][0].Value.([]byte)
	if !ok {
		return "", nil
	}
	isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
	if err != nil || bl2save.IsFakeItem(isWeapon, values) {
		return "", nil
	}
	zeroed, err := bl2save.ReplaceRawItemKey(rawItem, 0)
	if err != nil {
		return "", err
	}
	return bl2save.EncodeGibbedCode(zeroed), nil
}

// ExportAllCodes exports every non-fake item, grouped by section.
func (s *Store) ExportAllCodes(filename string) (map[string][]string, error) {
	_, tree, err := s.ReadSave(filename)
	if err != nil {
		return nil, err
	}
	sections := map[string][]string{"weapons": {}, "items": {}, "bank": {}}
	for _, section := range itemSections {
		for _, entry := range tree[section.Field] {
			sub, ok := subTree(entry)
			if !ok || len(sub[1]) == 0 {
				continue
			}
			rawItem, ok := sub[1][0].Value.([]byte)
			if !ok {
				continue
			}
			isWeapon, values, _, err := bl2save.UnwrapItem(rawItem)
			if err != nil || bl2save.IsFakeItem(isWeapon, values) {
				continue
			}
			zeroed, err := bl2save.ReplaceRawItemKey(rawItem, 0)
			if err != nil {
				continue
			}
			sections[section.Category] = append(sections[section.Category], bl2save.EncodeGibbedCode(zeroed))
		}
	}
	return sections, nil
}
