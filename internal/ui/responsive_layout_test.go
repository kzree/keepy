package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
)

func TestDraftAndFocusedFieldSurviveSidebarFullWidthTransitions(t *testing.T) {
	fake := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "TableMarker"}}}
	r := rootWithVault(t, fake)
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 150, Height: 30})
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	for _, char := range "Draft" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	for _, char := range "before" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 55, Height: 14})
	assert.NotContains(t, r.View().Content, "TableMarker")
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 28, Height: 18}) // stacked field labels
	for _, char := range "after" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 160, Height: 32})
	assert.Contains(t, r.View().Content, "TableMarker")
	for range 2 {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
	for _, char := range "secret" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	request := captureEntrySubmission(t, &r, updateRoot(&r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})))
	assert.Equal(t, "Draft", request.Entry.Title)
	assert.Equal(t, "beforeafter", request.Entry.Username, "resizing must not reset focus to the title")
	assert.Equal(t, "secret", request.Entry.Password)
	sendRoot(t, &r, request)
	assert.Equal(t, 1, fake.mutations)
}

func TestEditingAcrossWidthsKeepsIdentityAndPendingSave(t *testing.T) {
	id := gokeepasslib.NewUUID()
	fake := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Original"}}, passwords: map[gokeepasslib.UUID]string{id: "secret"}}
	r := rootWithVault(t, fake)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'e'}))
	for _, size := range [][2]int{{42, 12}, {150, 30}, {60, 18}} {
		sendRoot(t, &r, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
	}
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}))
	for _, char := range "Edited" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	for range 3 {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
	work := captureVaultCommand(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	pending := r.pendingOperation
	sendRoot(t, &r, tea.WindowSizeMsg{Width: 24, Height: 8})
	assert.Equal(t, pending, r.pendingOperation)
	assert.Contains(t, r.View().Content, "Saving entry")
	sendRoot(t, &r, work())
	require.Len(t, fake.entries, 1)
	assert.Equal(t, id, fake.entries[0].ID)
	assert.Equal(t, "Edited", fake.entries[0].Title)
	assert.Equal(t, 1, fake.mutations)
}

func TestResponsiveRecoveryAndHelpRemainWithinRootBudget(t *testing.T) {
	fake := &outcomeVault{saveErr: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}}
	r := rootWithVault(t, fake)
	submitThroughUI(t, &r, "add")
	for _, size := range [][2]int{{48, 16}, {100, 24}, {24, 8}, {150, 32}} {
		sendRoot(t, &r, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		assert.LessOrEqual(t, lipgloss.Width(r.View().Content), size[0])
		assert.LessOrEqual(t, lipgloss.Height(r.View().Content), size[1])
		assert.Contains(t, r.View().Content, "Ctrl+R")
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
		assert.LessOrEqual(t, lipgloss.Height(r.View().Content), size[1])
		assert.Contains(t, r.View().Content, "Ctrl+R")
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
		assert.True(t, r.list.IsTextInputActive())
	}
	assert.Equal(t, 1, fake.mutations)
}
