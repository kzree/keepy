// Package service contains the core business logic and data management of the application
package service

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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

// Vault owns one database session. Public methods serialize access to its state.
// A Vault must not be copied after first use; share the pointer from NewVault.
type Vault struct {
	mu sync.RWMutex

	dbPath      string
	keyFilePath string
	password    string
	useKeyFile  bool

	db *gokeepasslib.Database

	// baseline describes the exact bytes loaded during authentication. Future
	// transactional saves will replace it only when the replacement commits.
	baseline *vaultFileBaseline

	authenticated bool
	authError     error
}

type vaultFileBaseline struct {
	fingerprint [sha256.Size]byte
	mode        os.FileMode
}

func NewVault() *Vault {
	return &Vault{
		dbPath:        "",
		keyFilePath:   "",
		password:      "",
		useKeyFile:    false,
		authenticated: false,
		authError:     nil,
	}
}

// handleAuthError requires v.mu to be held for writing.
func (v *Vault) handleAuthError(err error) error {
	v.authenticated = false
	v.authError = err
	v.baseline = nil
	return err
}

func (v *Vault) Authenticate(dbPath, keyFilePath, password string, useKeyFile bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.authenticate(dbPath, keyFilePath, password, useKeyFile)
}

// authenticate requires v.mu to be held for writing.
func (v *Vault) authenticate(dbPath, keyFilePath, password string, useKeyFile bool) error {
	dbAbsPath, err := resolveVaultPath(dbPath)
	if err != nil {
		return v.handleAuthError(err)
	}
	file, err := os.Open(dbAbsPath)
	if err != nil {
		return v.handleAuthError(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return v.handleAuthError(err)
	}
	if !info.Mode().IsRegular() {
		return v.handleAuthError(errors.New("database path must refer to a regular file"))
	}
	// Decode and fingerprint the same snapshot, not two separate reads of a
	// path that another client may replace between reads.
	data, err := io.ReadAll(file)
	if err != nil {
		return v.handleAuthError(err)
	}
	db := gokeepasslib.NewDatabase()
	resolvedKeyFile := keyFilePath

	if useKeyFile {
		keyFile, err := resolveVaultPath(keyFilePath)
		if err != nil {
			return v.handleAuthError(err)
		}

		creds, err := gokeepasslib.NewPasswordAndKeyCredentials(password, keyFile)
		if err != nil {
			return v.handleAuthError(err)
		}
		db.Credentials = creds
		resolvedKeyFile = keyFile
	} else {
		db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	}
	err = gokeepasslib.NewDecoder(bytes.NewReader(data)).Decode(db)
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
	v.dbPath = dbAbsPath
	v.keyFilePath = resolvedKeyFile
	v.baseline = &vaultFileBaseline{
		fingerprint: sha256.Sum256(data),
		mode:        info.Mode().Perm(),
	}
	v.password = password
	v.useKeyFile = useKeyFile

	return nil
}

// resolveVaultPath pins symlinks (including parent-directory symlinks) to their
// current target so later writes and reauthentication use the opened location.
func resolveVaultPath(path string) (string, error) {
	expanded, err := util.PathFromHome(path)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func (v *Vault) ReAuthenticate() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.authenticate(v.dbPath, v.keyFilePath, v.password, v.useKeyFile)
}

func (v *Vault) GetEntriesFlat() []VaultEntry {
	v.mu.RLock()
	defer v.mu.RUnlock()
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
	v.mu.RLock()
	defer v.mu.RUnlock()
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
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	if len(v.db.Content.Root.Groups) == 0 {
		return errors.New("no groups found to save entry to")
	}

	newEntry := entry.ToKeepassEntry()

	v.db.Content.Root.Groups[0].Entries = append(v.db.Content.Root.Groups[0].Entries, newEntry)
	return v.lockAndSave()
}

func (v *Vault) UpdateEntry(id gokeepasslib.UUID, values NewVaultEntry) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	for i := range v.db.Content.Root.Groups {
		if updateEntryByID(&v.db.Content.Root.Groups[i], id, values) {
			return v.lockAndSave()
		}
	}

	return errors.New("failed to find entry to update")
}

func updateEntryByID(group *gokeepasslib.Group, id gokeepasslib.UUID, values NewVaultEntry) bool {
	for i := range group.Entries {
		entry := &group.Entries[i]
		if entry.UUID.Compare(id) {
			setEntryValue(entry, "Title", values.Title)
			setEntryValue(entry, UsernameKey, values.Username)
			setEntryValue(entry, PasswordKey, values.Password)
			setEntryValue(entry, URLKey, values.URL)
			return true
		}
	}

	for i := range group.Groups {
		if updateEntryByID(&group.Groups[i], id, values) {
			return true
		}
	}

	return false
}

func setEntryValue(entry *gokeepasslib.Entry, key, content string) {
	value := entry.Get(key)
	if value != nil {
		value.Value.Content = content
		return
	}

	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key: key,
		Value: gokeepasslib.V{
			Content:   content,
			Protected: wrappers.NewBoolWrapper(key == PasswordKey),
		},
	})
}

func (v *Vault) DeleteEntry(id gokeepasslib.UUID) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	for i := range v.db.Content.Root.Groups {
		found := v.findAndDeleteEntryByID(&v.db.Content.Root.Groups[i], id)
		if found {
			return v.lockAndSave()
		}
	}

	return errors.New("failed to find entry to delete")
}

// findAndDeleteEntryByID requires v.mu to be held for writing when group belongs
// to the live database.
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
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.lockAndSave()
}

// lockAndSave requires v.mu to be held for writing. This legacy implementation
// does not yet provide transactional rollback or atomic file replacement.
func (v *Vault) lockAndSave() error {
	if err := v.db.LockProtectedEntries(); err != nil {
		return err
	}
	file, err := os.Create(v.dbPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := gokeepasslib.NewEncoder(file).Encode(v.db); err != nil {
		return err
	}

	return v.db.UnlockProtectedEntries()
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
