package bl2save

import (
	"encoding/base64"
	"errors"
	"strings"
)

func b64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// ValidateGibbedCode validates a single BL2(...) Gibbed code and returns the
// decoded item bytes. Port of save_io.validate_gibbed_code.
func ValidateGibbedCode(codeStr string) ([]byte, error) {
	codeStr = strings.TrimSpace(codeStr)
	if !strings.HasPrefix(codeStr, "BL2(") || !strings.HasSuffix(codeStr, ")") {
		return nil, errors.New("not a valid BL2(...) code")
	}
	b64 := codeStr[4 : len(codeStr)-1]
	clean := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		}
		return r
	}, b64)
	codeBytes, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, errors.New("invalid base64 encoding")
	}
	if len(codeBytes) < 5 {
		return nil, errors.New("code too short, minimum 5 bytes")
	}
	isWeapon, itemValues, _, err := UnwrapItem(codeBytes)
	if err != nil {
		return nil, errors.New("failed to decode item data: " + err.Error())
	}
	_ = isWeapon
	critical := itemValues[1:4]
	empty := true
	for _, v := range critical {
		if v != ItemNone && v != 0 {
			empty = false
			break
		}
	}
	if empty {
		return nil, errors.New("item has no type, balance, or manufacturer")
	}
	return codeBytes, nil
}

// EncodeGibbedCode serializes raw item bytes into a BL2(...) code string.
func EncodeGibbedCode(raw []byte) string {
	return "BL2(" + b64Encode(raw) + ")"
}
