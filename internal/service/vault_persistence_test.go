package service

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
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

func TestVaultPreparationRejectsIgnoredEncodingWriteFailures(t *testing.T) {
	for _, shortWrite := range []bool{false, true} {
		name := "one-shot write error"
		if shortWrite {
			name = "short write without an error"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			original := []byte("original database")
			require.NoError(t, os.WriteFile(path, original, 0o600))
			p := newVaultPersistence()
			file := captureTestTempFile(p)
			cause := errors.New("one-shot write failure")
			if shortWrite {
				file.shortWrite = true
				cause = io.ErrShortWrite
			} else {
				file.writeErr, file.writeOnce = cause, true
			}
			prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
				// Simulate an encoder that ignores header write errors and then
				// reports success after continuing with the payload.
				w.Write([]byte("header"))
				w.Write([]byte("payload"))
				return nil
			})
			require.ErrorIs(t, err, cause)
			assert.Nil(t, prepared)
			assert.Equal(t, original, readTestFile(t, path))
			_, err = os.Stat(file.Name())
			assert.ErrorIs(t, err, os.ErrNotExist)
			assert.Equal(t, 1, file.closes)
		})
	}
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
	p.replace = func(string, string) (bool, error) { return false, cause }
	committed, err := prepared.replace(path)
	require.ErrorIs(t, err, cause)
	assert.False(t, committed)
	assert.Equal(t, original, readTestFile(t, path))
	tempPath := prepared.path
	require.NotEmpty(t, tempPath)
	require.NoError(t, prepared.discard())
	_, err = os.Stat(tempPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestVaultPersistenceReplacementTransfersOwnership(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original database"), 0o600))
	replacement := []byte("replacement database")
	p := newVaultPersistence()
	prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
		_, err := w.Write(replacement)
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	committed, err := prepared.commit(path)
	require.NoError(t, err)
	assert.True(t, committed)
	assert.Empty(t, prepared.path)
	require.NoError(t, prepared.discard())
	committed, err = prepared.commit(path)
	require.Error(t, err, "replacement cannot be repeated after ownership transfer")
	assert.False(t, committed)
	assert.Equal(t, replacement, readTestFile(t, path))
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	}
}

func TestVaultCommitPreservesPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original database"), 0o640))
	info, err := os.Stat(path)
	require.NoError(t, err)
	prepared, err := newVaultPersistence().prepare(path, info.Mode(), func(w io.Writer) error {
		_, err := w.Write([]byte("replacement database"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	committed, err := prepared.commit(path)
	require.NoError(t, err)
	require.True(t, committed)
	replacedInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, info.Mode().Perm(), replacedInfo.Mode().Perm())
}

func TestVaultAtomicReplacementKeePassRoundTrip(t *testing.T) {
	for _, version := range []int{3, 4} {
		t.Run(fmt.Sprintf("KDBX%d", version), func(t *testing.T) {
			source := richTestDatabase(t, version)
			originalEncoding, err := encodingVaultCopy(source)
			require.NoError(t, err)
			var original bytes.Buffer
			require.NoError(t, gokeepasslib.NewEncoder(&original).Encode(originalEncoding))
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			require.NoError(t, os.WriteFile(path, original.Bytes(), 0o600))
			working := copyVaultDatabase(source)
			entry := &working.Content.Root.Groups[0].Groups[0].Entries[0]
			setEntryValue(entry, PasswordKey, "replacement secret")
			encoding, err := encodingVaultCopy(working)
			require.NoError(t, err)
			prepared, err := newVaultPersistence().prepare(path, 0o600, func(w io.Writer) error {
				return gokeepasslib.NewEncoder(w).Encode(encoding)
			})
			require.NoError(t, err)
			t.Cleanup(func() { prepared.discard() })
			assert.Equal(t, original.Bytes(), readTestFile(t, path), "preparation must preserve the existing database")
			committed, err := prepared.commit(path)
			require.NoError(t, err)
			require.True(t, committed)
			reopened := decodeTestDatabase(t, readTestFile(t, path), source.Credentials)
			result := reopened.Content.Root.Groups[0].Groups[0].Entries[0]
			assert.Equal(t, entry.UUID, result.UUID)
			assert.Equal(t, "replacement secret", result.GetPassword())
			assert.Equal(t, "old secret", result.Histories[0].Entries[0].GetPassword())
		})
	}
}

func TestVaultCommitSeparatesUncommittedAndCommittedFailures(t *testing.T) {
	for _, operation := range []string{"replace", "post-replace flush", "directory sync"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			original, replacement := []byte("original database"), []byte("replacement database")
			require.NoError(t, os.WriteFile(path, original, 0o600))
			p := newVaultPersistence()
			prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
				_, err := w.Write(replacement)
				return err
			})
			require.NoError(t, err)
			t.Cleanup(func() { prepared.discard() })
			tempPath := prepared.path
			cause := errors.New("injected commit failure")
			directorySyncs := 0
			switch operation {
			case "replace":
				p.replace = func(string, string) (bool, error) { return false, cause }
			case "post-replace flush":
				replace := p.replace
				p.replace = func(from, to string) (bool, error) {
					committed, err := replace(from, to)
					if err != nil || !committed {
						return committed, err
					}
					return true, cause
				}
			}
			replace := p.replace
			replacementCalls := 0
			p.replace = func(from, to string) (bool, error) {
				replacementCalls++
				return replace(from, to)
			}
			p.syncDirectory = func(string) error {
				directorySyncs++
				return cause
			}
			committed, err := prepared.commit(path)
			require.ErrorIs(t, err, cause)
			assert.Equal(t, operation != "replace", committed)
			if committed {
				assert.Equal(t, replacement, readTestFile(t, path))
			} else {
				assert.Equal(t, original, readTestFile(t, path))
			}
			assert.Empty(t, prepared.path, "failed pre-commit attempts clean up; committed attempts consume the path")
			_, err = os.Stat(tempPath)
			assert.ErrorIs(t, err, os.ErrNotExist)
			if operation == "directory sync" {
				assert.Equal(t, 1, directorySyncs)
			} else {
				assert.Zero(t, directorySyncs)
			}
			require.NoError(t, prepared.discard())
			if committed {
				committedAgain, retryErr := prepared.commit(path)
				require.Error(t, retryErr)
				assert.False(t, committedAgain)
				assert.Equal(t, 1, replacementCalls, "committed errors must not allow a second replacement")
				if operation == "directory sync" {
					assert.Equal(t, 1, directorySyncs, "retry must not invoke directory sync again")
				} else {
					assert.Zero(t, directorySyncs)
				}
				assert.Equal(t, replacement, readTestFile(t, path))
			}
		})
	}
}

