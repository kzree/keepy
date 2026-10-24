package login

import (
	"charm.land/lipgloss/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/util/testutil"
)

func driveLogin(t *testing.T, m *LoginModel, msg tea.Msg, requests *[]LoginSubmitMsg) {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	drainLogin(t, m, cmd, requests)
}

func drainLogin(t *testing.T, m *LoginModel, cmd tea.Cmd, requests *[]LoginSubmitMsg) {
	t.Helper()
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
		if request, ok := msg.(LoginSubmitMsg); ok {
			*requests = append(*requests, request)
			return nil
		}
		var next tea.Cmd
		*m, next = m.Update(msg)
		return next
	})
}

func TestBrowsingSelectsDatabaseAndKeyWithoutLosingPassword(t *testing.T) {
	dir := testutil.TempHome(t)
	database, key := filepath.Join(dir, "a.kdbx"), filepath.Join(dir, "b.key")
	require.NoError(t, os.WriteFile(database, []byte("database"), 0o600))
	require.NoError(t, os.WriteFile(key, []byte("key"), 0o600))
	elsewhere := t.TempDir()
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: filepath.Join(elsewhere, "missing.kdbx"), KeyFilePath: filepath.Join(elsewhere, "saved.key")}})
	m.SetSize(160, 24)
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	for _, char := range "typed-secret" {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}), &requests)
	}
	original := m.form
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}), &requests)
	assert.Equal(t, pickerDatabase, m.picking)
	assert.Equal(t, dir, m.picker.CurrentDirectory)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), &requests)
	assert.Same(t, original, m.form, "cancel must retain the original form and focus")
	assert.Equal(t, "typed-secret", m.formValues.password)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	assert.Equal(t, pickerNone, m.picking)
	assert.Equal(t, database, m.formValues.dbPath)
	assert.Contains(t, m.View(), "a.kdbx")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF3}), &requests)
	assert.Equal(t, dir, m.picker.CurrentDirectory, "key-file browsing also starts at home")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	assert.Equal(t, key, m.formValues.keyFilePath)
	assert.True(t, m.formValues.useKeyFile)
	assert.Equal(t, "typed-secret", m.formValues.password)
	assert.Empty(t, requests, "selection must not authenticate automatically")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	require.Len(t, requests, 1)
	assert.Equal(t, database, requests[0].DBPath)
	assert.Equal(t, key, requests[0].KeyFilePath)
	assert.Equal(t, "typed-secret", requests[0].Password)
}

func TestOptionalKeyFileToggleRetainsPathAndOmitsDisabledKey(t *testing.T) {
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: "vault.kdbx", KeyFilePath: "saved.key"}})
	m.SetSize(100, 24)
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	for range 3 {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF4}), &requests)
	}
	assert.False(t, m.formValues.useKeyFile)
	assert.Equal(t, "saved.key", m.formValues.keyFilePath)
	assert.Contains(t, m.View(), "Disabled")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	require.Len(t, requests, 1)
	assert.Empty(t, requests[0].KeyFilePath)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}), &requests)
	assert.Equal(t, pickerNone, m.picking, "browsing must be disabled during authentication")
}

func TestPickerRejectsStaleResultsAndRecoversDirectoryErrors(t *testing.T) {
	dir := testutil.TempHome(t)
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: filepath.Join(dir, "vault.kdbx")}})
	m.SetSize(80, 24)
	m, cmd := m.openPicker(pickerDatabase)
	old := cmd()
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	m, newCmd := m.openPicker(pickerKeyFile)
	m, staleCmd := m.Update(old)
	assert.Nil(t, staleCmd)
	assert.True(t, m.pickerLoading)
	m, read := m.Update(newCmd())
	require.NotNil(t, read)
	require.NoError(t, os.Remove(dir))
	m, _ = m.Update(read())
	require.ErrorIs(t, m.pickerError, os.ErrNotExist)
	var requests []LoginSubmitMsg
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	assert.Equal(t, pickerKeyFile, m.picking)
	assert.Empty(t, requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyLeft}), &requests)
	require.NoError(t, m.pickerError)
	assert.Equal(t, filepath.Dir(dir), m.picker.CurrentDirectory)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), &requests)
	assert.False(t, m.formValues.useKeyFile, "cancelled key-file browsing must not enable it")
}

func TestPickerNavigatesDirectoriesAndManualPathsStayEditable(t *testing.T) {
	dir := testutil.TempHome(t)
	nested := filepath.Join(dir, "nested")
	require.NoError(t, os.Mkdir(nested, 0o755))
	path := filepath.Join(nested, "vault.kdbx")
	require.NoError(t, os.WriteFile(path, []byte("database"), 0o600))
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: dir}})
	m.SetSize(120, 24)
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	assert.Equal(t, nested, m.picker.CurrentDirectory)
	assert.Equal(t, pickerDatabase, m.picking)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	assert.Equal(t, path, m.formValues.dbPath)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}), &requests)
	assert.Equal(t, dir, m.picker.CurrentDirectory, "reopening starts at home rather than the selected file's folder")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: 't', Mod: tea.ModCtrl}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests) // database field
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}), &requests)
	for _, char := range "manual.kdbx" {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}), &requests)
	}
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	require.Len(t, requests, 1)
	assert.Equal(t, "manual.kdbx", requests[0].DBPath)
}

func TestLateFormCompletionAfterPickerCancelDoesNotAuthenticate(t *testing.T) {
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: "vault.kdbx"}})
	m.SetSize(100, 24)
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	m, pendingForm := m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	require.False(t, m.loading, "Huh completion is delivered through commands")
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyF2}))
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	drainLogin(t, &m, pendingForm, &requests)
	assert.Empty(t, requests)
	assert.False(t, m.loading)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	require.Len(t, requests, 1, "a fresh explicit submit should still work")
}

func TestLoginLayoutKeepsInputUsableWithLongPaths(t *testing.T) {
	m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: strings.Repeat("long-path/", 30) + "vault.kdbx", KeyFilePath: strings.Repeat("key-path/", 30) + "vault.key"}})
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	for _, size := range [][2]int{{100, 24}, {40, 12}, {18, 3}, {8, 1}, {100, 24}} {
		m.SetSize(size[0], size[1])
		assert.LessOrEqual(t, lipgloss.Height(m.View()), size[1])
		assert.LessOrEqual(t, lipgloss.Width(m.View()), size[0])
		if size[0] >= 18 {
			assert.Contains(t, m.View(), "Password")
		}
	}
}

func TestMissingRequiredPathsDoNotSubmit(t *testing.T) {
	m := NewLoginModel(nil)
	m.SetSize(100, 24)
	var requests []LoginSubmitMsg
	drainLogin(t, &m, m.Init(), &requests)
	for range 2 {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	}
	assert.Empty(t, requests, "database path is required")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF4}), &requests)
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	for _, char := range "vault.kdbx" {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}), &requests)
	}
	for range 2 {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	}
	assert.Empty(t, requests, "enabled key file needs a path")
	driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyF4}), &requests)
	for range 2 {
		driveLogin(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	}
	require.Len(t, requests, 1)
	assert.Equal(t, "vault.kdbx", requests[0].DBPath)
	assert.Empty(t, requests[0].KeyFilePath)
	assert.Contains(t, m.View(), "vault.kdbx", "selected database remains visible while authenticating")
}
