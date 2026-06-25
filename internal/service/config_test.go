package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfigReadsTomlConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte(`[credentials]
db_path = "~/vault.kdbx"
key_file_path = "~/vault.key"
`), 0o644))

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "~/vault.kdbx", cfg.Credentials.DBPath)
	assert.Equal(t, "~/vault.key", cfg.Credentials.KeyFilePath)
}

func TestLoadConfigReturnsErrorWhenConfigIsMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := LoadConfig()

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigReturnsErrorForInvalidToml(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("[credentials\n"), 0o644))

	cfg, err := LoadConfig()

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestSaveConfigRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	want := &Config{
		Credentials: Credentials{
			DBPath:      "~/db.kdbx",
			KeyFilePath: "~/db.key",
		},
	}

	require.NoError(t, SaveConfig(want))
	got, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, got)
}

func TestSaveConfigTruncatesExistingConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)

	require.NoError(t, SaveConfig(&Config{
		Credentials: Credentials{
			DBPath:      "~/a/very/long/database/path.kdbx",
			KeyFilePath: "~/a/very/long/key/file/path.key",
		},
	}))
	require.NoError(t, SaveConfig(&Config{
		Credentials: Credentials{
			DBPath:      "~/db.kdbx",
			KeyFilePath: "",
		},
	}))

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "very/long")

	cfg, err := LoadConfig()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "~/db.kdbx", cfg.Credentials.DBPath)
	assert.Empty(t, cfg.Credentials.KeyFilePath)
}

func TestSaveConfigCreatesParentDirectories(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := filepath.Join(home, configPathFromHome, configFileName)

	require.NoError(t, SaveConfig(&Config{
		Credentials: Credentials{
			DBPath:      "~/db.kdbx",
			KeyFilePath: "~/db.key",
		},
	}))

	info, err := os.Stat(configPath)
	require.NoError(t, err)
	assert.False(t, info.IsDir())
}
