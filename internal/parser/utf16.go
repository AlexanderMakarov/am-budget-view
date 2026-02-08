package parser

import (
	"bytes"
	"fmt"
	"unicode/utf16"
)

func decodeUTF16ToUTF8(utf16Bytes []byte) ([]byte, error) {
	if len(utf16Bytes)%2 != 0 {
		return nil, fmt.Errorf("UTF-16 byte slice must have even length")
	}

	utf16s := make([]uint16, len(utf16Bytes)/2)
	for i := 0; i < len(utf16Bytes); i += 2 {
		utf16s[i/2] = uint16(utf16Bytes[i]) + (uint16(utf16Bytes[i+1]) << 8)
	}

	var buf bytes.Buffer
	for _, r := range utf16.Decode(utf16s) {
		buf.WriteRune(r)
	}

	return buf.Bytes(), nil
}
