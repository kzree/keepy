package list

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list/entryform"
)

func TestConflictDraftSurvivesFailedReloadAndRecovery(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m.SetListTableSize(120, 30)
	id := gokeepasslib.NewUUID()
	entry := service.VaultEntry{ID: id, Title: "My draft"}
	m.SetEntries([]service.VaultEntry{entry})
	m, request := m.Update(tea.KeyPressMsg(tea.Key{Code: 'e'}))
	requestID := request().(EditEntryRequestMsg).RequestID
	m, _ = m.Update(OpenEditEntryMsg{RequestID: requestID, Entry: entry, Password: "draft secret"})
	m, _ = m.Update(entryform.SubmitFailedMsg{Error: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}})
	for _, reloadErr := range []error{errors.New("reloadfailure"), nil} {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
		require.NotNil(t, cmd)
		assert.IsType(t, ReloadVaultRequestMsg{}, cmd())
		m, _ = m.Update(ReloadVaultResultMsg{Error: reloadErr})
		assert.Equal(t, reloadErr != nil, m.saveConflict)
		assert.False(t, m.reloading)
		assert.True(t, m.IsTextInputActive())
		assert.Contains(t, m.entryForm.View(), "My draft")
		assert.Contains(t, m.entryForm.View(), "draft secret")
	}
	assert.Contains(t, m.entryForm.View(), "Review your draft")
}

func TestDelayedEditCannotReplaceNewDraftOrConflictRecovery(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m.SetListTableSize(120, 30)
	entry := service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Old edit"}
	m.SetEntries([]service.VaultEntry{entry})
	m, request := m.Update(tea.KeyPressMsg(tea.Key{Code: 'e'}))
	requestID := request().(EditEntryRequestMsg).RequestID
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n'}))
	m, cmd := m.Update(OpenEditEntryMsg{RequestID: requestID, Entry: entry, Password: "old secret"})
	assert.Nil(t, cmd)
	assert.NotContains(t, m.entryForm.View(), "Old edit")
	assert.NotContains(t, m.entryForm.View(), "old secret")
	m, _ = m.Update(entryform.SubmitFailedMsg{Error: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}})
	m, cmd = m.Update(OpenEditEntryMsg{RequestID: requestID, Entry: entry})
	assert.Nil(t, cmd)
	assert.True(t, m.saveConflict)
	assert.Contains(t, m.GlobalNotice(), "Ctrl+R")
}

func TestDeleteConflictRequiresReloadBeforeAnotherMutation(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m, _ = m.Update(SaveResultMsg{Error: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}})
	for _, key := range []rune{'n', 'e', 'd'} {
		var cmd tea.Cmd
		m, cmd = m.Update(tea.KeyPressMsg(tea.Key{Code: key}))
		assert.Nil(t, cmd)
		assert.False(t, m.showSidePane)
		assert.False(t, m.showDeletePrompt)
	}
	m, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	require.NotNil(t, cmd)
	assert.IsType(t, ReloadVaultRequestMsg{}, cmd())
	assert.True(t, m.reloading)
}

func TestDeleteOnEmptyListDoesNotOpenPrompt(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	assert.Nil(t, cmd)
	assert.False(t, m.showDeletePrompt)
}

func TestStaleConfirmationCannotConsumeNewPrompt(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m.SetEntries([]service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Entry"}})
	m, oldOpen := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	old := oldOpen().(common.OpenConfirmMsg)
	m, _ = m.Update(common.CloseConfirmMsg{RequestID: old.RequestID, Success: false})
	m, newOpen := m.Update(tea.KeyPressMsg(tea.Key{Code: 'd'}))
	current := newOpen().(common.OpenConfirmMsg)
	require.NotEqual(t, old.RequestID, current.RequestID)
	m, cmd := m.Update(old)
	assert.Nil(t, cmd)
	m, cmd = m.Update(common.CloseConfirmMsg{RequestID: old.RequestID, Success: true, Cmd: old.Cmd})
	assert.Nil(t, cmd)
	assert.True(t, m.showDeletePrompt)
	assert.False(t, m.deleting)
	m, _ = m.Update(current)
	m, confirm := m.Update(tea.KeyPressMsg(tea.Key{Code: 'y'}))
	require.NotNil(t, confirm)
	m, deletion := m.Update(confirm())
	require.NotNil(t, deletion)
	assert.Equal(t, current.RequestID, deletion().(DeleteEntryRequestMsg).RequestID)
}

func TestConflictBlocksMutationsButNotSearchInput(t *testing.T) {
	m := NewListModel(locale.En, control.DefaultKeymap())
	m.SetListTableSize(120, 30)
	m.SetEntries([]service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Needed"}})
	m, _ = m.Update(SaveResultMsg{Error: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}})
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Text: "n"}))
	assert.False(t, m.IsTextInputActive(), "conflict recovery blocks creating entries")
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Text: "f"}))
	for _, char := range "ned" {
		m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	assert.Equal(t, "ned", m.search.Value())
}
