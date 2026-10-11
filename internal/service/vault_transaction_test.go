package service

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

func TestVaultTransactionsRollBackAndRetry(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "save"} {
		for _, failure := range []string{"create", "write", "sync", "close", "replace"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				entry := (&NewVaultEntry{Title: "Original", Password: "secret"}).ToKeepassEntry()
				vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root", Entries: []gokeepasslib.Entry{entry}})
				original := readTestFile(t, vault.dbPath)
				live := vault.db
				before := snapshotTestDatabase(t, live)
				baseline := *vault.baseline
				cause := errors.New("injected " + failure)
				p := vault.persistence
				create := p.createTemp
				p.createTemp = func(dir, pattern string) (vaultTempFile, error) {
					if failure == "create" {
						return nil, cause
					}
					file, err := create(dir, pattern)
					if err != nil {
						return nil, err
					}
					fault := &faultVaultTempFile{vaultTempFile: file}
					switch failure {
					case "write":
						fault.writeErr = cause
					case "sync":
						fault.syncErr = cause
					case "close":
						fault.closeErr = cause
					}
					return fault, nil
				}
				if failure == "replace" {
					p.replace = func(string, string) (bool, error) { return false, cause }
				}
				err := runTestVaultMutation(vault, operation, entry.UUID)
				requireSaveErrorKind(t, err, SaveFailed)
				require.ErrorIs(t, err, cause)
				assert.Same(t, live, vault.db)
				assert.Equal(t, before, snapshotTestDatabase(t, vault.db))
				assert.Equal(t, baseline, *vault.baseline)
				assert.Equal(t, original, readTestFile(t, vault.dbPath))
				assertEntryPassword(t, vault, entry.UUID, "secret")
				temps, err := filepath.Glob(filepath.Join(filepath.Dir(vault.dbPath), ".keepy-*.tmp"))
				require.NoError(t, err)
				assert.Empty(t, temps)
				vault.persistence = newVaultPersistence()
				require.NoError(t, runTestVaultMutation(vault, operation, entry.UUID))
				reopened := authenticateTestVault(t, vault.dbPath, "")
				assert.Equal(t, vault.GetEntriesFlat(), reopened.GetEntriesFlat())
				switch operation {
				case "add":
					require.Len(t, reopened.GetEntriesFlat(), 2)
					for _, added := range reopened.GetEntriesFlat() {
						if added.ID != entry.UUID {
							assertEntryPassword(t, vault, added.ID, "changed secret")
							assertEntryPassword(t, reopened, added.ID, "changed secret")
						}
					}
				case "update":
					assertEntryPassword(t, reopened, entry.UUID, "changed secret")
				case "delete":
					assert.Empty(t, reopened.GetEntriesFlat())
				case "save":
					assertEntryPassword(t, reopened, entry.UUID, "secret")
				}
			})
		}
	}
}

func TestVaultTransactionsPreserveExternalChanges(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "save"} {
		for _, checkpoint := range []string{"before preparation", "before replacement"} {
			t.Run(operation+"/"+checkpoint, func(t *testing.T) {
				entry := (&NewVaultEntry{Title: "Original", Password: "secret"}).ToKeepassEntry()
				vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root", Entries: []gokeepasslib.Entry{entry}})
				live := snapshotTestDatabase(t, vault.db)
				baseline := *vault.baseline
				external := []byte("external edit must survive")
				if checkpoint == "before preparation" {
					require.NoError(t, os.WriteFile(vault.dbPath, external, 0o600))
				} else {
					create := vault.persistence.createTemp
					vault.persistence.createTemp = func(dir, pattern string) (vaultTempFile, error) {
						if err := os.WriteFile(vault.dbPath, external, 0o600); err != nil {
							return nil, err
						}
						return create(dir, pattern)
					}
				}
				requireSaveErrorKind(t, runTestVaultMutation(vault, operation, entry.UUID), SaveConflict)
				assert.Equal(t, live, snapshotTestDatabase(t, vault.db))
				assert.Equal(t, baseline, *vault.baseline)
				assert.Equal(t, external, readTestFile(t, vault.dbPath))
				assertEntryPassword(t, vault, entry.UUID, "secret")
			})
		}
	}
}

