// Package service contains the core business logic and data management of the application
package service

import (
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
	"kzree.com/keepy/internal/util"
)

const (
	UsernameKey = "UserName"
	PasswordKey = "Password"
	URLKey      = "URL"
)

type VaultEntry struct {
	ID       gokeepasslib.UUID
	Title    string
	Username string
	URL      string
	Group    string
}

type NewVaultEntry struct {
	Title    string
	Username string
	URL      string
	Password string
}

func (e *NewVaultEntry) ToKeepassEntry() gokeepasslib.Entry {
	newEntry := gokeepasslib.NewEntry()
	newEntry.Values = append(newEntry.Values,
		gokeepasslib.ValueData{
			Key:   "Title",
			Value: gokeepasslib.V{Content: e.Title},
		},
		gokeepasslib.ValueData{
			Key:   UsernameKey,
			Value: gokeepasslib.V{Content: e.Username},
		},
		gokeepasslib.ValueData{
			Key:   PasswordKey,
			Value: gokeepasslib.V{Content: e.Password, Protected: wrappers.NewBoolWrapper(true)},
		},
		gokeepasslib.ValueData{
			Key:   URLKey,
			Value: gokeepasslib.V{Content: e.URL},
		},
	)

	return newEntry
}

type Vault struct {
	dbPath      string
	keyFilePath string
	password    string

	db *gokeepasslib.Database

	authenticated bool
	authError     error
}

func NewVault() *Vault {
	return &Vault{
		dbPath:        "",
		keyFilePath:   "",
		password:      "",
		authenticated: false,
		authError:     nil,
	}
}

func (v *Vault) handleAuthError(err error) error {
	v.authenticated = false
	v.authError = err
	return err
}

func (v *Vault) Authenticate(dbPath, keyFilePath, password string, useKeyFile bool) error {
	dbAbsPath, err := util.PathFromHome(dbPath)
	if err != nil {
		return v.handleAuthError(err)
	}
	file, err := os.Open(dbAbsPath)
	if err != nil {
		return v.handleAuthError(err)
	}
	defer file.Close()

	db := gokeepasslib.NewDatabase()

	if useKeyFile {
		keyFile, err := util.PathFromHome(keyFilePath)
		if err != nil {
			return v.handleAuthError(err)
		}

		creds, err := gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
		if err != nil {
			return v.handleAuthError(err)
		}
		db.Credentials = creds
	} else {
		db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	}
	err = gokeepasslib.NewDecoder(file).Decode(db)
	if err != nil {
		return v.handleAuthError(err)
	}

	err = db.UnlockProtectedEntries()
	if err != nil {
		return v.handleAuthError(err)
	}

	v.authenticated = true
	v.authError = nil
	v.db = db
	v.dbPath = dbPath
	v.keyFilePath = keyFilePath
	v.password = password

	return nil
}

func (v *Vault) ReAuthenticate() error {
	return v.Authenticate(v.dbPath, v.keyFilePath, v.password, v.keyFilePath != "")
}

func (v *Vault) GetEntriesFlat() []VaultEntry {
	if !v.authenticated {
		return nil
	}
	var entries []VaultEntry
	for _, group := range v.db.Content.Root.Groups {
		entries = append(entries, collectEntries(group)...)
	}
	sort.Slice(entries, func(i, j int) bool {
		return strings.ToLower(entries[i].Title) < strings.ToLower(entries[j].Title)
	})
	return entries
}

func (v *Vault) GetEntryPassword(id gokeepasslib.UUID) (string, error) {
	if !v.authenticated {
		return "", nil
	}

	for _, group := range v.db.Content.Root.Groups {
		password, found := findEntryPassword(group, id)
		if found {
			return password, nil
		}
	}
	return "", nil
}

func (v *Vault) AddNewEntry(entry NewVaultEntry) error {
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	if len(v.db.Content.Root.Groups) == 0 {
		return errors.New("no groups found to save entry to")
	}

	newEntry := entry.ToKeepassEntry()

	v.db.Content.Root.Groups[0].Entries = append(v.db.Content.Root.Groups[0].Entries, newEntry)
	return v.LockAndSave()
}

func (v *Vault) DeleteEntry(id gokeepasslib.UUID) error {
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	for i := range v.db.Content.Root.Groups {
		found := v.findAndDeleteEntryByID(&v.db.Content.Root.Groups[i], id)
		if found {
			return v.LockAndSave()
		}
	}

	return errors.New("failed to find entry to delete")
}

func (v *Vault) findAndDeleteEntryByID(group *gokeepasslib.Group, id gokeepasslib.UUID) bool {
	for i, entry := range group.Entries {
		if entry.UUID.Compare(id) {
			group.Entries = append(group.Entries[:i], group.Entries[i+1:]...)
			return true
		}
	}

	for i := range group.Groups {
		found := v.findAndDeleteEntryByID(&group.Groups[i], id)
		if found {
			return true
		}
	}

	return false
}

func (v *Vault) LockAndSave() error {
	if err := v.db.LockProtectedEntries(); err != nil {
		return err
	}
	dbAbsPath, _ := util.PathFromHome(v.dbPath)
	file, err := os.Create(dbAbsPath)
	if err != nil {
		return err
	}
	defer file.Close()
	return gokeepasslib.NewEncoder(file).Encode(v.db)
}

func findEntryPassword(group gokeepasslib.Group, id gokeepasslib.UUID) (string, bool) {
	for _, entry := range group.Entries {
		if entry.UUID.Compare(id) {
			return entry.GetContent(PasswordKey), true
		}
	}
	for _, subgroup := range group.Groups {
		password, found := findEntryPassword(subgroup, id)
		if found {
			return password, true
		}
	}
	return "", false
}

func collectEntries(group gokeepasslib.Group) []VaultEntry {
	var entries []VaultEntry
	for _, entry := range group.Entries {
		entries = append(entries, VaultEntry{
			ID:       entry.UUID,
			Title:    entry.GetTitle(),
			Username: entry.GetContent(UsernameKey),
			URL:      entry.GetContent(URLKey),
			Group:    group.Name,
		})
	}
	for _, subgroup := range group.Groups {
		entries = append(entries, collectEntries(subgroup)...)
	}
	return entries
}
