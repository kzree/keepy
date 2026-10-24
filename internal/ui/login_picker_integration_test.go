package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

func TestLoginPickerHelpCancelAndAuthenticationPreserveState(t *testing.T) {
	home := testutil.TempHome(t)
	database := filepath.Join(home, "a.kdbx")
	require.NoError(t, os.WriteFile(database, []byte("database fixture"), 0o600))
	fake := &outcomeVault{}
	r := rootWithVault(t, fake)
	r.cfg = &service.Config{Preferences: service.Preferences{Locale: "saved-locale"}, Credentials: service.Credentials{DBPath: database}}
	r.activeView = screenLogin
	r.login = login.NewLoginModel(r.cfg, locale.En, control.DefaultKeymap())
	drainRoot(t, &r, r.login.Init())
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 100, Height: 28})
	for _, char := range "secret?" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	assert.Contains(t, r.View().Content, "a.kdbx")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	assert.Equal(t, control.FilePickerKeys, r.keymapContext())
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?'}))
	assert.True(t, r.helpVisible, "picker navigation is not text input")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	assert.Equal(t, control.FilePickerKeys, r.keymapContext())
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 40, Height: 12})
	assert.LessOrEqual(t, lipgloss.Height(r.View().Content), 12)
	assert.Contains(t, r.View().Content, "Esc: cancel")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	assert.Equal(t, control.LoginKeys, r.keymapContext())
	assert.Zero(t, fake.authCalls)
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 100, Height: 28})
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Zero(t, fake.authCalls, "file selection must not submit login")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Equal(t, screenList, r.activeView)
	assert.Equal(t, "secret?", fake.authPassword)
	assert.Equal(t, database, fake.authCredentials.DBPath)
	assert.False(t, fake.authUsesKey)
	loaded, err := service.LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "saved-locale", loaded.Preferences.Locale)
}
