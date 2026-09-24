package bl2save

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// ItemNone marks an absent value in an item value list (Python's None).
const ItemNone = int64(-1)

// ItemStructVersion is the item serial format version.
const ItemStructVersion = 7

// ItemSizes mirrors BaseApp.item_sizes: [0] = non-weapon, [1] = weapon.
var ItemSizes = [2][]int{
	{8, 17, 20, 11, 7, 7, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16},
	{8, 13, 20, 11, 7, 7, 17, 17, 17, 17, 17, 17, 17, 17, 17, 17, 17},
}

// itemHeaderBits mirrors BaseApp.item_header_sizes.
var itemHeaderBits = [2][]int{
	{8, 10, 7},
	{6, 10, 7},
}

// PackItemValues packs item values into the bit-packed serial body, stopping
// at the first ItemNone. Port of save_io.pack_item_values.
func PackItemValues(isWeapon int, values []int64) []byte {
	sizes := ItemSizes[isWeapon]
	itemBytes := make([]byte, 64)
	i := 0
	for idx, v := range values {
		if idx >= len(sizes) {
			break
		}
		if v == ItemNone {
			break
		}
		size := sizes[idx]
		j := i >> 3
		value := uint64(v) << uint(i&7)
		for value != 0 {
			itemBytes[j] |= byte(value & 0xFF)
			value >>= 8
			j++
		}
		i += size
	}
	if i&7 != 0 {
		itemBytes[i>>3] |= byte(0xFF<<uint(i&7)) & 0xFF
	}
	return itemBytes[:(i+7)>>3]
}

// UnpackItemValues unpacks the bit-packed serial body into values, with
// ItemNone for values past the end. Port of save_io.unpack_item_values.
func UnpackItemValues(isWeapon int, data []byte) []int64 {
	sizes := ItemSizes[isWeapon]
	buf := make([]byte, 0, len(data)+1)
	buf = append(buf, 0x20)
	buf = append(buf, data...)
	i := 8
	end := len(buf) * 8
	result := make([]int64, 0, len(sizes))
	for _, size := range sizes {
		j := i + size
		if j > end {
			result = append(result, ItemNone)
			continue
		}
		var value uint64
		start := j >> 3
		if start >= len(buf) {
			start = len(buf) - 1
		}
		for x := start; x >= i>>3; x-- {
			value = value<<8 | uint64(buf[x])
		}
		value >>= uint(i & 7)
		value &= ^(uint64(0xFF) << uint(size))
		result = append(result, int64(value))
		i = j
	}
	return result
}

// UnwrapItem decodes a packed item binary. Port of save_io.unwrap_item.
func UnwrapItem(data []byte) (int, []int64, int64, error) {
	if len(data) < 7 {
		return 0, nil, 0, errors.New("item data too short")
	}
	versionType := data[0]
	key := int64(int32(binary.BigEndian.Uint32(data[1:5])))
	isWeapon := int(versionType >> 7)
	raw := rotateDataRight(xorData(data[5:], key>>5), int(key&31))
	if len(raw) < 2 {
		return 0, nil, 0, errors.New("item data too short")
	}
	return isWeapon, UnpackItemValues(isWeapon, raw[2:]), key, nil
}

// WrapItem encodes item values into a packed item binary. Port of save_io.wrap_item.
func WrapItem(isWeapon int, values []int64, key int64) []byte {
	item := PackItemValues(isWeapon, values)
	header := make([]byte, 5)
	header[0] = byte(isWeapon<<7) | ItemStructVersion
	binary.BigEndian.PutUint32(header[1:5], uint32(key))
	return append(header, createBody(item, header, key)...)
}

// LibAsset is a library/asset pair from an item part reference.
type LibAsset struct {
	Lib   int64 `json:"lib"`
	Asset int64 `json:"asset"`
}

// ItemInfo is the high-level item decode/encode structure.
type ItemInfo struct {
	IsWeapon     int         `json:"is_weapon"`
	Key          int64       `json:"key"`
	Set          int64       `json:"set"`
	Type         LibAsset    `json:"type"`
	Balance      LibAsset    `json:"balance"`
	Manufacturer LibAsset    `json:"manufacturer"`
	Level        [2]int64    `json:"level"`
	Parts        []*LibAsset `json:"parts"`
	Raw          string      `json:"_raw"`
}

