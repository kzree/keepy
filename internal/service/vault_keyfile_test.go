package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
)

func TestKeyFileHandlesAreReleasedAfterAuthentication(t *testing.T) {
	for _, outcome := range []string{"success and reauthentication", "wrong password", "invalid XML key"} {
		t.Run(outcome, func(t *testing.T) {
			dir := t.TempDir()
			keyPath := filepath.Join(dir, "vault.key")
			dbPath := filepath.Join(dir, "vault.kdbx")
			writeTestKeyFile(t, keyPath, 42)
			writeTestDatabase(t, dbPath, testVaultPassword, keyPath, gokeepasslib.Group{Name: "Root"})
			// Fixture creation must release the key too. Windows rejects this rename
			// if a reader without delete sharing still owns an open handle.
			moved := keyPath + ".moved"
			require.NoError(t, os.Rename(keyPath, moved))
			require.NoError(t, os.Rename(moved, keyPath))
			password := testVaultPassword
			if outcome == "wrong password" {
				password = "incorrect password"
			}
			if outcome == "invalid XML key" {
				require.NoError(t, os.WriteFile(keyPath, []byte(`<KeyFile><Meta><Version>unsupported</Version></Meta><Key><Data>00</Data></Key></KeyFile>`), 0o600))
			}
			vault := NewVault()
			err := vault.Authenticate(dbPath, keyPath, password, true)
			if outcome == "success and reauthentication" {
				require.NoError(t, err)
				require.NoError(t, vault.ReAuthenticate())
			} else {
				require.Error(t, err)
				assert.False(t, vault.authenticated)
			}
			require.NoError(t, os.Rename(keyPath, moved), "authentication must not retain a key-file handle")
			require.NoError(t, os.Remove(moved))
		})
	}
}
