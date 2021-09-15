package utils

func GetStringIdx(runes []rune, runeIdx int) int {
	// Extract all the runes up to but not including the rune at runeIdx
	// Then find how many bytes it takes to store those runes
	// This is the index of the corresponding byte in the string representation of the runes
	return len(string(runes[0:runeIdx]))
}
