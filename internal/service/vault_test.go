package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
	"kzree.com/keepy/internal/util/testutil"
)

const testVaultPassword = "vault-password"

func TestNewVaultInitializesUnauthenticatedVault(t *testing.T) {
	vault := NewVault()

	assert.False(t, vault.authenticated)
	assert.NoError(t, vault.authError)
	assert.Empty(t, vault.dbPath)
	assert.Empty(t, vault.keyFilePath)
	assert.Empty(t, vault.password)
	assert.False(t, vault.useKeyFile)
	assert.Nil(t, vault.db)
}

func TestNewVaultEntryToKeepassEntryMapsFieldsAndProtectsPassword(t *testing.T) {
	input := NewVaultEntry{
		Title:    "Example",
		Username: "user@example.com",
		Password: "secret",
		URL:      "https://example.com",
	}

	entry := input.ToKeepassEntry()

	assert.NotEqual(t, gokeepasslib.UUID{}, entry.UUID)
	assert.Equal(t, input.Title, entry.GetTitle())
	assert.Equal(t, input.Username, entry.GetContent(UsernameKey))
	assert.Equal(t, input.Password, entry.GetContent(PasswordKey))
	assert.Equal(t, input.URL, entry.GetContent(URLKey))
	assert.True(t, entry.Get(PasswordKey).Value.Protected.Bool)
	assert.False(t, entry.Get("Title").Value.Protected.Bool)
	assert.False(t, entry.Get(UsernameKey).Value.Protected.Bool)
	assert.False(t, entry.Get(URLKey).Value.Protected.Bool)

	second := input.ToKeepassEntry()
	assert.NotEqual(t, entry.UUID, second.UUID)
}

func TestAuthenticatePasswordOnlyLoadsAndUnlocksDatabase(t *testing.T) {
	home := testutil.TempHome(t)
	entry := newTestEntry("Example", "user@example.com", "secret", "https://example.com")
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{entry},
	})
	vault := NewVault()

	err := vault.Authenticate("vault.kdbx", "", testVaultPassword, false)

	require.NoError(t, err)
	assert.True(t, vault.authenticated)
	assert.NoError(t, vault.authError)
	assert.Equal(t, "vault.kdbx", vault.dbPath)
	assert.Equal(t, testVaultPassword, vault.password)
	password, err := vault.GetEntryPassword(entry.UUID)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)
}

func TestAuthenticateWithPasswordAndKeyFile(t *testing.T) {
	home := testutil.TempHome(t)
	keyPath := filepath.Join(home, "vault.key")
	require.NoError(t, os.WriteFile(keyPath, bytes.Repeat([]byte{0x2a}, 32), 0o600))
	entry := newTestEntry("Example", "user@example.com", "secret", "https://example.com")
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, keyPath, gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{entry},
	})
	vault := NewVault()

	err := vault.Authenticate("vault.kdbx", "vault.key", testVaultPassword, true)

	require.NoError(t, err)
	password, err := vault.GetEntryPassword(entry.UUID)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)
}

func TestAuthenticateReturnsErrorForInvalidInputs(t *testing.T) {
	home := testutil.TempHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, "malformed.kdbx"), []byte("not a database"), 0o600))
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{Name: "Root"})

	tests := []struct {
		name        string
		dbPath      string
		keyFilePath string
		password    string
		useKeyFile  bool
	}{
		{name: "missing database", dbPath: "missing.kdbx", password: testVaultPassword},
		{name: "malformed database", dbPath: "malformed.kdbx", password: testVaultPassword},
		{name: "wrong password", dbPath: "vault.kdbx", password: "wrong-password"},
		{name: "missing key file", dbPath: "vault.kdbx", keyFilePath: "missing.key", password: testVaultPassword, useKeyFile: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vault := NewVault()

			err := vault.Authenticate(tt.dbPath, tt.keyFilePath, tt.password, tt.useKeyFile)

			require.Error(t, err)
			assert.False(t, vault.authenticated)
			assert.ErrorIs(t, vault.authError, err)
			assert.Nil(t, vault.GetEntriesFlat())
		})
	}
}

