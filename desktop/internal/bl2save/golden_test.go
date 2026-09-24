package bl2save

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"os"
	"reflect"
	"testing"
)

type goldenFixture struct {
	ProtobufSHA    string              `json:"protobuf_sha256"`
	ContainerSHA   string              `json:"container_sha256"`
	Tree           json.RawMessage     `json:"tree"`
	GibbedCodes    map[string][]string `json:"gibbed_codes"`
	SyntheticItems []goldenItem        `json:"synthetic_items"`
}

type goldenItem struct {
	IsWeapon int               `json:"is_weapon"`
	Values   []json.RawMessage `json:"values"`
	Key      int64             `json:"key"`
	RawB64   string            `json:"raw_b64"`
	Code     string            `json:"code"`
	Info     map[string]any    `json:"info"`
}

func loadGolden(t *testing.T) *goldenFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/Save0001.golden.json")
	if err != nil {
		t.Fatalf("load golden: %v", err)
	}
	var f goldenFixture
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("parse golden: %v", err)
	}
	return &f
}

func loadSaveBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/Save0001.sav")
	if err != nil {
		t.Fatalf("load save: %v", err)
	}
	return data
}

// treeFromJSON converts the golden JSON tree representation into a PBTree.
func treeFromJSON(t *testing.T, raw json.RawMessage) PBTree {
	t.Helper()
	var obj map[string][]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	tree := PBTree{}
	for k, entries := range obj {
		fieldNum := 0
		for _, c := range k {
			fieldNum = fieldNum*10 + int(c-'0')
		}
		for _, e := range entries {
			var pair []json.RawMessage
			if err := json.Unmarshal(e, &pair); err != nil {
				t.Fatalf("parse entry: %v", err)
			}
			var wt int
			if err := json.Unmarshal(pair[0], &wt); err != nil {
				t.Fatalf("parse wire type: %v", err)
			}
			value, err := valueFromJSON(pair[1])
			if err != nil {
				t.Fatalf("parse value: %v", err)
			}
			tree[fieldNum] = append(tree[fieldNum], PBEntry{WireType: wt, Value: value})
		}
	}
	return tree
}

func valueFromJSON(raw json.RawMessage) (any, error) {
	var asNum uint64
	if err := json.Unmarshal(raw, &asNum); err == nil {
		return asNum, nil
	}
	var asObj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &asObj); err != nil {
		return nil, err
	}
	if b64raw, ok := asObj["__b64__"]; ok {
		var s string
		if err := json.Unmarshal(b64raw, &s); err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(s)
	}
	sub := PBTree{}
	for k, entriesRaw := range asObj {
		var entries []json.RawMessage
		if err := json.Unmarshal(entriesRaw, &entries); err != nil {
			return nil, err
		}
		fieldNum := 0
		for _, c := range k {
			fieldNum = fieldNum*10 + int(c-'0')
		}
		for _, e := range entries {
			var pair []json.RawMessage
			if err := json.Unmarshal(e, &pair); err != nil {
				return nil, err
			}
			var wt int
			if err := json.Unmarshal(pair[0], &wt); err != nil {
				return nil, err
			}
			value, err := valueFromJSON(pair[1])
			if err != nil {
				return nil, err
			}
			sub[fieldNum] = append(sub[fieldNum], PBEntry{WireType: wt, Value: value})
		}
	}
	return &sub, nil
}

func goldenValuesToItemValues(t *testing.T, raw []json.RawMessage) []int64 {
	t.Helper()
	out := make([]int64, 0, len(raw))
	for _, r := range raw {
		var num *int64
		if err := json.Unmarshal(r, &num); err != nil {
			t.Fatalf("parse item value: %v", err)
		}
		if num == nil {
			out = append(out, ItemNone)
		} else {
			out = append(out, *num)
		}
	}
	return out
}

