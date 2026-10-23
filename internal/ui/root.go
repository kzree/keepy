// Package ui contains the user interface components of the application
package ui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/ui/common"
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

	login     login.LoginModel
	list      list.ListModel
	bottomBar common.BottomBarModel

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
		if r.pendingOperation.kind == operationDelete || r.pendingOperation.kind == operationReload {
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

func (r *RootModel) renderCurrentView(contentWidth, contentHeight int) string {
	switch r.activeView {
	case screenLogin:
		if r.configLoadError != nil {
			return "Configuration unavailable\n\nCorrect the configuration file or its access permissions.\nPress Ctrl+R to retry loading it, or Ctrl+C to quit.\nThe existing file will not be overwritten."
		}
		return r.login.View()
	case screenList:
		return r.list.View(contentWidth, contentHeight)
	}

	return ""
}

func (r *RootModel) getContentSize() (int, int) {
	frameW, frameH := style.Frame.GetFrameSize()

	width := max(0, r.width-frameW)
	height := max(0, r.height-frameH)
	bar := r.renderBottomBar(width, height)
	if bar != "" {
		height -= lipgloss.Height(bar)
	}
	return width, height
}

func (r RootModel) renderBottomBar(width, height int) string {
	bar := r.bottomBar
	if r.activeView == screenList {
		status := r.list.GlobalNotice()
		if r.pendingOperation.kind == operationDelete || r.pendingOperation.kind == operationReload {
			status = r.operationSpinner.View() + " " + status
		}
		bar.SetStatus(status)
	}
	// Keep the footer compact and preserve room for the active view. Extremely
	// small terminals still get one row of prioritized recovery guidance.
	budget := min(height, min(5, max(1, height/4)))
	return bar.View(width, budget)
}

func (r *RootModel) resizeViews() {
	w, h := r.getContentSize()
	r.list.SetListTableSize(w, h)
}

func (r RootModel) View() tea.View {
	w, h := r.getContentSize()
	body := r.renderCurrentView(w, h)
	if w <= 0 || h <= 0 {
		body = ""
	} else {
		body = lipgloss.NewStyle().Width(w).Height(h).MaxWidth(w).MaxHeight(h).Render(body)
	}
	_, frameH := style.Frame.GetFrameSize()
	bar := r.renderBottomBar(w, max(0, r.height-frameH))
	if bar != "" {
		if body == "" {
			body = bar
		} else {
			body = lipgloss.JoinVertical(lipgloss.Left, body, bar)
		}
	}

	view := tea.NewView(r.renderFrame(body))
	view.AltScreen = true
	return view
}
