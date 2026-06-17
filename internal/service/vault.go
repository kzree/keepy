// Package service contains the core business logic and data management of the application
package service

import (
	"os"
	"sort"
	"strings"

	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/util"
)

const (
	UsernameKey = "UserName"
	PasswordKey = "Password"
	URLKey      = "URL"
)

type VaultEntry struct {
	Title    string
	Username string
	Password string
	URL      string
	Group    string
}

type Vault struct {
	dbPath      string
	keyFilePath string
	password    string

	db *gokeepasslib.Database

	authenticated bool
	authError     *error
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

func (v *Vault) Authenticate(dbPath, keyFilePath, password string, useKeyFile bool) error {
	dbAbsPath, _ := util.PathFromHome(dbPath)
	file, _ := os.Open(dbAbsPath)

	db := gokeepasslib.NewDatabase()

	keyFile, _ := util.PathFromHome(keyFilePath)

	if useKeyFile {
		creds, err := gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
		if err != nil {
			v.authenticated = false
			v.authError = &err
			return err
		}
		db.Credentials = creds
	} else {
		db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	}
	_ = gokeepasslib.NewDecoder(file).Decode(db)

	err := db.UnlockProtectedEntries()
	if err != nil {
		v.authenticated = false
		v.authError = &err
		return err
	}

	v.authenticated = true
	v.authError = nil
	v.db = db

	return nil
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

func collectEntries(group gokeepasslib.Group) []VaultEntry {
	var entries []VaultEntry
	for _, entry := range group.Entries {
		entries = append(entries, VaultEntry{
			Title:    entry.GetTitle(),
			Username: entry.GetContent(UsernameKey),
			Password: entry.GetContent(PasswordKey),
			URL:      entry.GetContent(URLKey),
			Group:    group.Name,
		})
	}
	for _, subgroup := range group.Groups {
		entries = append(entries, collectEntries(subgroup)...)
	}
	return entries
}
