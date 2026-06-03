// Package ui contains the user interface components of the application
package ui

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/ui/screens"
)

type RootModel struct {
	activeView screen

	login screens.LoginModel
	list  screens.ListModel

	width  int
	height int
}

func NewRootModel() RootModel {
	return RootModel{
		activeView: screenLogin,
		login:      screens.NewLoginModel(),
	}
}

func (r RootModel) Init() tea.Cmd {
	switch r.activeView {
	case screenLogin:
		return r.login.Init()
	case screenList:
		return r.list.Init()
	}

	return nil
}

func (r RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return r, tea.Quit

		case "up", "k":
		case "down", "j":
			if r.activeView == screenLogin {
				r.activeView = screenList
			} else {
				r.activeView = screenLogin
			}
		}

	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
	}

	switch r.activeView {
	case screenLogin:
		login, cmd := r.login.Update(msg)
		r.login = login
		return r, cmd
	}

	return r, nil
}

func (r *RootModel) renderCurrentView() string {
	switch r.activeView {
	case screenLogin:
		return r.login.View()
	case screenList:
		return r.list.View().Content
	}

	return ""
}

func (r RootModel) View() tea.View {
	body := r.renderCurrentView()
	return tea.NewView(r.renderFrame(body))
}
