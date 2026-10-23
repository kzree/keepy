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

func TestLoginPreservesPreferencesAndUpdatesCredentials(t *testing.T) {
	testutil.TempHome(t)
	before := &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}, Credentials: service.Credentials{DBPath: "old.kdbx", KeyFilePath: "old.key"}}
	r := rootWithVault(t, &outcomeVault{})
	r.cfg = before
	r.activeView = screenLogin
	r, _ = r.handleLoginSubmitMsg(login.LoginSubmitMsg{DBPath: "new.kdbx", KeyFilePath: "new.key", Password: "secret-not-in-config"})
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
	r, _ = r.handleLoginSubmitMsg(login.LoginSubmitMsg{DBPath: "new.kdbx"})
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
	r, _ = r.handleLoginSubmitMsg(login.LoginSubmitMsg{DBPath: "first.kdbx"})
	assert.Contains(t, r.View().Content, "settings could not be saved")
	require.NoError(t, os.Remove(blocked))
	r, _ = r.handleLoginSubmitMsg(login.LoginSubmitMsg{DBPath: "second.kdbx"})
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
	r, cmd := r.handleLoginSubmitMsg(login.LoginSubmitMsg{DBPath: "new.kdbx"})
	assert.Equal(t, screenLogin, r.activeView)
	assert.Same(t, before, r.cfg)
	require.NotNil(t, cmd)
	assert.IsType(t, login.AuthenticationFailedMsg{}, cmd())
	loaded, err := service.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, before, loaded)
}
