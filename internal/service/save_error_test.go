package service

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveErrorClassifiesWrappedOutcomes(t *testing.T) {
	tests := []struct {
		kind      SaveErrorKind
		committed bool
		message   string
	}{
		{SaveFailed, false, "not committed"},
		{SaveConflict, false, "conflict; not committed"},
		{SaveDurabilityUncertain, true, "committed; durability uncertain"},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			outcome := &SaveError{Kind: tt.kind, Op: "sync directory", Err: io.ErrUnexpectedEOF}
			wrapped := fmt.Errorf("update entry: %w", outcome)
			var got *SaveError
			require.True(t, errors.As(wrapped, &got))
			assert.Equal(t, tt.kind, got.Kind)
			assert.Equal(t, tt.committed, got.Committed())
			assert.ErrorIs(t, wrapped, io.ErrUnexpectedEOF)
			assert.Contains(t, got.Error(), tt.message)
			assert.Contains(t, got.Error(), "sync directory")
		})
	}
}

func TestPlainErrorDoesNotPromiseTransactionalOutcome(t *testing.T) {
	var outcome *SaveError
	assert.False(t, errors.As(io.ErrUnexpectedEOF, &outcome))
}
