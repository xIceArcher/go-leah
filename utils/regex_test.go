package utils

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetLongestMatches(t *testing.T) {
	regex := regexp.MustCompile(`(?:[@|＠])[\p{L}\p{M}][\p{L}\p{M}\p{N}_]*`)
	text := "@a ＠bbbb @aaa ＠accc"
	replaceFunc := func(s string) string {
		return s + "xxx"
	}

	assert.Equal(t, 3, len("＠"))

	textWithEntities := &TextWithEntities{Text: text}
	textWithEntities.AddByRegex(regex, replaceFunc)
	assert.Equal(t, 4, len(textWithEntities.Entities))

	assert.Equal(t, NewEntityWithReplacement(0, "@a", "@axxx"), textWithEntities.Entities[0])
	assert.Equal(t, NewEntityWithReplacement(3, "＠bbbb", "＠bbbbxxx"), textWithEntities.Entities[1])
	assert.Equal(t, NewEntityWithReplacement(11, "@aaa", "@aaaxxx"), textWithEntities.Entities[2])
	assert.Equal(t, NewEntityWithReplacement(16, "＠accc", "＠acccxxx"), textWithEntities.Entities[3])
}

func Test_GetLongestMatches_2(t *testing.T) {
	regex := regexp.MustCompile(`(?:[#|＃])[\p{L}\p{M}][\p{L}\p{M}\p{N}_]*`)
	text := `たーくさんのありがとうを込めて	#ダイアローグ #ダイアローグワン`

	textWithEntities := &TextWithEntities{Text: text}
	textWithEntities.AddByRegex(regex, func(s string) string { return s })
	assert.Equal(t, 2, len(textWithEntities.Entities))
}

func Test_ReplaceAllMatches(t *testing.T) {
	regex := regexp.MustCompile(`(?:[@|＠])[\p{L}\p{M}][\p{L}\p{M}\p{N}_]*`)
	text := "@a ＠bbbb @aaa ＠accc abc"
	replaceFunc := func(string) string {
		return "#xxxxx"
	}

	textWithEntities := &TextWithEntities{Text: text}
	textWithEntities.AddByRegex(regex, replaceFunc)
	assert.Equal(t, 4, len(textWithEntities.Entities))

	ret := textWithEntities.GetReplacedText(1000, -1)
	assert.Equal(t, 1, len(ret))
	assert.Equal(t, "#xxxxx #xxxxx #xxxxx #xxxxx abc", ret[0])

	ret = textWithEntities.GetReplacedText(6, -1)
	assert.Equal(t, 8, len(ret))
	assert.Equal(t, "#xxxxx", ret[0])
	assert.Equal(t, " ", ret[1])
	assert.Equal(t, "#xxxxx", ret[2])
	assert.Equal(t, " ", ret[3])
	assert.Equal(t, "#xxxxx", ret[4])
	assert.Equal(t, " ", ret[5])
	assert.Equal(t, "#xxxxx", ret[6])
	assert.Equal(t, " abc", ret[7])

	ret = textWithEntities.GetReplacedText(6, 3)
	assert.Equal(t, 3, len(ret))
	assert.Equal(t, "#xxxxx", ret[0])
	assert.Equal(t, " ", ret[1])
	assert.Equal(t, "#xxxxx", ret[2])

	ret = textWithEntities.GetReplacedText(7, -1)
	assert.Equal(t, 5, len(ret))
	assert.Equal(t, "#xxxxx ", ret[0])
	assert.Equal(t, "#xxxxx ", ret[1])
	assert.Equal(t, "#xxxxx ", ret[2])
	assert.Equal(t, "#xxxxx ", ret[3])
	assert.Equal(t, "abc", ret[4])

	ret = textWithEntities.GetReplacedText(7, 2)
	assert.Equal(t, 2, len(ret))
	assert.Equal(t, "#xxxxx ", ret[0])
	assert.Equal(t, "#xxxxx ", ret[1])
}
