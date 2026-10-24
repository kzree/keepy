package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

func TestMalformedConfigRequiresExplicitRecoveryWithoutOverwrite(t *testing.T) {
	home := testutil.TempHome(t)
	path := filepath.Join(home, ".config", "keepy", "config.toml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	malformed := []byte("[credentials\n")
	require.NoError(t, os.WriteFile(path, malformed, 0o600))
	r := NewRootModel()
	vault := &outcomeVault{}
	r.db = vault
	initCalls := 0
	r.initClipboard = func() error { initCalls++; return nil }
	cmd := r.Init()
	assert.Zero(t, initCalls, "clipboard init must run as a command")
	drainRoot(t, &r, cmd)
	assert.Equal(t, 1, initCalls)
	require.Error(t, r.configLoadError)
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
	assert.Contains(t, r.View().Content, "Configuration unavailable")
	assert.Contains(t, r.View().Content, "Ctrl+R")
	sendRoot(t, &r, login.LoginSubmitMsg{DBPath: "must-not-save.kdbx"})
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	assert.Zero(t, vault.authCalls)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, malformed, data)
	// Repairing the file alone does not retry until the user asks.
	fixed := []byte("[preferences]\nlocale = 'saved-locale'\n[credentials]\ndb_path = 'restored.kdbx'\nkey_file_path = 'restored.key'\n")
	require.NoError(t, os.WriteFile(path, fixed, 0o600))
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
	require.Error(t, r.configLoadError)
	sendRoot(t, &r, login.LoginSubmitMsg{DBPath: "must-still-not-save.kdbx"})
	assert.Zero(t, vault.authCalls)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, fixed, data)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	require.NoError(t, r.configLoadError)
	assert.Equal(t, "saved-locale", r.cfg.Preferences.Locale)
	assert.Equal(t, "restored.kdbx", r.cfg.Credentials.DBPath)
	assert.NotContains(t, r.View().Content, "Configuration unavailable")
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, fixed, data)
	// Submit the recovered short form through real input, proving it uses the
	// credentials loaded by explicit recovery rather than an empty fallback.
	for _, char := range "unlocksecret" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, 1, vault.authCalls)
	assert.Equal(t, service.Credentials{DBPath: "restored.kdbx", KeyFilePath: "restored.key"}, vault.authCredentials)
	assert.Equal(t, "unlocksecret", vault.authPassword)
	assert.True(t, vault.authUsesKey)
	assert.Equal(t, "saved-locale", r.cfg.Preferences.Locale)
}

func TestClipboardResultCanArriveAfterLoginWithoutLosingOtherNotices(t *testing.T) {
	home := testutil.TempHome(t)
	r := NewRootModel()
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Existing"}}, passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r.db = vault
	initializations, writes := 0, 0
	r.initClipboard = func() error { initializations++; return errors.New("lateclipboardfailure") }
	r.writeClipboard = func([]byte) bool { writes++; return true }
	commands, ok := r.Init()().(tea.BatchMsg)
	require.True(t, ok)
	require.Len(t, commands, 2)
	drainRoot(t, &r, commands[0]) // Login lifecycle runs before clipboard initialization.
	assert.Zero(t, initializations)
	configPath := filepath.Join(home, ".config", "keepy", "config.toml")
	require.NoError(t, os.Remove(configPath))
	require.NoError(t, os.Mkdir(configPath, 0o755)) // Make settings save fail after successful auth.
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "vault.kdbx"})
	assert.Equal(t, screenList, r.activeView)
	require.Error(t, r.configSaveError)
	sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
	assert.Zero(t, vault.passwordReads)
	assert.Zero(t, writes)
	submitThroughUI(t, &r, "add")
	sendRoot(t, &r, commands[1]()) // Initialization completes after login and mutation.
	assert.Equal(t, 1, initializations)
	assert.Equal(t, screenList, r.activeView)
	assert.Contains(t, r.View().Content, "Clipboard unavailable")
	assert.Contains(t, r.View().Content, "settings could not be saved")
	sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
	assert.Zero(t, vault.passwordReads)
	assert.Zero(t, writes)
	assert.Equal(t, 1, vault.mutations)
	sendRoot(t, &r, clipboardInitializedMsg{})
	require.Error(t, r.clipboardError)
}

