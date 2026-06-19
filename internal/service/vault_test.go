package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

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