// UnwrapItemInfo decodes an item into lib/asset form. Port of save_io.unwrap_item_info.
func UnwrapItemInfo(value []byte) (*ItemInfo, error) {
	isWeapon, item, key, err := UnwrapItem(value)
	if err != nil {
		return nil, err
	}
	info := &ItemInfo{
		IsWeapon: isWeapon,
		Key:      key,
		Set:      item[0],
		Level:    [2]int64{item[4], item[5]},
		Raw:      b64Encode(value),
	}
	bits := itemHeaderBits[isWeapon]
	for i, name := range []string{"type", "balance", "manufacturer"} {
		x := item[1+i]
		if x == ItemNone {
			switch name {
			case "type":
				info.Type = LibAsset{}
			case "balance":
				info.Balance = LibAsset{}
			case "manufacturer":
				info.Manufacturer = LibAsset{}
			}
			continue
		}
		lib := x >> uint(bits[i])
		asset := x & ^(lib << uint(bits[i]))
		la := LibAsset{Lib: lib, Asset: asset}
		switch name {
		case "type":
			info.Type = la
		case "balance":
			info.Balance = la
		case "manufacturer":
			info.Manufacturer = la
		}
	}
	partBits := 10 + isWeapon
	for _, x := range item[6:] {
		if x == ItemNone {
			info.Parts = append(info.Parts, nil)
			continue
		}
		lib := x >> uint(partBits)
		asset := x & ^(lib << uint(partBits))
		info.Parts = append(info.Parts, &LibAsset{Lib: lib, Asset: asset})
	}
	return info, nil
}

// WrapItemInfo encodes lib/asset form back into a packed item. Port of save_io.wrap_item_info.
func WrapItemInfo(info *ItemInfo) []byte {
	parts := []int64{info.Set}
	bits := itemHeaderBits[info.IsWeapon]
	la := []LibAsset{info.Type, info.Balance, info.Manufacturer}
	for i, b := range bits {
		parts = append(parts, (la[i].Lib<<uint(b))|la[i].Asset)
	}
	parts = append(parts, info.Level[0], info.Level[1])
	partBits := 10 + info.IsWeapon
	for _, v := range info.Parts {
		if v == nil {
			parts = append(parts, ItemNone)
		} else {
			parts = append(parts, (v.Lib<<uint(partBits))|v.Asset)
		}
	}
	return WrapItem(info.IsWeapon, parts, info.Key)
}

// IsFakeItem reports whether the values describe a virtual DLC data item.
func IsFakeItem(isWeapon int, itemValues []int64) bool {
	if len(itemValues) == 0 || itemValues[0] != 255 {
		return false
	}
	for _, v := range itemValues[1:] {
		if v != ItemNone && v != 0 {
			return false
		}
	}
	return true
}

func xorData(data []byte, key int64) []byte {
	k := uint64(key) & 0xFFFFFFFF
	out := make([]byte, len(data))
	for idx, c := range data {
		k = (k * 279470273) % 4294967291
		out[idx] = c ^ byte(k&0xFF)
	}
	return out
}

func rotateDataRight(data []byte, steps int) []byte {
	n := len(data)
	if n == 0 {
		return data
	}
	steps = ((steps % n) + n) % n
	out := make([]byte, n)
	copy(out, data[n-steps:])
	copy(out[steps:], data[:n-steps])
	return out
}

func rotateDataLeft(data []byte, steps int) []byte {
	n := len(data)
	if n == 0 {
		return data
	}
	steps = ((steps % n) + n) % n
	out := make([]byte, n)
	copy(out, data[steps:])
	copy(out[n-steps:], data[:steps])
	return out
}

func createBody(item, header []byte, key int64) []byte {
	padding := make([]byte, 33-len(item))
	for i := range padding {
		padding[i] = 0xff
	}
	buf := make([]byte, 0, len(header)+2+len(item)+len(padding))
	buf = append(buf, header...)
	buf = append(buf, 0xff, 0xff)
	buf = append(buf, item...)
	buf = append(buf, padding...)
	h := crc32.ChecksumIEEE(buf)
	checksum := ((h >> 16) ^ h) & 0xFFFF
	cs := []byte{byte(checksum >> 8), byte(checksum)}
	body := rotateDataLeft(append(cs, item...), int(key&31))
	return xorData(body, key>>5)
}

// ReplaceRawItemKey re-encrypts a packed item with a new key.
func ReplaceRawItemKey(data []byte, key int64) ([]byte, error) {
	if len(data) < 7 {
		return nil, errors.New("item data too short")
	}
	oldKey := int64(int32(binary.BigEndian.Uint32(data[1:5])))
	rotated := rotateDataRight(xorData(data[5:], oldKey>>5), int(oldKey&31))
	if len(rotated) < 2 {
		return nil, errors.New("item data too short")
	}
	item := rotated[2:]
	header := make([]byte, 5)
	header[0] = data[0]
	binary.BigEndian.PutUint32(header[1:5], uint32(key))
	return append(header, createBody(item, header, key)...), nil
}
