package service

import "fmt"

// SaveErrorKind classifies outcomes of the transactional save pipeline.
// Validation/authentication errors are outside this classification.
type SaveErrorKind uint8

const (
	// SaveFailed means replacement did not commit. The original file and live
	// database must be unchanged before this outcome can be reported.
	SaveFailed SaveErrorKind = iota + 1
	// SaveConflict means the on-disk baseline changed before replacement.
	// The pending mutation did not commit.
	SaveConflict
	// SaveDurabilityUncertain means replacement committed, but a subsequent
	// durability check failed. Live state must reflect the committed mutation.
	// Retrying the mutation can duplicate an addition.
	SaveDurabilityUncertain
)

// SaveError describes a transactional save outcome and preserves its cause for
// errors.Is/errors.As. Op must describe an operation, never credentials or entry
// contents. CRUD and LockAndSave use these outcomes for persistence errors;
// authentication and in-memory validation errors remain plain errors.
type SaveError struct {
	Kind SaveErrorKind
	Op   string
	Err  error
}

func (e *SaveError) Error() string {
	outcome := "unknown outcome"
	switch e.Kind {
	case SaveFailed:
		outcome = "not committed"
	case SaveConflict:
		outcome = "conflict; not committed"
	case SaveDurabilityUncertain:
		outcome = "committed; durability uncertain"
	}
	return fmt.Sprintf("save %s (%s): %v", e.Op, outcome, e.Err)
}

func (e *SaveError) Unwrap() error { return e.Err }

// Committed reports whether the replacement committed despite this error.
// A false result is meaningful only for a recognized SaveErrorKind.
func (e *SaveError) Committed() bool { return e.Kind == SaveDurabilityUncertain }
