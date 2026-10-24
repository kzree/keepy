package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	r.writeClipboard = func([]byte) <-chan struct{} { writes++; return make(chan struct{}) }
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
	r.writeClipboard = func([]byte) <-chan struct{} { writes++; return make(chan struct{}) }
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
	r.writeClipboard = func([]byte) <-chan struct{} { t.Fatal("clipboard write before readiness"); return nil }
	sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
	assert.Zero(t, vault.passwordReads)
	sendRoot(t, &r, clipboardInitializedMsg{})
	assert.True(t, r.clipboardReady)
	assert.Nil(t, r.copyError)
	r.configSaveError = errors.New("settingsfailure")
	r.writeClipboard = func(data []byte) <-chan struct{} { assert.Equal(t, "secret", string(data)); return nil }
	r, cmd := r.handleCopyPasswordRequestMsg(list.CopyPasswordRequestMsg{ID: id})
	assert.Nil(t, cmd)
	require.Error(t, r.copyError)
	assert.Contains(t, r.View().Content, "settingsfailure")
	assert.Contains(t, r.View().Content, "Copy failed")
	r.writeClipboard = func(data []byte) <-chan struct{} { assert.Equal(t, "secret", string(data)); return make(chan struct{}) }
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

// fakeClipboard records writes. Closing changed simulates another application
// overwriting the copied password; fail makes writes report failure.
type fakeClipboard struct {
	writes  []string
	changed chan struct{}
	fail    bool
}

func (c *fakeClipboard) write(data []byte) <-chan struct{} {
	c.writes = append(c.writes, string(data))
	if c.fail {
		return nil
	}
	c.changed = make(chan struct{})
	return c.changed
}

// rootWithCopiedPassword copies "secret" with the given clear delay and returns
// the copy request for repeating it.
func rootWithCopiedPassword(t *testing.T, delay time.Duration) (RootModel, *fakeClipboard, list.CopyPasswordRequestMsg) {
	t.Helper()
	copyRequest := list.CopyPasswordRequestMsg{ID: gokeepasslib.NewUUID()}
	r := rootWithVault(t, &outcomeVault{passwords: map[gokeepasslib.UUID]string{copyRequest.ID: "secret"}})
	clip := &fakeClipboard{}
	r.clipboardReady, r.writeClipboard, r.clipboardClearDelay = true, clip.write, delay
	r, cmd := r.handleCopyPasswordRequestMsg(copyRequest)
	require.NotNil(t, cmd)
	require.Equal(t, []string{"secret"}, clip.writes)
	return r, clip, copyRequest
}

func TestCopiedPasswordIsClearedOnceAfterTimeout(t *testing.T) {
	r, clip, _ := rootWithCopiedPassword(t, 2*time.Minute)
	expired := clipboardExpiredMsg{ID: r.clipboardClearID}
	sendRoot(t, &r, expired)
	assert.Equal(t, []string{"secret", ""}, clip.writes)
	sendRoot(t, &r, expired)
	r, _ = r.quit()
	assert.Len(t, clip.writes, 2, "an already cleared clipboard is not written again")
}

func TestNewerCopySupersedesEarlierClipboardTimeout(t *testing.T) {
	r, clip, copyRequest := rootWithCopiedPassword(t, 2*time.Minute)
	first := clipboardExpiredMsg{ID: r.clipboardClearID}
	r, _ = r.handleCopyPasswordRequestMsg(copyRequest)
	sendRoot(t, &r, first)
	assert.Equal(t, []string{"secret", "secret"}, clip.writes, "the earlier timer must not shorten the newer copy")
	sendRoot(t, &r, clipboardExpiredMsg{ID: r.clipboardClearID})
	assert.Equal(t, []string{"secret", "secret", ""}, clip.writes)
}

func TestClipboardTimeoutKeepsContentCopiedElsewhere(t *testing.T) {
	r, clip, _ := rootWithCopiedPassword(t, 2*time.Minute)
	close(clip.changed)
	sendRoot(t, &r, clipboardExpiredMsg{ID: r.clipboardClearID})
	r, _ = r.quit()
	assert.Equal(t, []string{"secret"}, clip.writes)
}

func TestQuitClearsPendingCopiedPassword(t *testing.T) {
	r, clip, _ := rootWithCopiedPassword(t, 2*time.Minute)
	cmd := updateRoot(&r, tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
	assert.Equal(t, []string{"secret", ""}, clip.writes)
}

func TestDisabledClipboardTimeoutKeepsCopiedPassword(t *testing.T) {
	r, clip, _ := rootWithCopiedPassword(t, 0)
	sendRoot(t, &r, clipboardExpiredMsg{ID: r.clipboardClearID})
	r, _ = r.quit()
	assert.Equal(t, []string{"secret"}, clip.writes)
}

func TestClipboardClearFailureIsReportedUntilNextCopy(t *testing.T) {
	r, clip, copyRequest := rootWithCopiedPassword(t, 2*time.Minute)
	clip.fail = true
	sendRoot(t, &r, clipboardExpiredMsg{ID: r.clipboardClearID})
	assert.Contains(t, r.renderBottomBar(118, 28), "Clipboard could not be cleared")
	clip.fail = false
	r, _ = r.handleCopyPasswordRequestMsg(copyRequest)
	assert.NotContains(t, r.renderBottomBar(118, 28), "Clipboard could not be cleared")
}

func TestClipboardTimeoutComesFromConfig(t *testing.T) {
	for value, want := range map[string]time.Duration{"": service.DefaultClipboardTimeout, "30s": 30 * time.Second, "off": 0, "never": service.DefaultClipboardTimeout} {
		home := testutil.TempHome(t)
		path := filepath.Join(home, ".config", "keepy", "config.toml")
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("[preferences]\nclipboard_timeout = '"+value+"'\n"), 0o600))
		r := NewRootModel()
		assert.Equal(t, want, r.clipboardClearDelay, value)
		if value == "never" {
			assert.Contains(t, r.renderBottomBar(118, 28), "Clipboard timeout setting ignored", "invalid values are reported and keep the default")
		} else {
			assert.NotContains(t, r.renderBottomBar(118, 28), "Clipboard timeout", value)
		}
	}
}

func TestCopyReportsEntriesWithoutPasswordOrMissingEntries(t *testing.T) {
	empty, gone := gokeepasslib.NewUUID(), gokeepasslib.NewUUID()
	r := rootWithVault(t, &outcomeVault{passwords: map[gokeepasslib.UUID]string{empty: ""}})
	clip := &fakeClipboard{}
	r.clipboardReady, r.writeClipboard = true, clip.write

	r, _ = r.handleCopyPasswordRequestMsg(list.CopyPasswordRequestMsg{ID: empty})
	assert.Contains(t, r.renderBottomBar(118, 28), "this entry has no password")
	r, _ = r.handleCopyPasswordRequestMsg(list.CopyPasswordRequestMsg{ID: gone})
	assert.Contains(t, r.renderBottomBar(118, 28), "entry not found")
	assert.Empty(t, clip.writes)
}
