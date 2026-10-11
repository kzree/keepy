package service

import (
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

func TestGuardedSaveRejectsExternalChangesBeforePreparationAndReplacement(t *testing.T) {
	for _, phase := range []string{"before preparation", "before replacement"} {
		for _, change := range []string{"content", "deleted", "identical replacement", "permissions", "symlink", "directory"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				values := NewVaultEntry{Title: "Original", Password: "secret"}
				entry := values.ToKeepassEntry()
				vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root", Entries: []gokeepasslib.Entry{entry}})
				p, path := vault.persistence, vault.dbPath
				original := readTestFile(t, path)
				baseline := *vault.baseline
				liveBefore := snapshotTestDatabase(t, vault.db)
				encodings, replacements := 0, 0
				p.replace = func(string, string) (bool, error) {
					replacements++
					return false, errors.New("replacement must not run on conflict")
				}
				changeFile := func() {
					switch change {
					case "content":
						require.NoError(t, os.WriteFile(path, []byte("external contents"), baseline.mode))
					case "deleted":
						require.NoError(t, os.Remove(path))
					case "identical replacement":
						replaceTestFile(t, path, original, baseline.mode)
					case "permissions":
						require.NoError(t, os.Chmod(path, 0o444))
						t.Cleanup(func() { os.Chmod(path, baseline.mode) })
					case "symlink":
						backup := path + ".original"
						require.NoError(t, os.Rename(path, backup))
						symlinkTestFile(t, filepath.Base(backup), path)
					case "directory":
						require.NoError(t, os.Remove(path))
						require.NoError(t, os.Mkdir(path, 0o755))
					}
				}
				if phase == "before preparation" {
					changeFile()
				}
				prepared, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
					encodings++
					_, err := w.Write([]byte("pending mutation"))
					return err
				})
				if phase == "before replacement" {
					require.NoError(t, err)
					t.Cleanup(func() { prepared.discard() })
					tempPath := prepared.path
					changeFile()
					committed, commitErr := prepared.commitAndUpdateBaseline(path, vault.baseline)
					err = commitErr
					assert.False(t, committed)
					_, statErr := os.Stat(tempPath)
					assert.ErrorIs(t, statErr, os.ErrNotExist)
					assert.Equal(t, 1, encodings)
				} else {
					assert.Nil(t, prepared)
					assert.Zero(t, encodings)
				}
				requireSaveErrorKind(t, err, SaveConflict)
				assert.Zero(t, replacements)
				assert.Equal(t, baseline, *vault.baseline, "conflicts must not refresh the session baseline")
				assert.Equal(t, liveBefore, snapshotTestDatabase(t, vault.db))
				assertEntryPassword(t, vault, entry.UUID, "secret")
				switch change {
				case "deleted":
					_, err := os.Stat(path)
					assert.ErrorIs(t, err, os.ErrNotExist, "deleted database must not be recreated")
				case "directory":
					info, err := os.Stat(path)
					require.NoError(t, err)
					assert.True(t, info.IsDir())
				case "content":
					assert.Equal(t, []byte("external contents"), readTestFile(t, path))
				default:
					assert.Equal(t, original, readTestFile(t, path))
				}
			})
		}
	}
}

func TestBaselineCheckDetectsChangesBetweenReadAndNamespaceRecheck(t *testing.T) {
	for _, change := range []string{"replacement", "in-place write"} {
		t.Run(change, func(t *testing.T) {
			vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root"})
			p, path := vault.persistence, vault.dbPath
			original := readTestFile(t, path)
			open := p.open
			p.open = func(path string) (vaultReadFile, error) {
				file, err := open(path)
				if err != nil {
					return nil, err
				}
				return &faultVaultReadFile{vaultReadFile: file, afterClose: func() error {
					if change == "in-place write" {
						return os.WriteFile(path, []byte("external contents"), vault.baseline.mode)
					}
					replacement := path + ".external"
					if err := os.WriteFile(replacement, original, vault.baseline.mode); err != nil {
						return err
					}
					_, err := replaceVaultFile(replacement, path)
					return err
				}}, nil
			}
			requireSaveErrorKind(t, p.checkBaseline(path, *vault.baseline), SaveConflict)
		})
	}
}

func TestBaselineReadFailureDoesNotMasqueradeAsConflict(t *testing.T) {
	vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root"})
	original := readTestFile(t, vault.dbPath)
	vault.persistence.open = func(string) (vaultReadFile, error) { return nil, os.ErrPermission }
	err := vault.persistence.checkBaseline(vault.dbPath, *vault.baseline)
	requireSaveErrorKind(t, err, SaveFailed)
	assert.ErrorIs(t, err, os.ErrPermission)
	assert.Equal(t, original, readTestFile(t, vault.dbPath))
}

