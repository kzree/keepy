package ui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/login"
)

type vaultOperationKind uint8

const (
	operationNone vaultOperationKind = iota
	operationLogin
	operationFormSave
	operationDelete
	operationReload
)

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
	if kind == operationDelete || kind == operationReload {
		r.operationSpinner = spinner.New(spinner.WithSpinner(spinner.Dot))
	}
	r.resizeViews()
	return r.pendingOperation
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
		return r, tea.Quit
	}
	return r, cmd
}