func TestGoldenUnwrapAndRead(t *testing.T) {
	f := loadGolden(t)
	raw := loadSaveBytes(t)

	playerBytes, err := UnwrapPlayerData(raw)
	if err != nil {
		t.Fatalf("UnwrapPlayerData: %v", err)
	}
	tree, err := ReadProtobuf(playerBytes)
	if err != nil {
		t.Fatalf("ReadProtobuf: %v", err)
	}
	want := treeFromJSON(t, f.Tree)
	if !reflect.DeepEqual(tree, want) {
		t.Fatalf("decoded tree differs from golden")
	}
}

func TestGoldenProtobufWrite(t *testing.T) {
	f := loadGolden(t)
	raw := loadSaveBytes(t)

	playerBytes, err := UnwrapPlayerData(raw)
	if err != nil {
		t.Fatalf("UnwrapPlayerData: %v", err)
	}
	tree, err := ReadProtobuf(playerBytes)
	if err != nil {
		t.Fatalf("ReadProtobuf: %v", err)
	}
	out, err := WriteProtobuf(tree)
	if err != nil {
		t.Fatalf("WriteProtobuf: %v", err)
	}
	sum := sha256.Sum256(out)
	if hex.EncodeToString(sum[:]) != f.ProtobufSHA {
		t.Fatalf("protobuf re-encode mismatch: got %s", hex.EncodeToString(sum[:]))
	}
}

func TestGoldenContainerWrite(t *testing.T) {
	f := loadGolden(t)
	raw := loadSaveBytes(t)

	playerBytes, err := UnwrapPlayerData(raw)
	if err != nil {
		t.Fatalf("UnwrapPlayerData: %v", err)
	}
	out := WrapPlayerData(playerBytes)
	sum := sha256.Sum256(out)
	if hex.EncodeToString(sum[:]) != f.ContainerSHA {
		t.Fatalf("container wrap mismatch: got %s", hex.EncodeToString(sum[:]))
	}
}

func TestGoldenContainerRoundTrip(t *testing.T) {
	raw := loadSaveBytes(t)
	playerBytes, err := UnwrapPlayerData(raw)
	if err != nil {
		t.Fatalf("UnwrapPlayerData: %v", err)
	}
	rewrapped := WrapPlayerData(playerBytes)
	playerBytes2, err := UnwrapPlayerData(rewrapped)
	if err != nil {
		t.Fatalf("UnwrapPlayerData(rewrapped): %v", err)
	}
	if string(playerBytes) != string(playerBytes2) {
		t.Fatalf("round trip changed player bytes")
	}
}

func TestGoldenGibbedCodes(t *testing.T) {
	f := loadGolden(t)
	for section, codes := range f.GibbedCodes {
		for _, code := range codes {
			rawItem, err := ValidateGibbedCode(code)
			if err != nil {
				t.Fatalf("%s: %v", section, err)
			}
			zeroed, err := ReplaceRawItemKey(rawItem, 0)
			if err != nil {
				t.Fatalf("%s: ReplaceRawItemKey: %v", section, err)
			}
			if EncodeGibbedCode(zeroed) != code {
				t.Fatalf("%s: code re-encode mismatch", section)
			}
		}
	}
}

func TestGoldenSyntheticItems(t *testing.T) {
	f := loadGolden(t)
	for i, item := range f.SyntheticItems {
		rawWant, err := base64.StdEncoding.DecodeString(item.RawB64)
		if err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		values := goldenValuesToItemValues(t, item.Values)

		got := WrapItem(item.IsWeapon, values, item.Key)
		if string(got) != string(rawWant) {
			t.Fatalf("item %d: WrapItem mismatch", i)
		}

		isWeapon, gotValues, gotKey, err := UnwrapItem(rawWant)
		if err != nil {
			t.Fatalf("item %d: UnwrapItem: %v", i, err)
		}
		if isWeapon != item.IsWeapon || gotKey != item.Key || !reflect.DeepEqual(gotValues, values) {
			t.Fatalf("item %d: UnwrapItem mismatch", i)
		}

		code, err := ValidateGibbedCode(item.Code)
		if err != nil {
			t.Fatalf("item %d: ValidateGibbedCode: %v", i, err)
		}
		if string(code) != string(rawWant) {
			t.Fatalf("item %d: code decode mismatch", i)
		}

		info, err := UnwrapItemInfo(rawWant)
		if err != nil {
			t.Fatalf("item %d: UnwrapItemInfo: %v", i, err)
		}
		infoJSON, err := json.Marshal(info)
		if err != nil {
			t.Fatalf("item %d: marshal info: %v", i, err)
		}
		var gotInfo, wantInfo map[string]any
		if err := json.Unmarshal(infoJSON, &gotInfo); err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		wantInfoJSON, err := json.Marshal(item.Info)
		if err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		if err := json.Unmarshal(wantInfoJSON, &wantInfo); err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		if !reflect.DeepEqual(gotInfo, wantInfo) {
			t.Fatalf("item %d: info mismatch:\n got %v\nwant %v", i, gotInfo, wantInfo)
		}

		rewrapped := WrapItemInfo(info)
		if string(rewrapped) != string(rawWant) {
			t.Fatalf("item %d: WrapItemInfo mismatch", i)
		}
	}
}

func TestPackUnpackRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(49520))
	for _, isWeapon := range []int{0, 1} {
		sizes := ItemSizes[isWeapon]
		for trial := 0; trial < 200; trial++ {
			values := make([]int64, len(sizes))
			for i := range values {
				values[i] = ItemNone
			}
			for i, size := range sizes {
				if rng.Intn(10) == 0 {
					values[i] = ItemNone
					break
				}
				values[i] = int64(rng.Intn(1 << uint(size)))
			}
			packed := PackItemValues(isWeapon, values)
			unpacked := UnpackItemValues(isWeapon, packed)
			for i := range sizes {
				if i >= len(unpacked) {
					break
				}
				if values[i] == ItemNone {
					continue
				}
				if unpacked[i] != values[i] {
					t.Fatalf("is_weapon=%d trial=%d: value mismatch at %d: got %d want %d", isWeapon, trial, i, unpacked[i], values[i])
				}
			}
		}
	}
}

func TestWrapUnwrapRandomKeys(t *testing.T) {
	rng := rand.New(rand.NewSource(219))
	for _, isWeapon := range []int{0, 1} {
		sizes := ItemSizes[isWeapon]
		for trial := 0; trial < 100; trial++ {
			values := make([]int64, len(sizes))
			for i, size := range sizes {
				values[i] = int64(rng.Intn(1 << uint(size)))
			}
			key := int64(rng.Uint32()) - 0x80000000
			wrapped := WrapItem(isWeapon, values, key)
			gotIsWeapon, gotValues, gotKey, err := UnwrapItem(wrapped)
			if err != nil {
				t.Fatalf("is_weapon=%d trial=%d: %v", isWeapon, trial, err)
			}
			if gotIsWeapon != isWeapon || gotKey != key {
				t.Fatalf("is_weapon=%d trial=%d: header mismatch", isWeapon, trial)
			}
			if !reflect.DeepEqual(gotValues, values) {
				t.Fatalf("is_weapon=%d trial=%d: values mismatch", isWeapon, trial)
			}
		}
	}
}

func TestIsFakeItem(t *testing.T) {
	values := []int64{255, 0, 0, ItemNone, 0}
	if !IsFakeItem(0, values) {
		t.Fatal("expected fake item")
	}
	values[1] = 5
	if IsFakeItem(0, values) {
		t.Fatal("expected real item")
	}
}

func TestUnwrapRejectsCorrupt(t *testing.T) {
	raw := loadSaveBytes(t)
	if _, err := UnwrapPlayerData(raw[:len(raw)-5]); err == nil {
		t.Fatal("expected SHA1 failure on truncated save")
	}
	bad := append([]byte(nil), raw...)
	bad[30] ^= 0xFF
	if _, err := UnwrapPlayerData(bad); err == nil {
		t.Fatal("expected failure on corrupted save")
	}
}

func TestProtobufRejectsTruncated(t *testing.T) {
	if _, err := ReadProtobuf([]byte{0x08}); err == nil {
		t.Fatal("expected truncation error")
	}
	if _, err := ReadProtobuf([]byte{0x0A, 0xFF}); err == nil {
		t.Fatal("expected truncation error")
	}
}
