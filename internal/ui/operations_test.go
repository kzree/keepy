package ui

import (
	"errors"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
	"kzree.com/keepy/internal/util/testutil"
)

type gatedVault struct {
	*outcomeVault
	gate    string
	started chan struct{}
	release chan struct{}
}

func (v *gatedVault) wait(operation string) {
	if operation == v.gate {
		close(v.started)
		<-v.release
	}
}
func (v *gatedVault) Authenticate(db, key, password string, useKey bool) error {
	v.wait("login")
	return v.outcomeVault.Authenticate(db, key, password, useKey)
}
func (v *gatedVault) AddNewEntry(entry service.NewVaultEntry) error {
	v.wait("add")
	return v.outcomeVault.AddNewEntry(entry)
}
func (v *gatedVault) UpdateEntry(id gokeepasslib.UUID, entry service.NewVaultEntry) error {
	v.wait("update")
	return v.outcomeVault.UpdateEntry(id, entry)
}
func (v *gatedVault) DeleteEntry(id gokeepasslib.UUID) error {
	v.wait("delete")
	return v.outcomeVault.DeleteEntry(id)
}
func (v *gatedVault) ReAuthenticate() error { v.wait("reload"); return v.outcomeVault.ReAuthenticate() }

// captureVaultCommand drains focus/navigation until the root accepts an operation,
// then leaves its work command unexecuted so tests can control completion timing.
func captureVaultCommand(t *testing.T, r *RootModel, msg tea.Msg) tea.Cmd {
	t.Helper()
	cmd := updateRoot(r, msg)
	work := cmd
	if r.pendingOperation.kind == operationNone {
		work = nil
		testutil.DrainTeaCommands(t, cmd, func(msg tea.Msg) tea.Cmd {
			cmd := updateRoot(r, msg)
			if r.pendingOperation.kind != operationNone {
				work = cmd
				return nil
			}
			return cmd
		})
	}
	require.NotNil(t, work)
	if r.pendingOperation.kind == operationDelete || r.pendingOperation.kind == operationReload {
		// Those operations batch an initial spinner tick with the work command.
		batch, ok := work().(tea.BatchMsg)
		require.True(t, ok)
		require.Len(t, batch, 2)
		work = batch[1]
	}
	return work
}

func prepareDelayedOperation(t *testing.T, r *RootModel, operation string) tea.Cmd {
	t.Helper()
	switch operation {
	case "login":
		r.activeView = screenLogin
		r.login = login.NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: "vault.kdbx"}})
		drainRoot(t, r, r.login.Init())
		return captureVaultCommand(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	case "delete":
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: 'd'}))
		return captureVaultCommand(t, r, tea.KeyPressMsg(tea.Key{Code: 'y'}))
	case "reload":
		r.list, _ = r.list.Update(list.SaveResultMsg{Error: &service.SaveError{Kind: service.SaveConflict, Err: errors.New("external change")}})
		return captureVaultCommand(t, r, tea.KeyPressMsg(tea.Key{Code: 'r', Mod: tea.ModCtrl}))
	default:
		open := 'n'
		if operation == "update" {
			open = 'e'
		}
		sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: open}))
		for range 3 {
			sendRoot(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
		}
		return captureVaultCommand(t, r, tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	}
}

