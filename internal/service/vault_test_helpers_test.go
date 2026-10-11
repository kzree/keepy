package service

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
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
		credentials, err := gokeepasslib.NewPasswordAndKeyCredentials(password, keyFilePath)
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

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
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