func TestVaultCommitReportsReplacementAndCleanupFailures(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	p := newVaultPersistence()
	prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
		_, err := w.Write([]byte("replacement"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	replaceErr, cleanupErr := errors.New("replacement failed"), errors.New("cleanup failed")
	p.replace = func(string, string) (bool, error) { return false, replaceErr }
	remove := p.remove
	p.remove = func(string) error { return cleanupErr }
	committed, err := prepared.commit(path)
	assert.False(t, committed)
	require.ErrorIs(t, err, replaceErr)
	assert.ErrorIs(t, err, cleanupErr)
	assert.NotEmpty(t, prepared.path, "cleanup failure retains ownership for explicit discard")
	assert.Equal(t, []byte("original"), readTestFile(t, path))
	p.remove = remove
	require.NoError(t, prepared.discard())
}

func TestNativeVaultReplacementFailurePreservesDestination(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directory-target")
	require.NoError(t, os.Mkdir(path, 0o755))
	child := filepath.Join(path, "original")
	require.NoError(t, os.WriteFile(child, []byte("original contents"), 0o600))
	prepared, err := newVaultPersistence().prepare(path, 0o600, func(w io.Writer) error {
		_, err := w.Write([]byte("replacement"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	tempPath := prepared.path
	committed, err := prepared.commit(path)
	require.Error(t, err)
	assert.False(t, committed)
	assert.Equal(t, []byte("original contents"), readTestFile(t, child))
	_, err = os.Stat(tempPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestVaultReplacementReadersObserveOnlyCompleteFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows sharing restrictions can reject replacement while readers hold the target open")
	}
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	first, second := bytes.Repeat([]byte("a"), 16384), bytes.Repeat([]byte("b"), 16384)
	require.NoError(t, os.WriteFile(path, first, 0o600))
	done := make(chan struct{})
	p := newVaultPersistence()
	runConcurrentTestOperations(t,
		func() error {
			defer close(done)
			for i := range 24 {
				data := first
				if i%2 == 0 {
					data = second
				}
				prepared, err := p.prepare(path, 0o600, func(w io.Writer) error {
					_, err := w.Write(data)
					return err
				})
				if err != nil {
					return err
				}
				if _, err := prepared.commit(path); err != nil {
					return err
				}
			}
			return nil
		},
		func() error {
			for {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if !bytes.Equal(data, first) && !bytes.Equal(data, second) {
					return fmt.Errorf("reader observed an incomplete replacement (%d bytes)", len(data))
				}
				select {
				case <-done:
					return nil
				default:
				}
			}
		},
	)
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
