package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomPasswordGeneratesGivenLengthString(t *testing.T) {
	length := 16
	pwd := GenerateRandomPassword(length)

	assert.Len(t, pwd, length)
}

func TestGenerateRandomPasswordUsesAllowedCharacters(t *testing.T) {
	pwd := GenerateRandomPassword(256)

	for _, char := range pwd {
		assert.Contains(t, charset, string(char))
	}
}

func TestGenerateRandomPasswordZeroLengthReturnsEmptyString(t *testing.T) {
	assert.Empty(t, GenerateRandomPassword(0))
}

func TestGenerateRandomPasswordCanUseEntireCharset(t *testing.T) {
	seen := map[rune]bool{}

	for range 2000 {
		for _, char := range GenerateRandomPassword(64) {
			seen[char] = true
		}
	}

	for _, char := range charset {
		assert.Truef(t, seen[char], "expected generated passwords to include %q", char)
	}
}
