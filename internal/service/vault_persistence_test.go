package service

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

func TestVaultPersistenceReadsBytesAndOpenedFilePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	data := []byte("database snapshot")
	require.NoError(t, os.WriteFile(path, data, 0o640))
	info, err := os.Stat(path)
	require.NoError(t, err)
	snapshot, err := newVaultPersistence().read(path)
	require.NoError(t, err)
	assert.Equal(t, data, snapshot.data)
	assert.Equal(t, sha256.Sum256(data), snapshot.baseline.fingerprint)
	assert.Equal(t, info.Mode().Perm(), snapshot.baseline.mode)
}

func TestVaultPersistenceReadFailuresCloseFileAndDiscardSnapshot(t *testing.T) {
	cause := errors.New("injected read failure")
	for _, operation := range []string{"stat", "read", "close"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			require.NoError(t, os.WriteFile(path, []byte("snapshot"), 0o600))
			file, err := os.Open(path)
			require.NoError(t, err)
			t.Cleanup(func() { file.Close() })
			fault := &faultVaultReadFile{vaultReadFile: file}
			switch operation {
			case "stat":
				fault.statErr = cause
			case "read":
				fault.readErr = cause
			case "close":
				fault.closeErr = cause
			}
			p := newVaultPersistence()
			p.open = func(string) (vaultReadFile, error) { return fault, nil }
			snapshot, err := p.read(path)
			require.ErrorIs(t, err, cause)
			assert.Nil(t, snapshot)
			assert.Equal(t, 1, fault.closes)
		})
	}
}

func TestAuthenticationUsesInjectedPersistenceAndRejectsReadFailure(t *testing.T) {
	vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root"})
	cause := errors.New("injected open failure")
	vault.persistence.open = func(string) (vaultReadFile, error) { return nil, cause }
	require.ErrorIs(t, vault.ReAuthenticate(), cause)
	assert.False(t, vault.authenticated)
	assert.Nil(t, vault.baseline)
}

func TestVaultPersistencePreparesFileWithoutChangingDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	original, replacement := []byte("original"), []byte("replacement")
	require.NoError(t, os.WriteFile(path, original, 0o640))
	info, err := os.Stat(path)
	require.NoError(t, err)
	p := newVaultPersistence()
	file := captureTestTempFile(p)
	prepared, err := p.prepare(path, info.Mode(), func(w io.Writer) error {
		if runtime.GOOS != "windows" {
			initial, err := os.Stat(file.Name())
			if err != nil {
				return err
			}
			if initial.Mode().Perm() != 0o600 {
				return errors.New("temporary database is not private during encoding")
			}
		}
		_, err := w.Write(replacement)
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	assert.Equal(t, filepath.Dir(path), filepath.Dir(prepared.path))
	assert.Equal(t, original, readTestFile(t, path))
	assert.Equal(t, replacement, readTestFile(t, prepared.path))
	assert.Equal(t, sha256.Sum256(replacement), prepared.baseline.fingerprint)
	assert.Equal(t, info.Mode().Perm(), prepared.baseline.mode)
	tempInfo, err := os.Stat(prepared.path)
	require.NoError(t, err)
	assert.Equal(t, info.Mode().Perm(), tempInfo.Mode().Perm())
	assert.Equal(t, 1, file.closes)
	tempPath := prepared.path
	require.NoError(t, prepared.discard())
	require.NoError(t, prepared.discard(), "discard is safe after ownership is released")
	_, err = os.Stat(tempPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestVaultPersistencePreparationFailuresPreserveDestinationAndCleanUp(t *testing.T) {
	cause := errors.New("injected preparation failure")
	for _, operation := range []string{"create", "encode", "write", "chmod", "sync", "close"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			original := []byte("original database")
			require.NoError(t, os.WriteFile(path, original, 0o600))
			p := newVaultPersistence()
			file := captureTestTempFile(p)
			switch operation {
			case "create":
				p.createTemp = func(string, string) (vaultTempFile, error) { return nil, cause }
			case "write":
				file.writeErr = cause
			case "chmod":
				file.chmodErr = cause
			case "sync":
				file.syncErr = cause
			case "close":
				file.closeErr = cause
			}
			prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
				if _, err := w.Write([]byte("replacement database")); err != nil {
					return err
				}
				if operation == "encode" {
					return cause
				}
				return nil
			})
			require.ErrorIs(t, err, cause)
			assert.Nil(t, prepared)
			assert.Equal(t, original, readTestFile(t, path))
			entries, err := os.ReadDir(filepath.Dir(path))
			require.NoError(t, err)
			require.Len(t, entries, 1, "no temporary database should remain")
			assert.Equal(t, "vault.kdbx", entries[0].Name())
			if operation != "create" {
				assert.Equal(t, 1, file.closes)
			}
		})
	}
}

