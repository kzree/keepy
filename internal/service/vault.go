// Package service contains the core business logic and data management of the application
package service

import (
	"os"

	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/util"
)

type VaultEntry struct {
	Title    string
	Username string
	Password string
	URL      string
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
	db.Credentials, _ = gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
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
		for _, subgroup := range group.Groups {
			for _, entry := range subgroup.Entries {
				entries = append(entries, VaultEntry{
					Title:    entry.GetTitle(),
					Username: entry.GetContent("UserName"),
					Password: entry.GetContent("Password"),
					URL:      entry.GetContent("URL"),
				})
			}
		}

		for _, entry := range group.Entries {
			entries = append(entries, VaultEntry{
				Title:    entry.GetTitle(),
				Username: entry.GetContent("UserName"),
				Password: entry.GetContent("Password"),
				URL:      entry.GetContent("URL"),
			})
		}
	}

	return entries
}
