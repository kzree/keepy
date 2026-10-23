package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

func submitLogin(t *testing.T, r *RootModel, msg login.LoginSubmitMsg) {
	t.Helper()
	r.activeView = screenLogin
	r.login = login.NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: msg.DBPath, KeyFilePath: msg.KeyFilePath}})
	drainRoot(t, r, r.login.Init())
	for _, char := range msg.Password {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
}

func TestLoginPreservesPreferencesAndUpdatesCredentials(t *testing.T) {
	testutil.TempHome(t)
	before := &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}, Credentials: service.Credentials{DBPath: "old.kdbx", KeyFilePath: "old.key"}}
	r := rootWithVault(t, &outcomeVault{})
	r.cfg = before
	r.activeView = screenLogin
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "new.kdbx", KeyFilePath: "new.key", Password: "secret-not-in-config"})
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, before.Preferences, r.cfg.Preferences)
	assert.Equal(t, service.Credentials{DBPath: "new.kdbx", KeyFilePath: "new.key"}, r.cfg.Credentials)
	assert.Equal(t, "old.kdbx", before.Credentials.DBPath, "login must not mutate another model's config snapshot")
	loaded, err := service.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, r.cfg, loaded)
	assert.NotContains(t, r.View().Content, "settings could not be saved")
}

func TestConfigSaveFailureDoesNotUndoLogin(t *testing.T) {
	home := testutil.TempHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".config"), []byte("not a directory"), 0o600))
	vault := &outcomeVault{}
	r := rootWithVault(t, vault)
	r.cfg = &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}}
	r.activeView = screenLogin
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "new.kdbx"})
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, "saved-locale", r.cfg.Preferences.Locale)
	assert.Equal(t, "new.kdbx", r.cfg.Credentials.DBPath)
	assert.Contains(t, r.View().Content, "settings could not be saved")
	assert.LessOrEqual(t, lipgloss.Height(r.View().Content), r.height)
	// The warning remains visible while ordinary vault operations stay usable.
	submitThroughUI(t, &r, "add")
	assert.Equal(t, 1, vault.mutations)
	assert.Contains(t, r.View().Content, "Committed")
	assert.Contains(t, r.View().Content, "settings could not be saved")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	assert.True(t, r.list.IsTextInputActive())
	assert.Contains(t, r.View().Content, "settings could not be saved")
	assert.LessOrEqual(t, lipgloss.Height(r.View().Content), r.height)
}

func TestSuccessfulLoginClearsPreviousConfigWarning(t *testing.T) {
	home := testutil.TempHome(t)
	blocked := filepath.Join(home, ".config")
	require.NoError(t, os.WriteFile(blocked, []byte("blocked"), 0o600))
	r := rootWithVault(t, &outcomeVault{})
	r.cfg = &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}}
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "first.kdbx"})
	assert.Contains(t, r.View().Content, "settings could not be saved")
	require.NoError(t, os.Remove(blocked))
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "second.kdbx"})
	assert.NotContains(t, r.View().Content, "settings could not be saved")
	loaded, err := service.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "saved-locale", loaded.Preferences.Locale)
	assert.Equal(t, "second.kdbx", loaded.Credentials.DBPath)
}

func TestAuthenticationFailureDoesNotChangeOrSaveConfig(t *testing.T) {
	testutil.TempHome(t)
	before := &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}, Credentials: service.Credentials{DBPath: "old.kdbx"}}
	require.NoError(t, service.SaveConfig(before))
	r := rootWithVault(t, &outcomeVault{authErr: errors.New("wrong password")})
	r.cfg = before
	r.activeView = screenLogin
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "new.kdbx"})
	assert.Equal(t, screenLogin, r.activeView)
	assert.Same(t, before, r.cfg)
	assert.Contains(t, r.View().Content, "wrong password")
	loaded, err := service.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, before, loaded)
}

func TestAuthenticationFailureAllowsKeyboardCorrectionAndExplicitRetry(t *testing.T) {
	testutil.TempHome(t)
	fake := &outcomeVault{authErr: errors.New("wrong password")}
	r := rootWithVault(t, fake)
	r.cfg = &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}}
	submitLogin(t, &r, login.LoginSubmitMsg{DBPath: "wrong.kdbx", Password: "wrong"})
	assert.Equal(t, 1, fake.authCalls)
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 30})
	assert.Equal(t, 1, fake.authCalls, "failure and resizing must not automatically retry")
	fake.authErr = nil
	// Failure opens the full editable form: password, database, then key path.
	for _, value := range []string{"correct-password", "corrected.kdbx", "corrected.key"} {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}))
		for _, char := range value {
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
		}
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
	assert.Equal(t, 2, fake.authCalls)
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, service.Credentials{DBPath: "corrected.kdbx", KeyFilePath: "corrected.key"}, fake.authCredentials)
	assert.Equal(t, "correct-password", fake.authPassword)
	assert.True(t, fake.authUsesKey)
	assert.Equal(t, "saved-locale", r.cfg.Preferences.Locale)
}
