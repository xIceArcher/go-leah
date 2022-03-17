package utils

import (
	"regexp"

	"golang.org/x/exp/slices"
)

type TextWithEntities struct {
	Text     string
	Entities []*Entity
}

func (t *TextWithEntities) AddByRegex(regex *regexp.Regexp, f func(string) string) {
	regex.Longest()
	strings := regex.FindAllString(t.Text, -1)
	indices := regex.FindAllStringIndex(t.Text, -1)

	for i := 0; i < len(strings); i++ {
		t.Entities = append(t.Entities, NewEntityWithReplacement(indices[i][0], strings[i], f(strings[i])))
	}
}

func (t *TextWithEntities) AddEntities(f func(*Entity) string, entities ...*Entity) {
	for _, entity := range entities {
		t.Entities = append(t.Entities, NewEntityWithReplacement(entity.start, entity.Match, f(entity)))
	}
}

func (t *TextWithEntities) GetReplacedText(maxBytes int, n int) (ret []string) {
	slices.SortFunc(t.Entities, func(a, b *Entity) bool {
		return a.Start() < b.Start()
	})

	currBytesLeft := maxBytes
	currStr := ""
	currMatchIdx := 0

	for idx := 0; idx < len(t.Text); idx++ {
		var currMatch *Entity
		if currMatchIdx < len(t.Entities) {
			currMatch = t.Entities[currMatchIdx]
		} else {
			currMatch = nil
		}

		if currMatch != nil && idx == currMatch.Start() {
			if currBytesLeft >= len(currMatch.Replacement) {
				currStr += currMatch.Replacement
				currBytesLeft -= len(currMatch.Replacement)
			} else {
				ret = append(ret, currStr)
				currStr = currMatch.Replacement
				currBytesLeft = maxBytes - len(currMatch.Replacement)
			}

			idx = currMatch.End() - 1
			currMatchIdx++
		} else {
			currStr += t.Text[idx : idx+1]
			currBytesLeft--
		}

		if currBytesLeft == 0 {
			ret = append(ret, currStr)
			currStr = ""
			currBytesLeft = maxBytes
		}

		if len(ret) == n {
			return
		}
	}

	if currStr != "" {
		ret = append(ret, currStr)
	}

	if len(ret) == 0 {
		return []string{""}
	}

	return
}

type Entity struct {
	start int
	Match string

	Replacement    string
	HasReplacement bool
}

func NewEntity(start int, match string) *Entity {
	return &Entity{
		start: start,
		Match: match,

		HasReplacement: false,
	}
}

func NewEntityWithReplacement(start int, match string, replacement string) *Entity {
	return &Entity{
		start: start,
		Match: match,

		Replacement:    replacement,
		HasReplacement: true,
	}
}

func (r *Entity) Start() int {
	return r.start
}

func (r *Entity) End() int {
	return r.start + len(r.Match)
}
