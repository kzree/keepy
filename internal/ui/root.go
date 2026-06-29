// Package ui contains the user interface components of the application
package ui

import (
	tea "charm.land/bubbletea/v2"
	"golang.design/x/clipboard"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/ui/list"
	"kzree.com/keepy/internal/ui/list/entryform"
	"kzree.com/keepy/internal/ui/login"
)

type RootModel struct {
	activeView screen

	db  *service.Vault
	cfg *service.Config

	login login.LoginModel
	list  list.ListModel

	width  int
	height int
}

func NewRootModel() RootModel {
	// TODO: improve error handling
	cfg, err := service.LoadConfig()
	if err != nil {
		panic("Failed to load config: " + err.Error())
	}

	return RootModel{
		activeView: screenLogin,
		login:      login.NewLoginModel(cfg),
		list:       list.NewListModel(),
		db:         service.NewVault(),
		cfg:        cfg,
	}
}

func (r RootModel) Init() tea.Cmd {
	err := clipboard.Init()
	// TODO: better error handling
	if err != nil {
		panic("Failed to initialize clipboard: " + err.Error())
	}

	switch r.activeView {
	case screenLogin:
		return r.login.Init()
	}

	return nil
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case login.LoginSubmitMsg:
		return r.handleLoginSubmitMsg(msg)

	case list.CopyPasswordRequestMsg:
		return r.handleCopyPasswordRequestMsg(msg)

	case list.DeleteEntryRequestMsg:
		return r.handleDeleteEntryRequestMsg(msg)

	case tea.KeyPressMsg:
		if r, cmd, handled := r.handleKeyPressMsg(msg); handled {
			return r, cmd
		}

	case entryform.SubmitEntryMsg:
		return r.handleSubmitEntry(msg)

	case tea.WindowSizeMsg:
		return r.handleWindowSizeMsg(msg)
	}

	switch r.activeView {
	case screenLogin:
		login, cmd := r.login.Update(msg)
		r.login = login
		return r, cmd
	case screenList:
		list, cmd := r.list.Update(msg)
		r.list = list
		return r, cmd
	}

	return r, nil
}

func (r *RootModel) renderCurrentView(contentWidth, contentHeight int) string {
	switch r.activeView {
	case screenLogin:
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
	return width, height
}

func (r RootModel) View() tea.View {
	w, h := r.getContentSize()
	body := r.renderCurrentView(w, h)

	return tea.NewView(r.renderFrame(body))
}
