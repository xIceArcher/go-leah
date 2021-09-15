package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStringIndex(t *testing.T) {
	s := "aaaax aaa"
	assert.Equal(t, 4, GetStringIdx([]rune(s), 4))
}
