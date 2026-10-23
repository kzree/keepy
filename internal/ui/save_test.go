package ui

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/util/testutil"
)

type outcomeVault struct {
	snapshotReads   atomic.Int64
	authCalls       int
	authCredentials service.Credentials
	authPassword    string
	authUsesKey     bool
	passwordReads   int
	authErr         error
	entries         []service.VaultEntry
	passwords       map[gokeepasslib.UUID]string
	saveErr         error
	reloadErr       error
	reloaded        []service.VaultEntry
	mutations       int
	reloads         int
}

func (v *outcomeVault) Authenticate(dbPath, keyPath, password string, useKey bool) error {
	v.authCalls++
	v.authCredentials = service.Credentials{DBPath: dbPath, KeyFilePath: keyPath}
	v.authPassword = password
	v.authUsesKey = useKey
	return v.authErr
}
func (v *outcomeVault) ReAuthenticate() error {
	v.reloads++
	if v.reloadErr == nil {
		v.entries = v.reloaded
	}
	return v.reloadErr
}
func (v *outcomeVault) GetEntriesFlat() []service.VaultEntry {
	v.snapshotReads.Add(1)
	return append([]service.VaultEntry(nil), v.entries...)
}
func (v *outcomeVault) GetEntryPassword(id gokeepasslib.UUID) (string, error) {
	v.passwordReads++
	return v.passwords[id], nil
}
func (v *outcomeVault) AddNewEntry(entry service.NewVaultEntry) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		v.entries = append(v.entries, service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: entry.Title})
	}
	return v.saveErr
}
func (v *outcomeVault) UpdateEntry(id gokeepasslib.UUID, entry service.NewVaultEntry) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		for i := range v.entries {
			if v.entries[i].ID == id {
				v.entries[i].Title = entry.Title
			}
		}
	}
	return v.saveErr
}
func (v *outcomeVault) DeleteEntry(id gokeepasslib.UUID) error {
	v.mutations++
	if saveCommitted(v.saveErr) {
		for i := range v.entries {
			if v.entries[i].ID == id {
				v.entries = append(v.entries[:i], v.entries[i+1:]...)
				break
			}
		}
	}
	return v.saveErr
}

func rootWithVault(t *testing.T, vault *outcomeVault) RootModel {
	t.Helper()
	r := RootModel{activeView: screenList, db: vault, list: list.NewListModel(), width: 120, height: 30}
	r.list.SetEntries(vault.GetEntriesFlat())
	r.list.SetListTableSize(110, 25)
	return r
}

func sendRoot(t *testing.T, r *RootModel, msg tea.Msg) {
	t.Helper()
	model, cmd := r.Update(msg)
	*r = model.(RootModel)
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
		model, cmd := r.Update(msg)
		*r = model.(RootModel)
		return cmd
	})
}

func submitThroughUI(t *testing.T, r *RootModel, operation string) {
	t.Helper()
	if operation == "delete" {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'd'}))
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'y'}))
		return
	}
	open := 'n'
	if operation == "update" {
		open = 'e'
	}
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: open}))
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'k', Mod: tea.ModCtrl}))
	for _, char := range "Committed" {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	for range 4 {
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
}

func TestCommittedSaveOutcomesRefreshWithoutRepeatingMutation(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete"} {
		for _, uncertain := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/uncertain=%t", operation, uncertain), func(t *testing.T) {
				id := gokeepasslib.NewUUID()
				vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Original"}}}
				if uncertain {
					vault.saveErr = fmt.Errorf("wrapped: %w", &service.SaveError{Kind: service.SaveDurabilityUncertain, Err: errors.New("flush failed")})
				}
				r := rootWithVault(t, vault)
				submitThroughUI(t, &r, operation)
				assert.False(t, r.list.IsTextInputActive(), "committed form must close")
				if operation == "delete" {
					assert.Empty(t, vault.entries)
				} else {
					assert.Contains(t, r.View().Content, "Committed")
				}
				if uncertain {
					assert.Contains(t, r.View().Content, "Do not repeat")
				}
				for range 3 {
					model, _ := r.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
					r = model.(RootModel)
				}
				assert.Equal(t, 1, vault.mutations)
				if operation == "add" {
					assert.Len(t, vault.entries, 2, "addition must occur exactly once")
				}
			})
		}
	}
}