func TestFailedAuthenticationClearsPreviousAuthenticatedState(t *testing.T) {
	home := testutil.TempHome(t)
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{Name: "Root"})
	vault := NewVault()
	require.NoError(t, vault.Authenticate("vault.kdbx", "", testVaultPassword, false))

	err := vault.Authenticate("vault.kdbx", "", "wrong-password", false)

	require.Error(t, err)
	assert.False(t, vault.authenticated)
	assert.ErrorIs(t, vault.authError, err)
}

func TestReAuthenticateUsesOriginalKeyFileChoice(t *testing.T) {
	home := testutil.TempHome(t)
	entry := newTestEntry("Example", "user@example.com", "secret", "https://example.com")
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{entry},
	})
	vault := NewVault()
	require.NoError(t, vault.Authenticate("vault.kdbx", "unused.key", testVaultPassword, false))
	vault.db = nil

	err := vault.ReAuthenticate()

	require.NoError(t, err)
	password, err := vault.GetEntryPassword(entry.UUID)
	require.NoError(t, err)
	assert.Equal(t, "secret", password)
}

func TestGetEntriesFlatUnauthenticatedReturnsNil(t *testing.T) {
	vault := NewVault()

	entries := vault.GetEntriesFlat()

	assert.Nil(t, entries)
}

func TestGetEntriesFlatCollectsNestedEntriesAndSortsByTitle(t *testing.T) {
	beta := newTestEntry("Beta", "beta@example.com", "beta-password", "https://beta.example")
	alpha := newTestEntry("Alpha", "alpha@example.com", "alpha-password", "https://alpha.example")
	vault := newTestVault(gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{beta},
		Groups: []gokeepasslib.Group{
			{
				Name:    "Nested",
				Entries: []gokeepasslib.Entry{alpha},
			},
		},
	})

	entries := vault.GetEntriesFlat()

	require.Len(t, entries, 2)
	assert.Equal(t, alpha.UUID, entries[0].ID)
	assert.Equal(t, "Alpha", entries[0].Title)
	assert.Equal(t, "alpha@example.com", entries[0].Username)
	assert.Equal(t, "https://alpha.example", entries[0].URL)
	assert.Equal(t, "Nested", entries[0].Group)
	assert.Equal(t, beta.UUID, entries[1].ID)
	assert.Equal(t, "Beta", entries[1].Title)
	assert.Equal(t, "Root", entries[1].Group)
}

func TestGetEntryPasswordFindsNestedEntryByID(t *testing.T) {
	entry := newTestEntry("Secret", "secret@example.com", "correct horse battery staple", "https://secret.example")
	vault := newTestVault(gokeepasslib.Group{
		Name: "Root",
		Groups: []gokeepasslib.Group{
			{
				Name:    "Nested",
				Entries: []gokeepasslib.Entry{entry},
			},
		},
	})

	password, err := vault.GetEntryPassword(entry.UUID)

	require.NoError(t, err)
	assert.Equal(t, "correct horse battery staple", password)
}

func TestGetEntryPasswordReturnsEmptyForMissingOrUnauthenticatedVault(t *testing.T) {
	vault := newTestVault(gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{newTestEntry("Present", "present@example.com", "password", "https://present.example")},
	})

	password, err := vault.GetEntryPassword(gokeepasslib.NewUUID())
	require.NoError(t, err)
	assert.Empty(t, password)

	password, err = NewVault().GetEntryPassword(gokeepasslib.NewUUID())
	require.NoError(t, err)
	assert.Empty(t, password)
}

func TestAddNewEntryReturnsErrorWhenVaultCannotAcceptEntry(t *testing.T) {
	t.Run("unauthenticated", func(t *testing.T) {
		err := NewVault().AddNewEntry(NewVaultEntry{Title: "Example"})

		assert.EqualError(t, err, "vault is not authenticated")
	})

	t.Run("no groups", func(t *testing.T) {
		err := newTestVault().AddNewEntry(NewVaultEntry{Title: "Example"})

		assert.EqualError(t, err, "no groups found to save entry to")
	})
}

