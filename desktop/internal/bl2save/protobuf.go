package bl2save

import (
	"encoding/binary"
	"errors"
	"sort"
)

// PBEntry is one occurrence of a protobuf field: wire type plus its value.
// Value is uint64 for wire types 0/1/5, []byte for wire type 2, *PBTree for a
// nested message, or []uint64 for a packed/repeated run (written under a
// non-2 wire type, matching the Python implementation).
type PBEntry struct {
	WireType int
	Value    any
}

// PBTree maps field number to its entries in stream order.
type PBTree map[int][]PBEntry

var (
	errProtobufTruncated = errors.New("protobuf data truncated")
	errProtobufWireType  = errors.New("unsupported protobuf wire type")
)

func readVarint(data []byte, pos int) (uint64, int, error) {
	var value uint64
	var offset uint
	for {
		if pos >= len(data) {
			return 0, 0, errProtobufTruncated
		}
		b := data[pos]
		pos++
		value |= uint64(b&0x7F) << offset
		if b&0x80 == 0 {
			break
		}
		offset += 7
		if offset > 63 {
			return 0, 0, errProtobufTruncated
		}
	}
	return value, pos, nil
}

func appendVarint(out []byte, i uint64) []byte {
	for i > 0x7F {
		out = append(out, 0x80|byte(i&0x7F))
		i >>= 7
	}
	return append(out, byte(i))
}

func readProtobufValue(data []byte, pos int, wireType int) (any, int, error) {
	switch wireType {
	case 0:
		return readVarint(data, pos)
	case 1:
		if pos+8 > len(data) {
			return nil, 0, errProtobufTruncated
		}
		return binary.LittleEndian.Uint64(data[pos:]), pos + 8, nil
	case 2:
		length, pos, err := readVarint(data, pos)
		if err != nil {
			return nil, 0, err
		}
		if uint64(len(data)-pos) < length {
			return nil, 0, errProtobufTruncated
		}
		end := pos + int(length)
		value := make([]byte, length)
		copy(value, data[pos:end])
		return value, end, nil
	case 5:
		if pos+4 > len(data) {
			return nil, 0, errProtobufTruncated
		}
		return uint64(binary.LittleEndian.Uint32(data[pos:])), pos + 4, nil
	default:
		return nil, 0, errProtobufWireType
	}
}

// ReadProtobuf parses a protobuf byte stream into an ordered field map,
// mirroring borderlands.datautil.protobuf.read_protobuf.
func ReadProtobuf(data []byte) (PBTree, error) {
	fields := PBTree{}
	pos := 0
	for pos < len(data) {
		key, next, err := readVarint(data, pos)
		if err != nil {
			return nil, err
		}
		pos = next
		fieldNumber := int(key >> 3)
		wireType := int(key & 7)
		value, next, err := readProtobufValue(data, pos, wireType)
		if err != nil {
			return nil, err
		}
		pos = next
		fields[fieldNumber] = append(fields[fieldNumber], PBEntry{WireType: wireType, Value: value})
	}
	return fields, nil
}

func appendProtobufValue(out []byte, wireType int, value any) ([]byte, error) {
	switch wireType {
	case 0:
		v, ok := value.(uint64)
		if !ok {
			return nil, errors.New("expected uint64 for varint field")
		}
		return appendVarint(out, v), nil
	case 1:
		v, ok := value.(uint64)
		if !ok {
			return nil, errors.New("expected uint64 for fixed64 field")
		}
		var buf [8]byte
		binary.LittleEndian.PutUint64(buf[:], v)
		return append(out, buf[:]...), nil
	case 2:
		var b []byte
		switch v := value.(type) {
		case []byte:
			b = v
		case []uint64:
			for _, item := range v {
				out2, err := appendProtobufValue(nil, 0, item)
				if err != nil {
					return nil, err
				}
				b = append(b, out2...)
			}
		default:
			return nil, errors.New("expected bytes for length-delimited field")
		}
		out = appendVarint(out, uint64(len(b)))
		return append(out, b...), nil
	case 5:
		v, ok := value.(uint64)
		if !ok {
			return nil, errors.New("expected uint64 for fixed32 field")
		}
		var buf [4]byte
		binary.LittleEndian.PutUint32(buf[:], uint32(v))
		return append(out, buf[:]...), nil
	default:
		return nil, errProtobufWireType
	}
}

// WriteProtobuf serializes a field map back to protobuf bytes, mirroring
// borderlands.datautil.protobuf.write_protobuf (fields sorted by number).
func WriteProtobuf(tree PBTree) ([]byte, error) {
	out := make([]byte, 0, 256)
	keys := make([]int, 0, len(tree))
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, key := range keys {
		for _, entry := range tree[key] {
			wireType := entry.WireType
			value := entry.Value
			if sub, ok := value.(PBTree); ok {
				subBytes, err := WriteProtobuf(sub)
				if err != nil {
					return nil, err
				}
				value = subBytes
				wireType = 2
			} else if list, ok := value.([]uint64); ok && wireType != 2 {
				packed, err := appendProtobufValue(nil, wireType, list)
				if err != nil {
					return nil, err
				}
				value = packed
				wireType = 2
			}
			out = appendVarint(out, uint64(key<<3|wireType))
			var err error
			out, err = appendProtobufValue(out, wireType, value)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
