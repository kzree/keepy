package service

import (
	"bytes"
	"errors"

	"github.com/tobischo/gokeepasslib/v3"
)

// Authentication failures. Authenticate wraps them, and keeps the original cause
// available to errors.Is/errors.As.
var (
	ErrNotDatabase        = errors.New("not a KeePass KDBX database")
	ErrInvalidCredentials = errors.New("wrong password or key file, or the database is damaged")
)

// Vault access errors for a locked vault or an entry that no longer exists.
var (
	ErrNotAuthenticated = errors.New("vault is not authenticated")
	ErrEntryNotFound    = errors.New("entry not found")
)

// AuthFile identifies the file an authentication failure concerns.
type AuthFile uint8

const (
	AuthDatabase AuthFile = iota + 1
	AuthKeyFile
)

// AuthFileError reports that a database or key file could not be located or
// read. Its message is the cause's, which already names the path.
type AuthFileError struct {
	File AuthFile
	Err  error
}

func (e *AuthFileError) Error() string { return e.Err.Error() }

func (e *AuthFileError) Unwrap() error { return e.Err }

// hasKDBXSignature reports whether data starts with the KDBX file signature.
// gokeepasslib does not check it, so other files fail with unrelated read errors.
func hasKDBXSignature(data []byte) bool {
	return len(data) >= 8 &&
		bytes.Equal(data[:4], gokeepasslib.BaseSignature[:]) &&
		bytes.Equal(data[4:8], gokeepasslib.SecondarySignature[:])
}
