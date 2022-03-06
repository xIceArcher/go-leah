package utils

import "unicode/utf16"

func GetUTF16StringIdx(s string, utf16Idx int) int {
	prefixDecoded := SliceUTF16String(s, 0, utf16Idx)
	return len(string(prefixDecoded))
}

func SliceUTF16String(s string, start int, end int) string {
	utf16EncodedStr := utf16.Encode([]rune(s))

	if end > len(utf16EncodedStr) {
		end = len(utf16EncodedStr)
	}

	slice := utf16EncodedStr[start:end]
	return string(utf16.Decode(slice))
}
