package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTernaryReturnsTrueValue(t *testing.T) {
	assert.Equal(t, "yes", Ternary(true, "yes", "no"))
}

func TestTernaryReturnsFalseValue(t *testing.T) {
	assert.Equal(t, 2, Ternary(false, 1, 2))
}
