package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

func TestHelpPreservesDraftAndQuestionMarksRemainInput(t *testing.T) {
	fake := &outcomeVault{}
	r := rootWithVault(t, fake)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	for _, char := range "Draft?" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	assert.False(t, r.helpVisible)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	assert.True(t, r.helpVisible)
	assert.Contains(t, r.View().Content, "Key bindings")
	assert.NotContains(t, r.View().Content, "Draft?")
	for _, char := range []rune{'n', 'd', 'y', 'c'} {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char}))
	}
	sendRoot(t, &r, tea.PasteMsg{Content: "must-not-reach-the-draft"})
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Zero(t, fake.mutations)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	assert.True(t, r.list.IsTextInputActive())
	assert.Contains(t, r.View().Content, "Draft?")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	for _, value := range []string{"user?", "https://example.test/?q=test", "secret?"} {
		for _, char := range value {
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
		}
		assert.False(t, r.helpVisible)
		if value != "secret?" {
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		}
	}
	request := captureEntrySubmission(t, &r, updateRoot(&r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})))
	assert.Equal(t, "user?", request.Entry.Username)
	assert.Equal(t, "https://example.test/?q=test", request.Entry.URL)
	assert.Equal(t, "secret?", request.Entry.Password)
	sendRoot(t, &r, request)
	require.Len(t, fake.entries, 1)
	assert.Equal(t, "Draft?", fake.entries[0].Title)
}

func TestHelpKeepsSelectionAndConfirmationState(t *testing.T) {
	id := gokeepasslib.NewUUID()
	fake := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "First"}, {ID: id, Title: "Second"}}, passwords: map[gokeepasslib.UUID]string{id: "selected-secret"}}
	r := rootWithVault(t, fake)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?'}))
	assert.True(t, r.helpVisible)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?'}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'e'}))
	assert.Contains(t, r.View().Content, "Second")
	assert.Contains(t, r.View().Content, "selected-secret")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'd'}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'y'}))
	assert.Zero(t, fake.mutations)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'y'}))
	assert.Equal(t, 1, fake.mutations)
	require.Len(t, fake.entries, 1)
	assert.NotEqual(t, id, fake.entries[0].ID)
}

func TestQuestionMarksInSearchAndLoginAreNotHelpShortcuts(t *testing.T) {
	r := rootWithVault(t, &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Match?"}, {ID: gokeepasslib.NewUUID(), Title: "Hidden"}}})
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'f'}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	assert.False(t, r.helpVisible)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Contains(t, r.View().Content, "Match?")
	assert.NotContains(t, r.View().Content, "Hidden")
	testutil.TempHome(t)
	fake := &outcomeVault{}
	r = rootWithVault(t, fake)
	r.activeView = screenLogin
	r.login = login.NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: "vault.kdbx"}}, locale.En)
	drainRoot(t, &r, r.login.Init())
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?', Text: "?"}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	assert.True(t, r.helpVisible)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	assert.Equal(t, "?", fake.authPassword)
}

func TestHelpAllowsPendingCompletionWithoutLosingNotices(t *testing.T) {
	for _, operation := range []string{"login", "add", "delete", "reload"} {
		t.Run(operation, func(t *testing.T) {
			testutil.TempHome(t)
			fake := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Original"}}}
			if operation == "add" {
				fake.saveErr = &service.SaveError{Kind: service.SaveDurabilityUncertain, Err: errors.New("flush failed")}
			}
			r := rootWithVault(t, fake)
			work := prepareDelayedOperation(t, &r, operation)
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
			assert.True(t, r.helpVisible)
			assert.Contains(t, r.View().Content, "operation in progress")
			sendRoot(t, &r, tea.WindowSizeMsg{Width: 100, Height: 24})
			sendRoot(t, &r, work())
			assert.True(t, r.helpVisible)
			assert.Equal(t, operationNone, r.pendingOperation.kind)
			assert.Contains(t, r.View().Content, "Key bindings")
			if operation == "add" {
				assert.Contains(t, r.View().Content, "Do not repeat")
			}
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
			assert.False(t, r.helpVisible)
			assert.Equal(t, screenList, r.activeView)
		})
	}
}

func TestHelpWorksDuringRecoveryAndHonorsQuit(t *testing.T) {
	r := rootWithVault(t, &outcomeVault{})
	r.activeView = screenLogin
	r.configLoadError = errors.New("malformed config")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: '?'}))
	assert.True(t, r.helpVisible, "startup recovery has no text input")
	cmd := updateRoot(&r, tea.KeyPressMsg(tea.Key{Code: 'q'}))
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
	// Opening help during a save must retain deferred-quit protection.
	r = rootWithVault(t, &outcomeVault{})
	work := prepareDelayedOperation(t, &r, "add")
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyF1}))
	assert.Nil(t, updateRoot(&r, tea.KeyPressMsg(tea.Key{Code: 'q'})))
	assert.True(t, r.quitAfterOperation)
	cmd = updateRoot(&r, work())
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}