func TestDelayedVaultOperationsKeepUpdateResponsiveAndRejectReplay(t *testing.T) {
	for _, operation := range []string{"login", "add", "update", "delete", "reload"} {
		t.Run(operation, func(t *testing.T) {
			testutil.TempHome(t)
			id := gokeepasslib.NewUUID()
			fake := &outcomeVault{entries: []service.VaultEntry{{ID: id, Title: "Original"}}, reloaded: []service.VaultEntry{{ID: id, Title: "Reloaded"}}}
			if operation != "login" && operation != "reload" {
				fake.saveErr = &service.SaveError{Kind: service.SaveDurabilityUncertain, Err: errors.New("flush failed")}
			}
			vault := &gatedVault{outcomeVault: fake, gate: operation, started: make(chan struct{}), release: make(chan struct{})}
			r := rootWithVault(t, fake)
			r.db = vault
			r.clipboardReady = true
			r.writeClipboard = func([]byte) bool { return true }
			work := prepareDelayedOperation(t, &r, operation)
			initialReads := fake.passwordReads
			initialSnapshots := fake.snapshotReads.Load()
			pending := r.pendingOperation
			assert.Zero(t, fake.mutations, "accepting a request must not execute it")
			assert.Zero(t, fake.authCalls)
			results := make(chan tea.Msg, 1)
			finished := make(chan struct{})
			var once sync.Once
			release := func() { once.Do(func() { close(vault.release) }) }
			t.Cleanup(func() {
				release()
				select {
				case <-finished:
				case <-time.After(3 * time.Second):
					t.Error("worker did not stop")
				}
			})
			go func() { defer close(finished); results <- work() }()
			select {
			case <-vault.started:
			case <-time.After(3 * time.Second):
				t.Fatal("operation did not start")
			}
			// These updates run on the test goroutine while a worker is blocked.
			// Read counters detect accidental vault access from Update while busy.
			sendRoot(t, &r, tea.WindowSizeMsg{Width: 100, Height: 26})
			assert.Equal(t, 100, r.width)
			assert.NotEmpty(t, r.View().Content)
			sendRoot(t, &r, list.CopyPasswordRequestMsg{ID: id})
			sendRoot(t, &r, list.EditEntryRequestMsg{Entry: service.VaultEntry{ID: id}})
			sendRoot(t, &r, entryform.SubmitEntryMsg{RequestID: pending.requestID})
			sendRoot(t, &r, list.DeleteEntryRequestMsg{RequestID: pending.requestID, ID: id})
			assert.Equal(t, pending, r.pendingOperation)
			assert.Equal(t, initialSnapshots, fake.snapshotReads.Load())
			// Wrong completions cannot reset loading or publish another snapshot.
			stale := pending
			stale.id++
			sendRoot(t, &r, vaultOperationResult{operation: stale})
			assert.Equal(t, pending, r.pendingOperation)
			if operation == "delete" || operation == "reload" {
				_, tickCmd := r.Update(r.operationSpinner.Tick())
				assert.NotNil(t, tickCmd, "spinner must continue scheduling while work is pending")
			}
			release()
			var result tea.Msg
			select {
			case result = <-results:
			case <-time.After(3 * time.Second):
				t.Fatal("operation did not finish")
			}
			sendRoot(t, &r, result)
			assert.Equal(t, operationNone, r.pendingOperation.kind)
			sendRoot(t, &r, result) // Duplicate completion cannot apply twice.
			assert.Equal(t, initialReads, fake.passwordReads, "pending read requests must not wait on the vault writer")
			assert.Equal(t, initialSnapshots+1, fake.snapshotReads.Load(), "only the worker may collect the completion snapshot")
			if operation == "login" {
				assert.Equal(t, 1, fake.authCalls)
				assert.Equal(t, screenList, r.activeView)
			} else if operation == "reload" {
				assert.Equal(t, 1, fake.reloads)
				assert.Contains(t, r.View().Content, "Reloaded")
			} else {
				assert.Equal(t, 1, fake.mutations)
				assert.Contains(t, r.View().Content, "Do not repeat")
				if operation == "add" {
					assert.Len(t, fake.entries, 2)
				}
			}
		})
	}
}

func TestQuitWaitsForAcceptedSaveOutcome(t *testing.T) {
	for _, scenario := range []struct {
		name, operation string
		key             tea.Key
		quit            bool
		err             error
	}{
		{"save", "add", tea.Key{Code: 'c', Mod: tea.ModCtrl}, true, nil},
		{"failed save", "add", tea.Key{Code: 'c', Mod: tea.ModCtrl}, true, &service.SaveError{Kind: service.SaveFailed, Err: errors.New("failed")}},
		{"delete", "delete", tea.Key{Code: 'q'}, true, nil},
		{"reload", "reload", tea.Key{Code: 'q'}, true, nil},
		{"form text", "add", tea.Key{Code: 'q'}, false, nil},
		{"login text", "login", tea.Key{Code: 'q'}, false, nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testutil.TempHome(t)
			fake := &outcomeVault{entries: []service.VaultEntry{{ID: gokeepasslib.NewUUID(), Title: "Entry"}}, saveErr: scenario.err}
			r := rootWithVault(t, fake)
			work := prepareDelayedOperation(t, &r, scenario.operation)
			model, cmd := r.Update(tea.KeyPressMsg(scenario.key))
			r = model.(RootModel)
			assert.Nil(t, cmd)
			assert.Equal(t, scenario.quit, r.quitAfterOperation)
			model, cmd = r.Update(work())
			r = model.(RootModel)
			if scenario.quit {
				require.NotNil(t, cmd)
				assert.IsType(t, tea.QuitMsg{}, cmd())
			} else if cmd != nil {
				assert.NotEqual(t, tea.QuitMsg{}, cmd())
			}
			assert.Equal(t, operationNone, r.pendingOperation.kind)
		})
	}
}

func TestOldCompletionCannotReplaceNewerPendingSave(t *testing.T) {
	fake := &outcomeVault{}
	r := rootWithVault(t, fake)
	first := prepareDelayedOperation(t, &r, "add")
	oldResult := first()
	sendRoot(t, &r, oldResult)
	second := prepareDelayedOperation(t, &r, "add")
	pending := r.pendingOperation
	sendRoot(t, &r, oldResult)
	assert.Equal(t, pending, r.pendingOperation)
	assert.True(t, r.list.IsTextInputActive())
	assert.Len(t, fake.entries, 1)
	sendRoot(t, &r, second())
	assert.Equal(t, 2, fake.mutations)
	assert.Len(t, fake.entries, 2)
}
