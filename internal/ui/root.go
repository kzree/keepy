// Package ui contains the user interface components of the application
package ui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/ui/common"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
)

type RootModel struct {
	activeView screen

	db                 vaultSession
	cfg                *service.Config
	configLoadError    error
	configSaveError    error
	clipboardError     error
	copyError          error
	clipboardPending   bool
	clipboardReady     bool
	initClipboard      func() error
	writeClipboard     func([]byte) bool
	operationSequence  uint64
	pendingOperation   pendingVaultOperation
	operationSpinner   spinner.Model
	quitAfterOperation bool

	login       login.LoginModel
	list        list.ListModel
	bottomBar   common.BottomBarModel
	helpVisible bool
	keymapHelp  common.KeymapHelpModel

	width  int
	height int
}

func NewRootModel() RootModel {
	cfg, err := service.LoadConfig()

	r := RootModel{
		activeView:       screenLogin,
		login:            login.NewLoginModel(cfg),
		list:             list.NewListModel(),
		db:               service.NewVault(),
		cfg:              cfg,
		configLoadError:  err,
		clipboardPending: true,
		initClipboard:    initializeClipboard,
		writeClipboard:   writeClipboardText,
	}
	r.updateApplicationNotice()
	return r
}

func (r RootModel) Init() tea.Cmd {
	var loginCmd tea.Cmd
	if r.activeView == screenLogin && r.configLoadError == nil {
		loginCmd = r.login.Init()
	}
	init := r.initClipboard
	return tea.Batch(loginCmd, func() tea.Msg { return clipboardInitializedMsg{Error: init()} })
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case vaultOperationResult:
		return r.handleVaultOperationResult(msg)
	case spinner.TickMsg:
		if r.pendingOperation.kind.usesGlobalSpinner() {
			var cmd tea.Cmd
			r.operationSpinner, cmd = r.operationSpinner.Update(msg)
			return r, cmd
		}
	case clipboardInitializedMsg:
		if !r.clipboardPending {
			return r, nil
		}
		r.clipboardPending = false
		r.clipboardError = msg.Error
		r.clipboardReady = msg.Error == nil
		r.copyError = nil
		r.updateApplicationNotice()
		r.resizeViews()
		return r, nil
	case login.LoginSubmitMsg:
		return r.handleLoginSubmitMsg(msg)

	case list.CopyPasswordRequestMsg:
		return r.handleCopyPasswordRequestMsg(msg)

	case list.DeleteEntryRequestMsg:
		return r.handleDeleteEntryRequestMsg(msg)

	case list.EditEntryRequestMsg:
		return r.handleEditEntryRequestMsg(msg)
	case list.ReloadVaultRequestMsg:
		return r.handleReloadVault(msg)

	case tea.KeyPressMsg:
		if r, cmd, handled := r.handleKeyPressMsg(msg); handled {
			return r, cmd
		}

	case entryform.SubmitEntryMsg:
		return r.handleSubmitEntry(msg)

	case entryform.SubmitEditEntryMsg:
		return r.handleSubmitEditEntry(msg)

	case tea.WindowSizeMsg:
		return r.handleWindowSizeMsg(msg)
	case tea.MouseMsg:
		if r.helpVisible {
			var cmd tea.Cmd
			r.keymapHelp, cmd = r.keymapHelp.Update(msg)
			return r, cmd
		}
	case tea.PasteMsg:
		if r.helpVisible {
			return r, nil
		}
	}

	switch r.activeView {
	case screenLogin:
		if r.configLoadError != nil {
			return r, nil
		}
		login, cmd := r.login.Update(msg)
		r.login = login
		return r, cmd
	case screenList:
		list, cmd := r.list.Update(msg)
		r.list = list
		r.resizeViews()
		return r, cmd
	}

	return r, nil
}