func TestAddNewEntryPersistsEntriesAndPreservesPasswordsAcrossSaves(t *testing.T) {
	home := testutil.TempHome(t)
	original := newTestEntry("Original", "original@example.com", "original-secret", "https://original.example")
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{original},
	})
	vault := NewVault()
	require.NoError(t, vault.Authenticate("vault.kdbx", "", testVaultPassword, false))

	require.NoError(t, vault.AddNewEntry(NewVaultEntry{
		Title: "Second", Username: "second@example.com", Password: "second-secret", URL: "https://second.example",
	}))
	entries := vault.GetEntriesFlat()
	require.Len(t, entries, 2)
	assertEntryPassword(t, vault, original.UUID, "original-secret")
	secondID := entries[1].ID
	assertEntryPassword(t, vault, secondID, "second-secret")

	require.NoError(t, vault.AddNewEntry(NewVaultEntry{
		Title: "Third", Username: "third@example.com", Password: "third-secret", URL: "https://third.example",
	}))

	reopened := NewVault()
	require.NoError(t, reopened.Authenticate("vault.kdbx", "", testVaultPassword, false))
	entries = reopened.GetEntriesFlat()
	require.Len(t, entries, 3)
	assertEntryPassword(t, reopened, original.UUID, "original-secret")
	assertEntryPassword(t, reopened, secondID, "second-secret")
	assertEntryPassword(t, reopened, entries[2].ID, "third-secret")
}

func TestUpdateEntryReturnsErrorsForUnauthenticatedAndMissingEntries(t *testing.T) {
	values := NewVaultEntry{Title: "Updated"}
	assert.EqualError(t, NewVault().UpdateEntry(gokeepasslib.NewUUID(), values), "vault is not authenticated")

	vault := newTestVault(gokeepasslib.Group{Name: "Root"})
	assert.EqualError(t, vault.UpdateEntry(gokeepasslib.NewUUID(), values), "failed to find entry to update")
}

func TestUpdateEntryUpdatesNestedEntryAndPersists(t *testing.T) {
	home := testutil.TempHome(t)
	original := newTestEntry("Original", "original@example.com", "original-secret", "https://original.example")
	original.Get(PasswordKey).Value.Protected = wrappers.NewBoolWrapper(true)
	original.Values = append(original.Values, gokeepasslib.ValueData{
		Key:   "Notes",
		Value: gokeepasslib.V{Content: "preserve me"},
	})
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{
		Name: "Root",
		Groups: []gokeepasslib.Group{{
			Name:    "Nested",
			Entries: []gokeepasslib.Entry{original},
		}},
	})
	vault := NewVault()
	require.NoError(t, vault.Authenticate("vault.kdbx", "", testVaultPassword, false))
	updated := NewVaultEntry{
		Title:    "Updated",
		Username: "updated@example.com",
		Password: "updated-secret",
		URL:      "https://updated.example",
	}

	require.NoError(t, vault.UpdateEntry(original.UUID, updated))

	reopened := NewVault()
	require.NoError(t, reopened.Authenticate("vault.kdbx", "", testVaultPassword, false))
	entries := reopened.GetEntriesFlat()
	require.Len(t, entries, 1)
	assert.Equal(t, original.UUID, entries[0].ID)
	assert.Equal(t, updated.Title, entries[0].Title)
	assert.Equal(t, updated.Username, entries[0].Username)
	assert.Equal(t, updated.URL, entries[0].URL)
	assert.Equal(t, "Nested", entries[0].Group)
	assertEntryPassword(t, reopened, original.UUID, updated.Password)
	assert.True(t, reopened.db.Content.Root.Groups[0].Groups[0].Entries[0].Get(PasswordKey).Value.Protected.Bool)
	assert.Equal(t, "preserve me", reopened.db.Content.Root.Groups[0].Groups[0].Entries[0].GetContent("Notes"))
}

func TestDeleteEntryReturnsErrorsForUnauthenticatedAndMissingEntries(t *testing.T) {
	assert.EqualError(t, NewVault().DeleteEntry(gokeepasslib.NewUUID()), "vault is not authenticated")

	vault := newTestVault(gokeepasslib.Group{Name: "Root"})
	assert.EqualError(t, vault.DeleteEntry(gokeepasslib.NewUUID()), "failed to find entry to delete")
}