func TestVaultPersistenceReportsCleanupFailuresAlongsideEncodingFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	p := newVaultPersistence()
	file := captureTestTempFile(p)
	encodeErr := errors.New("encoding failed")
	closeErr := errors.New("close failed")
	removeErr := errors.New("remove failed")
	file.closeErr = closeErr
	p.remove = func(string) error { return removeErr }
	prepared, err := p.prepare(path, 0o600, func(io.Writer) error { return encodeErr })
	assert.Nil(t, prepared)
	require.ErrorIs(t, err, encodeErr)
	assert.ErrorIs(t, err, closeErr)
	assert.ErrorIs(t, err, removeErr)
	assert.Equal(t, 1, file.closes)
	require.NoError(t, os.Remove(file.Name()))
}

func TestVaultPersistenceReplacementFailureRetainsFileForDiscard(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	original := []byte("original database")
	require.NoError(t, os.WriteFile(path, original, 0o600))
	p := newVaultPersistence()
	prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
		_, err := w.Write([]byte("replacement database"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	cause := errors.New("replacement failed")
	p.replace = func(string, string) error { return cause }
	require.ErrorIs(t, prepared.replace(path), cause)
	assert.Equal(t, original, readTestFile(t, path))
	tempPath := prepared.path
	require.NotEmpty(t, tempPath)
	require.NoError(t, prepared.discard())
	_, err = os.Stat(tempPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestVaultPersistenceReplacementTransfersOwnership(t *testing.T) {
	// Use a new destination here: overwriting an existing file with platform-
	// specific atomicity and directory syncing belongs to the later save task.
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	replacement := []byte("replacement database")
	p := newVaultPersistence()
	prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(replacement)
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	require.NoError(t, prepared.replace(path))
	assert.Empty(t, prepared.path)
	require.NoError(t, prepared.discard())
	require.Error(t, prepared.replace(path), "replacement cannot be repeated after ownership transfer")
	assert.Equal(t, replacement, readTestFile(t, path))
}

func TestVaultPersistenceDiscardFailureCanBeRetried(t *testing.T) {
	for _, alreadyRemoved := range []bool{false, true} {
		name := "file still exists"
		if alreadyRemoved {
			name = "file already removed"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			p := newVaultPersistence()
			prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
				_, err := w.Write([]byte("pending database"))
				return err
			})
			require.NoError(t, err)
			t.Cleanup(func() { prepared.discard() })
			tempPath := prepared.path
			remove := p.remove
			cause := errors.New("discard failed")
			p.remove = func(string) error { return cause }
			require.ErrorIs(t, prepared.discard(), cause)
			assert.Equal(t, tempPath, prepared.path, "failed discard must retain ownership")
			assert.Equal(t, []byte("pending database"), readTestFile(t, tempPath))
			p.remove = remove

			if alreadyRemoved {
				// Another cleanup may remove the file before the retry.
				require.NoError(t, os.Remove(tempPath))
			}
			require.NoError(t, prepared.discard())
			assert.Empty(t, prepared.path, "successful discard releases ownership")
			_, err = os.Stat(tempPath)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}
