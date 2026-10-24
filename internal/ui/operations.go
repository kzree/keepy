package ui

import (
	"errors"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
)

// vaultSession keeps the UI dependent on operation outcomes, not persistence
// internals. The production implementation is a shared *service.Vault.
type vaultSession interface {
	Authenticate(string, string, string, bool) error
	ReAuthenticate() error
	GetEntriesFlat() []service.VaultEntry
	GetEntryPassword(gokeepasslib.UUID) (string, error)
	AddNewEntry(service.NewVaultEntry) error
	UpdateEntry(gokeepasslib.UUID, service.NewVaultEntry) error
	DeleteEntry(gokeepasslib.UUID) error
}

type vaultOperationKind uint8

const (
	operationNone vaultOperationKind = iota
	operationLogin
	operationFormSave
	operationDelete
	operationReload
)

func (kind vaultOperationKind) usesGlobalSpinner() bool {
	return kind == operationDelete || kind == operationReload
}

type pendingVaultOperation struct {
	id        uint64
	kind      vaultOperationKind
	requestID uint64
}

// Commands capture only immutable input and the shared vault. Results carry
// snapshots so Update never waits on a vault lock held by a background writer.
type vaultOperationResult struct {
	operation   pendingVaultOperation
	entries     []service.VaultEntry
	err         error
	config      *service.Config
	configError error
}

func (r RootModel) canStartVaultOperation() bool {
	return r.pendingOperation.kind == operationNone && !r.quitAfterOperation
}

func (r *RootModel) beginVaultOperation(kind vaultOperationKind, requestID uint64) pendingVaultOperation {
	r.operationSequence++
	r.pendingOperation = pendingVaultOperation{id: r.operationSequence, kind: kind, requestID: requestID}
	if kind.usesGlobalSpinner() {
		r.operationSpinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	}
	r.resizeViews()
	return r.pendingOperation
}

// startVaultOperation owns common command plumbing. The callback captures only
// operation inputs and returns data; it must not access or update UI models.
// Callers validate admission and the source request before invoking it.
func (r RootModel) startVaultOperation(kind vaultOperationKind, requestID uint64, work func(vaultSession) vaultOperationResult) (RootModel, tea.Cmd) {
	op := r.beginVaultOperation(kind, requestID)
	vault := r.db
	cmd := func() tea.Msg {
		result := work(vault)
		result.operation = op
		return result
	}
	if kind.usesGlobalSpinner() {
		return r, tea.Batch(r.operationSpinner.Tick, cmd)
	}
	return r, cmd
}

func (r RootModel) startMutation(kind vaultOperationKind, requestID uint64, mutate func(vaultSession) error) (RootModel, tea.Cmd) {
	return r.startVaultOperation(kind, requestID, func(vault vaultSession) vaultOperationResult {
		result := vaultOperationResult{err: mutate(vault)}
		if saveCommitted(result.err) {
			result.entries = vault.GetEntriesFlat()
		}
		return result
	})
}

func saveCommitted(err error) bool {
	var outcome *service.SaveError
	return err == nil || (errors.As(err, &outcome) && outcome.Committed())
}

// finishFormSave closes a form for every committed outcome. Reporting a committed
// durability error as a form failure would offer a retry of an applied mutation.
func (r RootModel) finishFormSave(err error, entries []service.VaultEntry) (RootModel, tea.Cmd) {
	var result tea.Msg = entryform.SubmitFailedMsg{Error: err}
	if saveCommitted(err) {
		r.list.SetEntries(entries)
		result = entryform.SubmitSuccessMsg{Warning: err}
	}
	var cmd tea.Cmd
	r.list, cmd = r.list.Update(result)
	return r, cmd
}

func (r RootModel) handleVaultOperationResult(msg vaultOperationResult) (RootModel, tea.Cmd) {
	if r.pendingOperation.kind == operationNone || msg.operation != r.pendingOperation {
		return r, nil
	}
	r.pendingOperation = pendingVaultOperation{}
	var cmd tea.Cmd
	switch msg.operation.kind {
	case operationLogin:
		if msg.err != nil {
			r.login, cmd = r.login.Update(login.AuthenticationFailedMsg{Error: msg.err})
		} else {
			r.cfg, r.configSaveError = msg.config, msg.configError
			r.updateApplicationNotice()
			r.list.SetEntries(msg.entries)
			r.activeView = screenList
			cmd = r.list.Init()
		}
	case operationFormSave:
		r, cmd = r.finishFormSave(msg.err, msg.entries)
	case operationDelete:
		if saveCommitted(msg.err) {
			r.list.SetEntries(msg.entries)
		}
		r.list, cmd = r.list.Update(list.SaveResultMsg{Error: msg.err})
	case operationReload:
		if msg.err == nil {
			r.list.SetEntries(msg.entries)
		}
		r.list, cmd = r.list.Update(list.ReloadVaultResultMsg{Error: msg.err})
	}
	r.resizeViews()
	// A quit requested during a save waits for the outcome; it must not abandon
	// an operation whose atomic replacement may already have committed.
	if r.quitAfterOperation {
		return r.quit()
	}
	return r, cmd
}
