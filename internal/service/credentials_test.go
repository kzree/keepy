package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSavedCredentialsEmptyConfigReturnsNil(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	creds, err := LoadSavedCredentials()

	require.NoError(t, err)
	assert.Nil(t, creds)
}

func TestLoadSavedCredentialsReadsSavedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`{"dbPath":"~/vault.kdbx","keyFilePath":"~/vault.key"}`), 0o644))

	creds, err := LoadSavedCredentials()

	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "~/vault.kdbx", creds.DBPath)
	assert.Equal(t, "~/vault.key", creds.KeyFilePath)
}

func TestSaveCredentialsRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := &Credentials{
		DBPath:      "~/db.kdbx",
		KeyFilePath: "~/db.key",
	}

	require.NoError(t, SaveCredentials(want))
	got, err := LoadSavedCredentials()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, got)
}

func TestSaveCredentialsTruncatesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)

	require.NoError(t, SaveCredentials(&Credentials{
		DBPath:      "~/a/very/long/database/path.kdbx",
		KeyFilePath: "~/a/very/long/key/file/path.key",
	}))
	require.NoError(t, SaveCredentials(&Credentials{
		DBPath:      "~/db.kdbx",
		KeyFilePath: "",
	}))

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "very/long")

	creds, err := LoadSavedCredentials()
	require.NoError(t, err)
	require.NotNil(t, creds)
	assert.Equal(t, "~/db.kdbx", creds.DBPath)
	assert.Empty(t, creds.KeyFilePath)
}