func TestDeleteEntryRemovesNestedEntryAndPersists(t *testing.T) {
	home := testutil.TempHome(t)
	keep := newTestEntry("Keep", "keep@example.com", "keep-secret", "https://keep.example")
	remove := newTestEntry("Remove", "remove@example.com", "remove-secret", "https://remove.example")
	writeTestDatabase(t, filepath.Join(home, "vault.kdbx"), testVaultPassword, "", gokeepasslib.Group{
		Name:    "Root",
		Entries: []gokeepasslib.Entry{keep},
		Groups: []gokeepasslib.Group{{
			Name:    "Nested",
			Entries: []gokeepasslib.Entry{remove},
		}},
	})
	vault := NewVault()
	require.NoError(t, vault.Authenticate("vault.kdbx", "", testVaultPassword, false))

	require.NoError(t, vault.DeleteEntry(remove.UUID))

	reopened := NewVault()
	require.NoError(t, reopened.Authenticate("vault.kdbx", "", testVaultPassword, false))
	entries := reopened.GetEntriesFlat()
	require.Len(t, entries, 1)
	assert.Equal(t, keep.UUID, entries[0].ID)
	assertEntryPassword(t, reopened, keep.UUID, "keep-secret")
}

func TestFindAndDeleteEntryByIDRemovesDirectEntry(t *testing.T) {
	remove := newTestEntry("Remove", "", "", "")
	keep := newTestEntry("Keep", "", "", "")
	group := gokeepasslib.Group{Entries: []gokeepasslib.Entry{remove, keep}}

	found := NewVault().findAndDeleteEntryByID(&group, remove.UUID)

	assert.True(t, found)
	require.Len(t, group.Entries, 1)
	assert.Equal(t, keep.UUID, group.Entries[0].UUID)
}

func TestLockAndSaveReturnsPersistenceErrors(t *testing.T) {
	t.Run("lock protected entries", func(t *testing.T) {
		db := gokeepasslib.NewDatabase()
		db.Header.Signature = nil
		vault := &Vault{db: db}

		require.Error(t, vault.LockAndSave())
	})

	t.Run("create database file", func(t *testing.T) {
		testutil.TempHome(t)
		db := gokeepasslib.NewDatabase()
		vault := &Vault{db: db, dbPath: filepath.Join("missing", "vault.kdbx")}

		require.Error(t, vault.LockAndSave())
	})

	t.Run("encode database", func(t *testing.T) {
		testutil.TempHome(t)
		db := gokeepasslib.NewDatabase()
		db.Credentials = nil
		vault := &Vault{db: db, dbPath: "vault.kdbx"}

		require.Error(t, vault.LockAndSave())
	})
}

func newTestVault(groups ...gokeepasslib.Group) *Vault {
	db := gokeepasslib.NewDatabase()
	db.Content.Root.Groups = groups

	return &Vault{
		db:            db,
		authenticated: true,
	}
}

func newTestEntry(title, username, password, url string) gokeepasslib.Entry {
	entry := gokeepasslib.NewEntry()
	entry.Values = []gokeepasslib.ValueData{
		{Key: "Title", Value: gokeepasslib.V{Content: title}},
		{Key: UsernameKey, Value: gokeepasslib.V{Content: username}},
		{Key: PasswordKey, Value: gokeepasslib.V{Content: password}},
		{Key: URLKey, Value: gokeepasslib.V{Content: url}},
	}
	return entry
}

func writeTestDatabase(t *testing.T, path, password, keyFilePath string, groups ...gokeepasslib.Group) {
	t.Helper()
	db := gokeepasslib.NewDatabase()
	if keyFilePath == "" {
		db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	} else {
		credentials, err := gokeepasslib.NewPasswordAndKeyCredentials(password, keyFilePath)
		require.NoError(t, err)
		db.Credentials = credentials
	}
	db.Content.Root.Groups = groups
	require.NoError(t, db.LockProtectedEntries())

	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, gokeepasslib.NewEncoder(file).Encode(db))
	require.NoError(t, file.Close())
}

func assertEntryPassword(t *testing.T, vault *Vault, id gokeepasslib.UUID, want string) {
	t.Helper()
	got, err := vault.GetEntryPassword(id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}