func TestVaultTransactionsPublishOnCommittedErrors(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "save"} {
		for _, failure := range []string{"platform post-commit", "directory sync"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				entry := (&NewVaultEntry{Title: "Original", Password: "secret"}).ToKeepassEntry()
				vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root", Entries: []gokeepasslib.Entry{entry}})
				cause := errors.New("injected durability error")
				replacements := 0
				replace := vault.persistence.replace
				vault.persistence.replace = func(from, to string) (bool, error) {
					replacements++
					committed, err := replace(from, to)
					if committed && failure == "platform post-commit" {
						err = errors.Join(err, cause)
					}
					return committed, err
				}
				if failure == "directory sync" {
					vault.persistence.syncDirectory = func(string) error { return cause }
				}
				err := runTestVaultMutation(vault, operation, entry.UUID)
				requireSaveErrorKind(t, err, SaveDurabilityUncertain)
				require.ErrorIs(t, err, cause)
				assert.Equal(t, 1, replacements, "committed mutations must not be retried")
				reopened := authenticateTestVault(t, vault.dbPath, "")
				assert.Equal(t, reopened.GetEntriesFlat(), vault.GetEntriesFlat())
				assert.Equal(t, reopened.baseline.fingerprint, vault.baseline.fingerprint)
				assert.True(t, os.SameFile(reopened.baseline.info, vault.baseline.info))
				switch operation {
				case "add":
					require.Len(t, vault.GetEntriesFlat(), 2)
					assertEntryPassword(t, vault, entry.UUID, "secret")
					for _, added := range vault.GetEntriesFlat() {
						if added.ID != entry.UUID {
							assertEntryPassword(t, vault, added.ID, "changed secret")
							assertEntryPassword(t, reopened, added.ID, "changed secret")
						}
					}
				case "update":
					assertEntryPassword(t, vault, entry.UUID, "changed secret")
					assertEntryPassword(t, reopened, entry.UUID, "changed secret")
				case "delete":
					assert.Empty(t, vault.GetEntriesFlat())
				case "save":
					assertEntryPassword(t, vault, entry.UUID, "secret")
				}
				// A subsequent save must use the committed baseline without repeating CRUD.
				vault.persistence = newVaultPersistence()
				require.NoError(t, vault.LockAndSave())
				savedAgain := authenticateTestVault(t, vault.dbPath, "")
				assert.Equal(t, reopened.GetEntriesFlat(), savedAgain.GetEntriesFlat())
				for _, saved := range savedAgain.GetEntriesFlat() {
					password, err := reopened.GetEntryPassword(saved.ID)
					require.NoError(t, err)
					assertEntryPassword(t, savedAgain, saved.ID, password)
				}
			})
		}
	}
}

func TestVaultTransactionPreservesCleanupCause(t *testing.T) {
	vault := newPersistedTestVault(t, gokeepasslib.Group{Name: "Root"})
	live := snapshotTestDatabase(t, vault.db)
	baseline := *vault.baseline
	original := readTestFile(t, vault.dbPath)
	replaceCause := errors.New("replacement failure")
	cleanupCause := errors.New("cleanup failure")
	vault.persistence.replace = func(string, string) (bool, error) { return false, replaceCause }
	vault.persistence.remove = func(string) error { return cleanupCause }
	err := vault.AddNewEntry(NewVaultEntry{Title: "Pending", Password: "pending secret"})
	requireSaveErrorKind(t, err, SaveFailed)
	require.ErrorIs(t, err, replaceCause)
	require.ErrorIs(t, err, cleanupCause)
	assert.Equal(t, live, snapshotTestDatabase(t, vault.db))
	assert.Equal(t, baseline, *vault.baseline)
	assert.Equal(t, original, readTestFile(t, vault.dbPath))
	// Remove the deliberately retained temporary file after clearing the fault.
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(vault.dbPath), ".keepy-*.tmp"))
	require.NoError(t, err)
	require.Len(t, temps, 1)
	require.NoError(t, os.Remove(temps[0]))
}
