package service

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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

// writeTestConfig creates a config in a temporary home and returns its path.
func writeTestConfig(t *testing.T, data string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(testutil.TempHome(t), ".config", "keepy", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(data), mode))
	return path
}

func TestConfigReplacementFailuresKeepPreviousConfig(t *testing.T) {
	injected := errors.New("injected failure")
	for name, inject := range map[string]func(*vaultPersistence){
		"create temp": func(p *vaultPersistence) {
			p.createTemp = func(string, string) (vaultTempFile, error) { return nil, injected }
		},
		"write":   func(p *vaultPersistence) { captureTestTempFile(p).writeErr = injected },
		"short":   func(p *vaultPersistence) { captureTestTempFile(p).shortWrite = true },
		"chmod":   func(p *vaultPersistence) { captureTestTempFile(p).chmodErr = injected },
		"sync":    func(p *vaultPersistence) { captureTestTempFile(p).syncErr = injected },
		"close":   func(p *vaultPersistence) { captureTestTempFile(p).closeErr = injected },
		"replace": func(p *vaultPersistence) { p.replace = func(string, string) (bool, error) { return false, injected } },
	} {
		t.Run(name, func(t *testing.T) {
			path := writeTestConfig(t, "[preferences]\nlocale = 'en'\n", 0o644)
			p := newConfigPersistence()
			inject(p)

			err := replaceConfig(p, path, []byte("[preferences]\nlocale = 'replacement'\n"))

			require.Error(t, err)
			if name == "short" {
				require.ErrorIs(t, err, io.ErrShortWrite)
			} else {
				require.ErrorIs(t, err, injected)
			}
			assert.Contains(t, err.Error(), "config")
			assert.NotContains(t, err.Error(), "database")
			assert.Equal(t, "[preferences]\nlocale = 'en'\n", string(readTestFile(t, path)))
			assertNoVaultTemps(t, path)
		})
	}
}

func TestConfigDirectorySyncFailureReportsCommittedSave(t *testing.T) {
	path := writeTestConfig(t, "old", 0o644)
	injected := errors.New("directory sync failed")
	p := newConfigPersistence()
	p.syncDirectory = func(string) error { return injected }

	err := replaceConfig(p, path, []byte("new"))

	require.ErrorIs(t, err, injected)
	var durability *ConfigDurabilityError
	require.ErrorAs(t, err, &durability)
	assert.Equal(t, "new", string(readTestFile(t, path)))
	assertNoVaultTemps(t, path)
}

func TestSaveConfigPreservesSymlinkAndPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	home := testutil.TempHome(t)
	target := filepath.Join(home, "dotfiles", "keepy.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o755))
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o640))
	link := filepath.Join(home, ".config", "keepy", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	symlinkTestFile(t, target, link)

	require.NoError(t, SaveConfig(&Config{Preferences: Preferences{Locale: "en"}}))

	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the link must not be replaced by a regular file")
	assert.Contains(t, string(readTestFile(t, target)), "locale = 'en'")
	targetInfo, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), targetInfo.Mode().Perm())
	assertNoVaultTemps(t, target)
	assertNoVaultTemps(t, link)
}

func TestConfigReturnsHomeResolutionErrors(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	require.Error(t, SaveConfig(&Config{}))
	cfg, err := LoadConfig()
	require.Error(t, err)
	assert.Nil(t, cfg)
}

func TestLoadConfigCreatesDefaultConfigWhenConfigIsMissing(t *testing.T) {
	home := testutil.TempHome(t)
	configPath := filepath.Join(home, configPathFromHome, configFileName)

	cfg, err := LoadConfig()

	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "en", cfg.Preferences.Locale)
	assert.Empty(t, cfg.Credentials.DBPath)
	assert.Empty(t, cfg.Credentials.KeyFilePath)

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), `locale = 'en'`)

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

func TestSaveConfigReplacesLongerExistingConfig(t *testing.T) {
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

func TestClipboardClearDelay(t *testing.T) {
	for value, want := range map[string]time.Duration{
		"":     DefaultClipboardTimeout,
		"30s":  30 * time.Second,
		" 5m ": 5 * time.Minute,
		"off":  0,
		"OFF":  0,
		"0":    0,
	} {
		delay, err := Preferences{ClipboardTimeout: value}.ClipboardClearDelay()
		require.NoError(t, err, value)
		assert.Equal(t, want, delay, value)
	}
	for _, value := range []string{"2 minutes", "-1m", "never"} {
		delay, err := Preferences{ClipboardTimeout: value}.ClipboardClearDelay()
		require.ErrorContains(t, err, "clipboard_timeout", value)
		assert.Equal(t, DefaultClipboardTimeout, delay, "invalid values must keep clearing enabled")
	}
}
