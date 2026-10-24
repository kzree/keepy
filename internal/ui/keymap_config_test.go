package ui

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/util/testutil"
)

// startWithKeybindings saves overrides, starts the root, and logs in through the UI.
func startWithKeybindings(t *testing.T, overrides map[string][]string) (RootModel, string) {
	t.Helper()
	home := testutil.TempHome(t)
	require.NoError(t, service.SaveConfig(&service.Config{Credentials: service.Credentials{DBPath: "vault.kdbx"}, Keybindings: overrides}))
	r := NewRootModel()
	r.db = &outcomeVault{}
	r.initClipboard = func() error { return nil }
	drainRoot(t, &r, r.Init())
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 40})
	return r, filepath.Join(home, ".config", "keepy", "config.toml")
}

func loginThroughUI(t *testing.T, r *RootModel) {
	t.Helper()
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	require.Equal(t, screenList, r.activeView)
}

func TestConfiguredKeybindingsDriveDispatchAndHelp(t *testing.T) {
	r, _ := startWithKeybindings(t, map[string][]string{"new_entry": {"a"}, "show_help_anywhere": {"f12"}})
	assert.Contains(t, ansi.Strip(r.View().Content), "F12: help", "login hints show the effective key")
	loginThroughUI(t, &r)

	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	assert.False(t, r.list.IsTextInputActive(), "the replaced default no longer creates an entry")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	assert.False(t, r.helpVisible)

	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF12}))
	require.True(t, r.helpVisible)
	help := ansi.Strip(r.View().Content)
	assert.Contains(t, help, "a — create entry")
	assert.Contains(t, help, "f12 — open help everywhere")
	assert.NotContains(t, help, "f1 —")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF12}))

	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'a', Text: "a"}))
	assert.True(t, r.list.IsTextInputActive(), "the override opens the entry form")
}

func TestInvalidKeybindingsUseDefaultsWithoutBlockingOrRewritingConfig(t *testing.T) {
	r, path := startWithKeybindings(t, map[string][]string{"new_entry": {"e"}})
	require.Error(t, r.keymapError)
	assert.Contains(t, ansi.Strip(r.View().Content), "Key binding overrides ignored")
	loginThroughUI(t, &r)
	assert.Contains(t, ansi.Strip(r.View().Content), "new_entry", "the notice survives login")

	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	assert.True(t, r.list.IsTextInputActive(), "defaults remain in effect")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "new_entry", "saving credentials keeps the user's overrides")
}