func TestUncommittedSaveErrorsRemainVisibleAndDoNotPanic(t *testing.T) {
	for _, kind := range []service.SaveErrorKind{service.SaveFailed, service.SaveConflict} {
		for _, operation := range []string{"add", "update", "delete"} {
			t.Run(fmt.Sprintf("%s/%d", operation, kind), func(t *testing.T) {
				id := gokeepasslib.NewUUID()
				vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Original"}}, saveErr: &service.SaveError{Kind: kind, Err: errors.New("savefailure")}}
				r := rootWithVault(t, vault)
				submitThroughUI(t, &r, operation)
				assert.Len(t, vault.entries, 1)
				assert.Contains(t, r.View().Content, "savefailure")
				assert.Equal(t, operation != "delete", r.list.IsTextInputActive())
				assert.Equal(t, 1, vault.mutations)
			})
		}
	}
}

func TestConflictReloadIsExplicitAndDoesNotRepeatMutation(t *testing.T) {
	vault := &outcomeVault{saveErr: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}, reloaded: []service.VaultEntry{{Title: "External entry"}}}
	r := rootWithVault(t, vault)
	submitThroughUI(t, &r, "add")
	assert.Zero(t, vault.reloads)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	assert.Equal(t, 1, vault.reloads)
	assert.Equal(t, 1, vault.mutations)
	assert.Contains(t, r.View().Content, "External entry")
	assert.Contains(t, r.View().Content, "reloaded")
	assert.True(t, r.list.IsTextInputActive())
}

func TestDuplicateConfirmationAndQueuedRequestsCannotRepeatDelete(t *testing.T) {
	id := gokeepasslib.NewUUID()
	vault := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Original"}}, saveErr: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}}
	r := rootWithVault(t, vault)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'd'}))
	model, confirmCmd := r.Update(tea.KeyPressMsg(tea.Key{Code: 'y'}))
	r = model.(RootModel)
	require.NotNil(t, confirmCmd)
	confirmation := confirmCmd().(common.CloseConfirmMsg)
	model, request := r.Update(confirmation)
	r = model.(RootModel)
	require.NotNil(t, request)
	model, duplicate := r.Update(confirmation)
	r = model.(RootModel)
	assert.Nil(t, duplicate)
	model, whilePending := r.Update(tea.KeyPressMsg(tea.Key{Code: 'n'}))
	r = model.(RootModel)
	assert.Nil(t, whilePending)
	msg := request()
	sendRoot(t, &r, msg)
	sendRoot(t, &r, msg)
	assert.Equal(t, 1, vault.mutations)
	assert.Contains(t, r.View().Content, "ctrl+r")
}

func TestSaveNoticesFitTerminalHeightWithPopulatedList(t *testing.T) {
	for _, kind := range []service.SaveErrorKind{service.SaveConflict, service.SaveDurabilityUncertain} {
		vault := &outcomeVault{saveErr: &service.SaveError{Kind: kind, Err: errors.New("filesystem failure")}}
		for range 30 {
			vault.entries = append(vault.entries, service.VaultEntry{ID: gokeepasslib.NewUUID(), Title: "Entry"})
		}
		r := rootWithVault(t, vault)
		sendRoot(t, &r, tea.WindowSizeMsg{Width: 120, Height: 24})
		submitThroughUI(t, &r, "delete")
		assert.LessOrEqual(t, lipgloss.Height(r.View().Content), 24)
		if kind == service.SaveConflict {
			assert.Contains(t, r.View().Content, "ctrl+r")
		} else {
			assert.Contains(t, r.View().Content, "Do not repeat")
		}
	}
}

