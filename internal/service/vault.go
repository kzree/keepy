// Package service contains the core business logic and data management of the application
package service

import (
	"bytes"
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

	persistence *vaultPersistence

	// baseline describes the authenticated or last committed file. Saves update
	// it only when replacement commits, including durability-uncertain outcomes.
	baseline *vaultFileBaseline

	authenticated bool
	authError     error
}

func NewVault() *Vault {
	return &Vault{
		persistence:   newVaultPersistence(),
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
	snapshot, err := v.persistenceBackend().read(dbAbsPath)
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

		// The library's path-based key loader does not close its file in v3.6.2.
		// ReadFile owns and closes the handle, including on failed reads; the
		// byte-based parser preserves all supported KeePass key-file formats.
		keyData, err := os.ReadFile(keyFile)
		if err != nil {
			return v.handleAuthError(err)
		}
		creds, err := gokeepasslib.NewPasswordAndKeyDataCredentials(password, keyData)
		if err != nil {
			return v.handleAuthError(err)
		}
		db.Credentials = creds
		resolvedKeyFile = keyFile
	} else {
		db.Credentials = gokeepasslib.NewPasswordCredentials(password)
	}
	err = gokeepasslib.NewDecoder(bytes.NewReader(snapshot.data)).Decode(db)
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
	// Copy the baseline so its pointer does not retain the snapshot's data.
	baseline := snapshot.baseline
	v.baseline = &baseline
	v.password = password
	v.useKeyFile = useKeyFile

	return nil
}

// persistenceBackend requires v.mu to be held. The fallback supports vaults
// constructed without NewVault, including in-memory test fixtures.
func (v *Vault) persistenceBackend() *vaultPersistence {
	if v.persistence == nil {
		return newVaultPersistence()
	}
	return v.persistence
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

// Close forgets the decrypted database and credentials. Authenticate opens the
// vault again; until then, reads return nothing and mutations fail.
func (v *Vault) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.authenticated = false
	v.authError = nil
	v.db = nil
	v.password = ""
	v.baseline = nil
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
	working := copyVaultDatabase(v.db)
	working.Content.Root.Groups[0].Entries = append(working.Content.Root.Groups[0].Entries, newEntry)
	return v.saveWorkingDatabase(working)
}

func (v *Vault) UpdateEntry(id gokeepasslib.UUID, values NewVaultEntry) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}

	working := copyVaultDatabase(v.db)
	for i := range working.Content.Root.Groups {
		if updateEntryByID(&working.Content.Root.Groups[i], id, values) {
			return v.saveWorkingDatabase(working)
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

	working := copyVaultDatabase(v.db)
	for i := range working.Content.Root.Groups {
		found := v.findAndDeleteEntryByID(&working.Content.Root.Groups[i], id)
		if found {
			return v.saveWorkingDatabase(working)
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

// LockAndSave transactionally saves an authenticated vault without a CRUD
// mutation. Protected values remain unlocked in live state. A SaveError whose
// Committed method returns true means the file and live state already changed.
func (v *Vault) LockAndSave() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.lockAndSave()
}

// lockAndSave requires v.mu to be held for writing.
func (v *Vault) lockAndSave() error {
	if !v.authenticated {
		return errors.New("vault is not authenticated")
	}
	return v.saveWorkingDatabase(copyVaultDatabase(v.db))
}

// saveWorkingDatabase requires v.mu to be held for writing. Only the separate
// encoding copy is locked or changed by the encoder. Pre-commit failures preserve
// live state and the destination. Every committed result publishes the unlocked
// working state and baseline before releasing the lock, even on durability error.
// Callers must not automatically retry a mutation with a committed outcome.
func (v *Vault) saveWorkingDatabase(working *gokeepasslib.Database) error {
	encoding, err := encodingVaultCopy(working)
	if err != nil {
		return err
	}
	if v.baseline == nil {
		return &SaveError{Kind: SaveFailed, Op: "prepare database", Err: errors.New("database baseline is unavailable")}
	}
	prepared, err := v.persistenceBackend().prepareChecked(v.dbPath, *v.baseline, func(w io.Writer) error {
		return gokeepasslib.NewEncoder(w).Encode(encoding)
	})
	if err != nil {
		return uncommittedSaveError(err)
	}
	committed, err := prepared.commitAndUpdateBaseline(v.dbPath, v.baseline)
	if committed {
		v.db = working
		if err != nil {
			return &SaveError{Kind: SaveDurabilityUncertain, Op: "commit database", Err: err}
		}
		return nil
	}
	return uncommittedSaveError(err)
}

// uncommittedSaveError preserves guarded conflict classification and all joined
// cleanup causes; filesystem failures receive the transactional failure outcome.
func uncommittedSaveError(err error) error {
	var outcome *SaveError
	if errors.As(err, &outcome) {
		return err
	}
	return &SaveError{Kind: SaveFailed, Op: "prepare or replace database", Err: err}
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
