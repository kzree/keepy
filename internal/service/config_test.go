package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/util/testutil"
)

func TestLoadConfigReadsTomlConfig(t *testing.T) {
	home := testutil.TempHome(t)
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

func TestLoadConfigCreatesDefaultConfigWhenConfigIsMissing(t *testing.T) {
	home := testutil.TempHome(t)
	configPath := filepath.Join(home, configPathFromHome, configFileName)

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "En", cfg.Preferences.Locale)
	assert.Empty(t, cfg.Credentials.DBPath)
	assert.Empty(t, cfg.Credentials.KeyFilePath)

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `locale = 'En'`)

	loaded, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, cfg, loaded)
}

func TestLoadConfigReturnsErrorForInvalidToml(t *testing.T) {
	home := testutil.TempHome(t)
	configPath := filepath.Join(home, configPathFromHome, configFileName)
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0o755))
	require.NoError(t, os.WriteFile(configPath, []byte("[credentials\n"), 0o644))

	cfg, err := LoadConfig()

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestSaveConfigRoundTrip(t *testing.T) {
	testutil.TempHome(t)
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
	home := testutil.TempHome(t)
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
	home := testutil.TempHome(t)
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

func TestLoadConfigReturnsReadErrorWhenConfigPathIsDirectory(t *testing.T) {
	home := testutil.TempHome(t)
	configPath := filepath.Join(home, configPathFromHome, configFileName)
	require.NoError(t, os.MkdirAll(configPath, 0o755))

	cfg, err := LoadConfig()

	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestSaveConfigReturnsErrorWhenParentPathIsFile(t *testing.T) {
	home := testutil.TempHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config"), []byte("not a directory"), 0o644))

	err := SaveConfig(&Config{})

	require.Error(t, err)
}