func TestDeferredSubmissionIsAcceptedAtMostOnce(t *testing.T) {
	vault := &outcomeVault{saveErr: &service.SaveError{Kind: service.SaveDurabilityUncertain, Err: errors.New("flush failed")}}
	r := rootWithVault(t, vault)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	for range 3 {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
	model, cmd := r.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	r = model.(RootModel)
	var request entryform.SubmitEntryMsg
	testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
		if submit, ok := msg.(entryform.SubmitEntryMsg); ok {
			request = submit
			return nil
		}
		model, cmd := r.Update(msg)
		r = model.(RootModel)
		return cmd
	})
	require.NotZero(t, request.RequestID)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape})) // Pending form cannot close.
	assert.True(t, r.list.IsTextInputActive())
	sendRoot(t, &r, request)
	sendRoot(t, &r, request)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	sendRoot(t, &r, request) // Stale request cannot affect a new form.
	assert.Equal(t, 1, vault.mutations)
	assert.Len(t, vault.entries, 1)
	assert.True(t, r.list.IsTextInputActive())
}

func TestEscapeClosesSynchronouslyWithoutAffectingNewDraft(t *testing.T) {
	vault := &outcomeVault{}
	r := rootWithVault(t, vault)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	model, oldInit := r.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	r = model.(RootModel)
	assert.False(t, r.list.IsTextInputActive(), "Escape must close before any command is delivered")
	model, secondEscape := r.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	r = model.(RootModel)
	assert.Nil(t, secondEscape)
	sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'n'}))
	for _, char := range "Newdraft" {
		sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
	}
	for phase := range 2 {
		var request entryform.SubmitEntryMsg
		if phase == 1 {
			for range 3 {
				sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			}
			model, cmd := r.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			r = model.(RootModel)
			testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
				if submit, ok := msg.(entryform.SubmitEntryMsg); ok {
					request = submit
					return nil
				}
				model, cmd := r.Update(msg)
				r = model.(RootModel)
				return cmd
			})
			require.NotZero(t, request.RequestID)
		}
		testutil.DrainTeaCommands(t, oldInit, func(msg tea.Msg) tea.Cmd {
			model, cmd := r.Update(msg)
			r = model.(RootModel)
			return cmd
		})
		assert.True(t, r.list.IsTextInputActive())
		if phase == 1 {
			sendRoot(t, &r, request)
		}
	}
	require.Len(t, vault.entries, 1)
	assert.Equal(t, "Newdraft", vault.entries[0].Title)
	assert.Equal(t, 1, vault.mutations)
}

func TestMutationAndReloadRefreshKeepActiveSearch(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete", "reload"} {
		t.Run(operation, func(t *testing.T) {
			vault := &outcomeVault{entries: []service.VaultEntry{
				{ID: gokeepasslib.NewUUID(), Title: "Match original"},
				{ID: gokeepasslib.NewUUID(), Title: "Hidden unrelated"},
			}}
			r := rootWithVault(t, vault)
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'f'}))
			for _, char := range "match" {
				sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: char, Text: string(char)}))
			}
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
			if operation == "reload" {
				vault.saveErr = &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}
				vault.reloaded = []service.VaultEntry{
					{ID: gokeepasslib.NewUUID(), Title: "Match external"},
					{ID: gokeepasslib.NewUUID(), Title: "Hidden external"},
				}
				submitThroughUI(t, &r, "delete")
				sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
				assert.Contains(t, r.View().Content, "Match external")
				assert.NotContains(t, r.View().Content, "Hidden external")
				assert.Equal(t, 1, vault.reloads)
				assert.Len(t, vault.entries, 2)
			} else {
				submitThroughUI(t, &r, operation)
				assert.NotContains(t, r.View().Content, "Committed", "nonmatching additions/edits must remain filtered out")
				switch operation {
				case "add":
					require.Len(t, vault.entries, 3)
					assert.Equal(t, "Committed", vault.entries[2].Title)
					assert.Contains(t, r.View().Content, "Match original")
				case "update":
					assert.Equal(t, "Committed", vault.entries[0].Title)
					assert.NotContains(t, r.View().Content, "Match original")
				case "delete":
					require.Len(t, vault.entries, 1)
					assert.Equal(t, "Hidden unrelated", vault.entries[0].Title)
					assert.NotContains(t, r.View().Content, "Match original")
				}
			}
			assert.Equal(t, 1, vault.mutations)
			assert.NotContains(t, r.View().Content, "Hidden unrelated")
			sendRoot(t, &r, tea.KeyPressMsg(tea.Key{Code: 'f'}))
			assert.Contains(t, r.View().Content, "match", "the search input must retain the query")
		})
	}
}
