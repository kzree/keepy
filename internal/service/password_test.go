package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateRandomPasswordGeneratesGivenLengthString(t *testing.T) {
	for _, length := range []int{1, 16, 24, 256} {
		assert.Len(t, GenerateRandomPassword(length), length)
	}
}

func TestGenerateRandomPasswordUsesAllowedCharacters(t *testing.T) {
	pwd := GenerateRandomPassword(256)

	for _, char := range pwd {
		assert.Contains(t, charset, string(char))
	}
}

func TestGenerateRandomPasswordNonpositiveLengthReturnsEmptyString(t *testing.T) {
	for _, length := range []int{0, -1, -24} {
		assert.Empty(t, GenerateRandomPassword(length))
	}
}

func TestPasswordByteMappingIsCompleteAndUnbiased(t *testing.T) {
	// Exhaust every possible byte, proving each distinct alphabet character has
	// exactly four preimages without relying on random sampling.
	random := make([]byte, 256)
	for i := range random {
		random[i] = byte(i)
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890-_"
	assert.Len(t, alphabet, 64)
	password := passwordFromRandomBytes(random)
	assert.Equal(t, strings.Repeat(alphabet, 4), password)
	counts := make(map[rune]int)
	for _, char := range password {
		counts[char]++
	}
	assert.Len(t, counts, 64)
	for _, count := range counts {
		assert.Equal(t, 4, count)
	}
}