func TestUnreadableConfigPathCanBeRepairedAndRetried(t *testing.T) {
	home := testutil.TempHome(t)
	path := filepath.Join(home, ".config", "keepy", "config.toml")
	require.NoError(t, os.MkdirAll(path, 0o755))
	r := NewRootModel()
	require.Error(t, r.configLoadError)
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.WriteFile(path, []byte("[preferences]\nlocale='En'\n"), 0o600))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	require.NoError(t, r.configLoadError)
}

func TestClipboardStartupFailureDoesNotPreventVaultUse(t *testing.T) {
	testutil.TempHome(t)
	r := NewRootModel()
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Existing"}}, passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r.db = vault
	r.initClipboard = func() error { return errors.New("clipboardbackendfailure") }
	writes := 0
	r.writeClipboard = func([]byte) bool { writes++; return true }
	drainRoot(t, &r, r.Init())
	require.Error(t, r.clipboardError)
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "vault.kdbx"})
	assert.Equal(t, screenList, r.activeView)
	assert.Contains(t, r.View().Content, "Clipboard unavailable")
	sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
	assert.Zero(t, writes)
	assert.Zero(t, vault.passwordReads, "unavailable clipboard must not retrieve a secret")
	submitThroughUI(t, &r, "add")
	assert.Equal(t, 1, vault.mutations)
	assert.Contains(t, r.View().Content, "Clipboard unavailable")
	// A stale startup message must not clear the failure.
	sendRoot(t, &r, clipboardInitializedMsg{})
	require.Error(t, r.clipboardError)
}

func TestClipboardWriteFailureAndRecoveryKeepOtherNotices(t *testing.T) {
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r := rootWithVault(t, vault)
	r.clipboardPending = true
	r.initClipboard = func() error { return nil }
	r.writeClipboard = func([]byte) bool { t.Fatal("clipboard write before readiness"); return false }
	sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
	assert.Zero(t, vault.passwordReads)
	sendRoot(t, &r, clipboardInitializedMsg{})
	assert.True(t, r.clipboardReady)
	assert.Nil(t, r.copyError)
	r.configSaveError = errors.New("settingsfailure")
	r.writeClipboard = func(data []byte) bool { assert.Equal(t, "secret", string(data)); return false }
	r, cmd := r.handleCopyPasswordRequestMsg(list.CopyPasswordRequestMsg{ID: id})
	assert.Nil(t, cmd)
	require.Error(t, r.copyError)
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.Contains(t, r.View().Content, "Copy failed")
	r.writeClipboard = func(data []byte) bool { assert.Equal(t, "secret", string(data)); return true }
	r, cmd = r.handleCopyPasswordRequestMsg(list.CopyPasswordRequestMsg{ID: id})
	require.NotNil(t, cmd)
	assert.IsType(t, list.CopyPasswordSuccessMsg{}, cmd())
	assert.Nil(t, r.copyError)
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.NotContains(t, r.View().Content, "Copy failed")
}

func TestConfigurationRecoveryKeepsClipboardFailure(t *testing.T) {
	testutil.TempHome(t)
	r := NewRootModel()
	r.configLoadError = errors.New("configfailure")
	r.clipboardError = errors.New("clipboardfailure")
	r.updateApplicationNotice()
	r, _ = r.retryConfiguration()
	require.NoError(t, r.configLoadError)
	require.Error(t, r.clipboardError)
	assert.Contains(t, r.renderBottomBar(118, 28), "Clipboard unavailable")
	assert.NotContains(t, r.renderBottomBar(118, 28), "configfailure")
	assert.NotNil(t, r.cfg)
	assert.Equal(t, service.Preferences{Locale: "en"}, r.cfg.Preferences)
}
