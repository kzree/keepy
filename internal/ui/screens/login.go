// Package screens contains different screens for the application
package screens

import tea "charm.land/bubbletea/v2"

type LoginModel struct{}

func NewLoginModel() LoginModel {
	return LoginModel{}
}

func (m LoginModel) Init() tea.Cmd {
	return nil
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	return m, nil
}

func (m LoginModel) View() tea.View {
	return tea.NewView("Login")
}
