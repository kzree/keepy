// Package ui contains the user interface components of the application
package ui

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/ui/screens"
)

type RootModel struct {
	activeView screen

	db *service.Vault

	login screens.LoginModel
	list  screens.ListModel

	width  int
	height int
}

func NewRootModel() RootModel {
	return RootModel{
		activeView: screenLogin,
		login:      screens.NewLoginModel(),
		list:       screens.NewListModel(),
		db:         service.NewVault(),
	}
}

func (r RootModel) Init() tea.Cmd {
	switch r.activeView {
	case screenLogin:
		return r.login.Init()
	}

	return nil
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case screens.LoginSubmitMsg:
		creds := service.Credentials{
			DBPath:      msg.DBPath,
			KeyFilePath: msg.KeyFilePath,
		}
		service.SaveCredentials(&creds)

		if err := r.db.Authenticate(msg.DBPath, msg.KeyFilePath, msg.Password, msg.KeyFilePath != ""); err != nil {
			return r, func() tea.Msg {
				return screens.AuthenticationFailedMsg{Error: err}
			}
		}
		r.list.SetEntries(r.db.GetEntriesFlat())

		w, h := r.getContentSize()
		r.list.SetListTableSize(w, h)
		r.activeView = screenList
		return r, r.list.Init()
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return r, tea.Quit
		}

	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height

		r.list.SetListTableSize(r.width, r.height)
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
