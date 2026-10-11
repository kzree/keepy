package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"github.com/tobischo/gokeepasslib/v3/wrappers"
	"kzree.com/keepy/internal/util/testutil"
)

// newTestVault creates an in-memory vault for tests that do not save to disk.
func newTestVault(groups ...gokeepasslib.Group) *Vault {
	db := gokeepasslib.NewDatabase()
	db.Content.Root.Groups = groups
	return &Vault{db: db, authenticated: true}
}

// newPersistedTestVault creates and opens a password-only database under a
// temporary home directory. Tests of authentication itself use explicit setup.
func newPersistedTestVault(t *testing.T, groups ...gokeepasslib.Group) *Vault {
	t.Helper()
	home := testutil.TempHome(t)
	path := filepath.Join(home, "vault.kdbx")
	writeTestDatabase(t, path, testVaultPassword, "", groups...)
	return authenticateTestVault(t, "vault.kdbx", "")
}

func authenticateTestVault(t *testing.T, dbPath, keyFilePath string) *Vault {
	t.Helper()
	vault := NewVault()
	require.NoError(t, vault.Authenticate(dbPath, keyFilePath, testVaultPassword, keyFilePath != ""))
	return vault
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
		credentials, err := gokeepasslib.NewPasswordAndKeyDataCredentials(password, readTestFile(t, keyFilePath))
		require.NoError(t, err)
		db.Credentials = credentials
	}
	db.Content.Root.Groups = groups
	require.NoError(t, db.LockProtectedEntries())

	file, err := os.Create(path)
	require.NoError(t, err)
	defer file.Close()
	require.NoError(t, gokeepasslib.NewEncoder(file).Encode(db))
	require.NoError(t, file.Close())
}

func writeTestKeyFile(t *testing.T, path string, value byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte{value}, 32), 0o600))
}

// writeTestVaultDatabase persists an encoding copy without changing the source.
func writeTestVaultDatabase(t *testing.T, path string, db *gokeepasslib.Database) {
	t.Helper()
	encoding, err := encodingVaultCopy(db)
	require.NoError(t, err)
	var data bytes.Buffer
	require.NoError(t, gokeepasslib.NewEncoder(&data).Encode(encoding))
	require.NoError(t, os.WriteFile(path, data.Bytes(), 0o600))
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func assertNoVaultTemps(t *testing.T, path string) {
	t.Helper()
	temps, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".keepy-*.tmp"))
	require.NoError(t, err)
	assert.Empty(t, temps)
}

// assertVaultFormatAndMetadata compares stable fields after decoding, excluding
// encoder-generated raw bytes, header hashes, random seeds, IVs, and KDF salt.
func assertVaultFormatAndMetadata(t *testing.T, before, after *gokeepasslib.Database) {
	t.Helper()
	assert.Equal(t, before.Header.Signature, after.Header.Signature)
	b, a := before.Header.FileHeaders, after.Header.FileHeaders
	assert.Equal(t, b.CipherID, a.CipherID)
	assert.Equal(t, b.CompressionFlags, a.CompressionFlags)
	assert.Equal(t, b.TransformRounds, a.TransformRounds)
	assert.Equal(t, b.InnerRandomStreamID, a.InnerRandomStreamID)
	if b.KdfParameters != nil {
		require.NotNil(t, a.KdfParameters)
		kb, ka := b.KdfParameters, a.KdfParameters
		assert.Equal(t, kb.UUID, ka.UUID)
		assert.Equal(t, kb.Rounds, ka.Rounds)
		assert.Equal(t, kb.Parallelism, ka.Parallelism)
		assert.Equal(t, kb.Memory, ka.Memory)
		assert.Equal(t, kb.Iterations, ka.Iterations)
		assert.Equal(t, kb.Version, ka.Version)
		assert.Equal(t, kb.SecretKey, ka.SecretKey)
		assert.Equal(t, kb.AssocData, ka.AssocData)
	} else {
		assert.Nil(t, a.KdfParameters)
	}
	mb, ma := before.Content.Meta, after.Content.Meta
	assert.Equal(t, mb.DatabaseName, ma.DatabaseName)
	assert.Equal(t, mb.DatabaseDescription, ma.DatabaseDescription)
	assert.Equal(t, mb.DefaultUserName, ma.DefaultUserName)
	assert.Equal(t, mb.SettingsChanged, ma.SettingsChanged)
	assert.Equal(t, mb.DatabaseNameChanged, ma.DatabaseNameChanged)
	assert.Equal(t, mb.DatabaseDescriptionChanged, ma.DatabaseDescriptionChanged)
	assert.Equal(t, mb.DefaultUserNameChanged, ma.DefaultUserNameChanged)
	assert.Equal(t, mb.MasterKeyChanged, ma.MasterKeyChanged)
	assert.Equal(t, mb.RecycleBinChanged, ma.RecycleBinChanged)
	assert.Equal(t, mb.EntryTemplatesGroupChanged, ma.EntryTemplatesGroupChanged)
	assert.Equal(t, mb.RecycleBinEnabled, ma.RecycleBinEnabled)
	assert.Equal(t, mb.RecycleBinUUID, ma.RecycleBinUUID)
	assert.Equal(t, mb.CustomIcons, ma.CustomIcons)
	assert.Equal(t, mb.CustomData, ma.CustomData)
}

func resolvedTestPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

func symlinkTestFile(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	require.NoError(t, err)
}

// runConcurrentTestOperations releases all workers from one start gate and
// checks their results on the test goroutine after every worker has finished.
// Operations return errors rather than calling require, which uses FailNow.
func runConcurrentTestOperations(t *testing.T, operations ...func() error) {
	t.Helper()
	start := make(chan struct{})
	errs := make([]error, len(operations))
	var workers sync.WaitGroup
	for i, operation := range operations {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			errs[i] = operation()
		}()
	}
	close(start)
	workers.Wait()
	for i, err := range errs {
		require.NoError(t, err, "concurrent operation %d", i)
	}
}

func assertEntryPassword(t *testing.T, vault *Vault, id gokeepasslib.UUID, want string) {
	t.Helper()
	got, err := vault.GetEntryPassword(id)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func runTestVaultMutation(vault *Vault, operation string, id gokeepasslib.UUID) error {
	values := NewVaultEntry{Title: "Changed", Username: "changed user", Password: "changed secret"}
	switch operation {
	case "add":
		return vault.AddNewEntry(values)
	case "update":
		return vault.UpdateEntry(id, values)
	case "delete":
		return vault.DeleteEntry(id)
	default:
		return vault.LockAndSave()
	}
}

// faultVaultTempFile delegates to a real temporary file except at the selected
// failure points, keeping tests independent of host filesystem restrictions.
type faultVaultTempFile struct {
	vaultTempFile
	writeErr   error
	writeOnce  bool
	shortWrite bool
	chmodErr   error
	statErr    error
	syncErr    error
	closeErr   error
	closes     int
}

func captureTestTempFile(p *vaultPersistence) *faultVaultTempFile {
	file := &faultVaultTempFile{}
	create := p.createTemp
	p.createTemp = func(dir, pattern string) (vaultTempFile, error) {
		created, err := create(dir, pattern)
		if err != nil {
			return nil, err
		}
		file.vaultTempFile = created
		return file, nil
	}
	return file
}

func (f *faultVaultTempFile) Write(data []byte) (int, error) {
	if f.shortWrite {
		f.shortWrite = false
		return f.vaultTempFile.Write(data[:len(data)/2])
	}
	if f.writeErr != nil {
		cause := f.writeErr
		if f.writeOnce {
			f.writeErr = nil
		}
		n, err := f.vaultTempFile.Write(data[:len(data)/2])
		return n, errors.Join(err, cause)
	}
	return f.vaultTempFile.Write(data)
}

func (f *faultVaultTempFile) Chmod(mode os.FileMode) error {
	if f.chmodErr != nil {
		return f.chmodErr
	}
	return f.vaultTempFile.Chmod(mode)
}

func (f *faultVaultTempFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.vaultTempFile.Sync()
}

func (f *faultVaultTempFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.vaultTempFile.Stat()
}

func (f *faultVaultTempFile) Close() error {
	f.closes++
	return errors.Join(f.vaultTempFile.Close(), f.closeErr)
}

type faultVaultReadFile struct {
	vaultReadFile
	statErr    error
	readErr    error
	closeErr   error
	closes     int
	afterClose func() error
}

func (f *faultVaultReadFile) Stat() (os.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.vaultReadFile.Stat()
}

func (f *faultVaultReadFile) Read(data []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return f.vaultReadFile.Read(data)
}

func (f *faultVaultReadFile) Close() error {
	f.closes++
	err := errors.Join(f.vaultReadFile.Close(), f.closeErr)
	if f.afterClose != nil {
		err = errors.Join(err, f.afterClose())
	}
	return err
}

func replaceTestFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	replacement := path + ".external"
	require.NoError(t, os.WriteFile(replacement, data, mode))
	committed, err := replaceVaultFile(replacement, path)
	require.NoError(t, err)
	require.True(t, committed)
}

func requireSaveErrorKind(t *testing.T, err error, kind SaveErrorKind) {
	t.Helper()
	var outcome *SaveError
	require.ErrorAs(t, err, &outcome)
	require.Equal(t, kind, outcome.Kind)
	require.Equal(t, kind == SaveDurabilityUncertain, outcome.Committed())
}

func snapshotTestDatabase(t *testing.T, db *gokeepasslib.Database) []byte {
	t.Helper()
	data, err := json.Marshal(db)
	require.NoError(t, err)
	return data
}