func TestGuardedPreparationRequiresAnAuthenticatedBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("existing database"), 0o600))
	encoded := false
	prepared, err := newVaultPersistence().prepareChecked(path, vaultFileBaseline{}, func(io.Writer) error {
		encoded = true
		return nil
	})
	requireSaveErrorKind(t, err, SaveFailed)
	assert.Nil(t, prepared)
	assert.False(t, encoded)
	assert.Equal(t, []byte("existing database"), readTestFile(t, path))
}

func TestGuardedCommitUpdatesBaselineOnlyAfterCommit(t *testing.T) {
	for _, outcome := range []string{"success", "replace failure", "post-replace failure", "directory sync failure"} {
		t.Run(outcome, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "vault.kdbx")
			require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
			p := newVaultPersistence()
			snapshot, err := p.read(path)
			require.NoError(t, err)
			baseline := snapshot.baseline
			before := baseline
			replacement := []byte("saved database")
			prepared, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
				_, err := w.Write(replacement)
				return err
			})
			require.NoError(t, err)
			t.Cleanup(func() { prepared.discard() })
			cause := errors.New("injected save failure")
			replace, syncDirectory := p.replace, p.syncDirectory
			switch outcome {
			case "replace failure":
				p.replace = func(string, string) (bool, error) { return false, cause }
			case "post-replace failure":
				p.replace = func(from, to string) (bool, error) {
					committed, err := replace(from, to)
					if err != nil || !committed {
						return committed, err
					}
					return true, cause
				}
			case "directory sync failure":
				p.syncDirectory = func(string) error { return cause }
			}
			committed, err := prepared.commitAndUpdateBaseline(path, &baseline)
			if outcome == "success" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, cause)
			}
			assert.Equal(t, outcome != "replace failure", committed)
			if !committed {
				assert.Equal(t, before, baseline)
				assert.Equal(t, []byte("original"), readTestFile(t, path))
				return
			}
			assert.Equal(t, sha256.Sum256(replacement), baseline.fingerprint)
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.True(t, os.SameFile(info, baseline.info))
			assert.False(t, os.SameFile(before.info, baseline.info))
			p.replace, p.syncDirectory = replace, syncDirectory
			// Own commits must not look like external replacements on the next save.
			next, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
				_, err := w.Write([]byte("next save"))
				return err
			})
			require.NoError(t, err)
			require.NoError(t, next.discard())
		})
	}
}

func TestBaselineAfterCommitDoesNotAdoptAnExternalWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	p := newVaultPersistence()
	snapshot, err := p.read(path)
	require.NoError(t, err)
	baseline := snapshot.baseline
	ours := []byte("our committed save")
	prepared, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
		_, err := w.Write(ours)
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	p.syncDirectory = func(string) error {
		return os.WriteFile(path, []byte("external edit after commit"), baseline.mode)
	}
	committed, err := prepared.commitAndUpdateBaseline(path, &baseline)
	require.NoError(t, err)
	require.True(t, committed)
	assert.Equal(t, sha256.Sum256(ours), baseline.fingerprint)
	requireSaveErrorKind(t, p.checkBaseline(path, baseline), SaveConflict)
	assert.Equal(t, []byte("external edit after commit"), readTestFile(t, path))
}

func TestGuardedCommitCannotRedirectToAnotherDestination(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	other := filepath.Join(filepath.Dir(path), "other.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	require.NoError(t, os.WriteFile(other, []byte("other database"), 0o600))
	p := newVaultPersistence()
	snapshot, err := p.read(path)
	require.NoError(t, err)
	baseline := snapshot.baseline
	prepared, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
		_, err := w.Write([]byte("pending save"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	committed, err := prepared.commitAndUpdateBaseline(other, &baseline)
	require.Error(t, err)
	assert.False(t, committed)
	assert.Equal(t, snapshot.baseline, baseline)
	assert.Equal(t, []byte("original"), readTestFile(t, path))
	assert.Equal(t, []byte("other database"), readTestFile(t, other))
}

func TestGuardedCommitPreservesConflictAndCleanupErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))
	p := newVaultPersistence()
	snapshot, err := p.read(path)
	require.NoError(t, err)
	baseline := snapshot.baseline
	prepared, err := p.prepareChecked(path, baseline, func(w io.Writer) error {
		_, err := w.Write([]byte("pending mutation"))
		return err
	})
	require.NoError(t, err)
	t.Cleanup(func() { prepared.discard() })
	require.NoError(t, os.WriteFile(path, []byte("external edit"), baseline.mode))
	cleanupErr := errors.New("temporary cleanup failed")
	remove := p.remove
	p.remove = func(string) error { return cleanupErr }
	committed, err := prepared.commitAndUpdateBaseline(path, &baseline)
	assert.False(t, committed)
	requireSaveErrorKind(t, err, SaveConflict)
	assert.ErrorIs(t, err, cleanupErr)
	assert.Equal(t, snapshot.baseline, baseline)
	assert.Equal(t, []byte("external edit"), readTestFile(t, path))
	assert.NotEmpty(t, prepared.path, "failed cleanup keeps the temporary file owned")
	p.remove = remove
	require.NoError(t, prepared.discard())
}
