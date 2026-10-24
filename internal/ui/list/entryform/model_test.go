package entryform

import (
	"errors"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/util/testutil"
)

func TestFailedSubmissionPreservesDraftWithoutAutomaticRetry(t *testing.T) {
	for _, kind := range []service.SaveErrorKind{service.SaveFailed, service.SaveConflict} {
		t.Run((&service.SaveError{Kind: kind}).Error(), func(t *testing.T) {
			m := NewEntryFormModel(locale.En, control.DefaultKeymap())
			id := gokeepasslib.NewUUID()
			draft := service.NewVaultEntry{Title: "Draft", Username: "user", Password: "draft secret", URL: "https://example.com"}
			m.Edit(id, draft)
			m.SetSize(50, 20)
			m.form.State = huh.StateCompleted
			m.loading = true
			m, _ = m.Update(SubmitFailedMsg{Error: &service.SaveError{Kind: kind, Err: errors.New("save failed")}})
			assert.False(t, m.loading)
			assert.Equal(t, huh.StateNormal, m.form.State)
			assert.Equal(t, id, *m.editingID)
			assert.Equal(t, draft.Password, m.formValues.Password)
			assert.Equal(t, draft.Title, m.formValues.Title)
			for range 3 {
				m, _ = m.Update(spinner.TickMsg{})
				assert.False(t, m.loading, "unrelated messages must not resubmit")
			}
			if kind == service.SaveConflict {
				m.form.State = huh.StateCompleted
				m, _ = m.Update(spinner.TickMsg{})
				assert.False(t, m.loading, "a conflict must block even an explicit submission until reload")
				m, _ = m.Update(ReloadSuccessMsg{})
				assert.False(t, m.loading)
				assert.Equal(t, huh.StateNormal, m.form.State)
				assert.Equal(t, draft.Password, m.formValues.Password)
				assert.Contains(t, m.View(), "Review your draft")
			}
			// Only completing the new, editable form starts the explicit retry.
			m.form.State = huh.StateCompleted
			m, cmd := m.Update(spinner.TickMsg{})
			assert.True(t, m.loading)
			require.NotNil(t, cmd)
		})
	}
}

func sendForm(t *testing.T, m *EntryFormModel, msg tea.Msg, requests *[]SubmitEntryMsg) {
	t.Helper()
	var cmd tea.Cmd
	*m, cmd = m.Update(msg)
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
		if request, ok := msg.(SubmitEntryMsg); ok {
			*requests = append(*requests, request)
			return nil
		}
		var cmd tea.Cmd
		*m, cmd = m.Update(msg)
		return cmd
	})
}

func TestKeyboardFailureEditAndExplicitRetryPreservesGeneratedPassword(t *testing.T) {
	m := NewEntryFormModel(locale.En, control.DefaultKeymap())
	m.SetSize(55, 20)
	var requests []SubmitEntryMsg
	testutil.DrainTeaCommands(t, m.Init(), func(msg tea.Msg) tea.Cmd { var cmd tea.Cmd; m, cmd = m.Update(msg); return cmd })
	for _, char := range "Draft" {
		sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}), &requests)
	}
	sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: 'g', Mod: tea.ModCtrl}), &requests)
	generated := m.formValues.Password
	require.Len(t, generated, 24)
	for range 4 {
		sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	}
	require.Len(t, requests, 1)
	assert.Equal(t, "Draft", requests[0].Entry.Title)
	assert.Equal(t, generated, requests[0].Entry.Password)
	sendForm(t, &m, SubmitFailedMsg{Error: &service.SaveError{Kind: service.SaveFailed, Err: errors.New("write failed")}}, &requests)
	sendForm(t, &m, tea.WindowSizeMsg{Width: 55, Height: 20}, &requests)
	assert.Len(t, requests, 1, "initialization and resizing must not retry")
	sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}), &requests)
	sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}), &requests)
	for _, char := range "Revised" {
		sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}), &requests)
	}
	assert.Equal(t, "Revised", m.formValues.Title, "restarted form must be focused and editable")
	for range 4 {
		sendForm(t, &m, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}), &requests)
	}
	require.Len(t, requests, 2)
	assert.NotEqual(t, requests[0].RequestID, requests[1].RequestID)
	assert.Equal(t, "Revised", requests[1].Entry.Title)
	assert.Equal(t, generated, requests[1].Entry.Password)
}

func TestFormCannotBeClosedOrChangedWhileSaving(t *testing.T) {
	m := NewEntryFormModel(locale.En, control.DefaultKeymap())
	m.formValues.Password = "existing draft"
	m.loading = true
	for _, key := range []tea.Key{{Code: tea.KeyEscape}, {Code: 'g', Mod: tea.ModCtrl}} {
		m, cmd := m.Update(tea.KeyPressMsg(key))
		assert.Nil(t, cmd)
		assert.True(t, m.loading)
		assert.Equal(t, "existing draft", m.formValues.Password)
	}
}

func TestEditSubmissionCapturesIDBeforeReset(t *testing.T) {
	m := NewEntryFormModel(locale.En, control.DefaultKeymap())
	id := gokeepasslib.NewUUID()
	m.Edit(id, service.NewVaultEntry{Title: "Draft", Password: "secret"})
	cmd := m.submitEntry()
	m.reset()
	msg, ok := cmd().(SubmitEditEntryMsg)
	require.True(t, ok)
	assert.Equal(t, id, msg.ID)
	assert.Equal(t, "Draft", msg.Entry.Title)
	assert.Equal(t, "secret", msg.Entry.Password)
}