// richTestDatabase includes preservation-sensitive data beyond Keepy's editable
// fields. Both formats have referenced and unused attachments so encoder cleanup
// can be checked without hiding the scenario-specific assertions in a helper.
func richTestDatabase(t *testing.T, version int) *gokeepasslib.Database {
	t.Helper()
	option := gokeepasslib.WithDatabaseKDBXVersion3()
	if version == 4 {
		option = gokeepasslib.WithDatabaseKDBXVersion4()
	}
	db := gokeepasslib.NewDatabase(option)
	db.Credentials = gokeepasslib.NewPasswordCredentials(testVaultPassword)
	db.Credentials.Key = bytes.Repeat([]byte{0x2a}, 32)
	db.Credentials.Windows = bytes.Repeat([]byte{0x3b}, 32)
	db.Header.RawData = []byte("original header bytes")
	db.Header.FileHeaders.Comment = []byte("database comment")
	db.Content.RawData = []byte("original content bytes")
	db.Hashes.Sha256[0] = 42
	db.Hashes.Hmac[0] = 43
	when := &wrappers.TimeWrapper{Formatted: true, Time: time.Date(2024, 3, 2, 1, 0, 0, 0, time.UTC)}
	meta := db.Content.Meta
	meta.DatabaseName = "preserve database"
	meta.DatabaseDescription = "preserve description"
	meta.DefaultUserName = "default user"
	meta.SettingsChanged = when
	meta.DatabaseNameChanged = when
	meta.DatabaseDescriptionChanged = when
	meta.DefaultUserNameChanged = when
	meta.MasterKeyChanged = when
	meta.RecycleBinChanged = when
	meta.EntryTemplatesGroupChanged = when
	meta.RecycleBinEnabled = wrappers.NewBoolWrapper(true)
	meta.RecycleBinUUID = gokeepasslib.NewUUID()
	meta.CustomIcons = []gokeepasslib.CustomIcon{{UUID: gokeepasslib.NewUUID(), Data: "aWNvbg=="}}
	meta.CustomData = []gokeepasslib.CustomData{{Key: "plugin", Value: "database data"}}
	entry := newTestEntry("Original", "user", "secret", "https://example.com")
	entry.Get(PasswordKey).Value.Protected = wrappers.NewBoolWrapper(true)
	entry.Values = append(entry.Values, gokeepasslib.ValueData{
		Key: "Custom secret", Value: gokeepasslib.V{Content: "custom secret", Protected: wrappers.NewBoolWrapper(true)},
	})
	entry.Tags = "preserve tags"
	entry.CustomIconUUID = meta.CustomIcons[0].UUID
	entry.CustomData = []gokeepasslib.CustomData{{Key: "plugin", Value: "entry data"}}
	entry.AutoType.Associations = []gokeepasslib.AutoTypeAssociation{{Window: "window", KeystrokeSequence: "{PASSWORD}"}}
	entry.Times.CreationTime = when
	entry.Times.LastModificationTime = when
	entry.Times.LastAccessTime = when
	entry.Times.ExpiryTime = when
	entry.Times.LocationChanged = when
	history := newTestEntry("History", "old user", "old secret", "https://old.example.com")
	history.Get(PasswordKey).Value.Protected = wrappers.NewBoolWrapper(true)
	entry.Histories = []gokeepasslib.History{{Entries: []gokeepasslib.Entry{history}}}
	binary := db.AddBinary([]byte{0xff, 0x00, 0x01, 0x02})
	entry.Binaries = []gokeepasslib.BinaryReference{binary.CreateReference("attachment.bin")}
	db.AddBinary([]byte("unused attachment"))
	group := gokeepasslib.NewGroup()
	group.Name = "Nested"
	group.Notes = "preserve group notes"
	group.Entries = []gokeepasslib.Entry{entry}
	root := gokeepasslib.NewGroup()
	root.Name = "Root"
	root.Groups = []gokeepasslib.Group{group}
	db.Content.Root.Groups = []gokeepasslib.Group{root}
	db.Content.Root.DeletedObjects = []gokeepasslib.DeletedObjectData{{UUID: gokeepasslib.NewUUID(), DeletionTime: when}}
	if version == 4 {
		db.Header.FileHeaders.PublicCustomData = testVariantDictionary("plugin", "public data")
	}
	return db
}

func testVariantDictionary(name, value string) *gokeepasslib.VariantDictionary {
	return &gokeepasslib.VariantDictionary{
		Version: 256,
		Items: []*gokeepasslib.VariantDictionaryItem{{
			Type: 0x18, Name: []byte(name), NameLength: int32(len(name)),
			Value: []byte(value), ValueLength: int32(len(value)),
		}},
	}
}

func decodeTestDatabase(t *testing.T, data []byte, credentials *gokeepasslib.DBCredentials) *gokeepasslib.Database {
	t.Helper()
	db := gokeepasslib.NewDatabase()
	db.Credentials = credentials
	require.NoError(t, gokeepasslib.NewDecoder(bytes.NewReader(data)).Decode(db))
	require.NoError(t, db.UnlockProtectedEntries())
	return db
}
