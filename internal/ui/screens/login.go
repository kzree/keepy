// Package screens contains different screens for the application
package screens

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

type LoginModel struct {
	form *huh.Form
}

func NewLoginModel() LoginModel {
	return LoginModel{
		form: huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Database path").
					Key("dbPath").
					Placeholder("/path/to/database.db"),
				huh.NewInput().
					Title("Key file path").
					Key("keyFilePath").
					Placeholder("/path/to/keyfile.key"),
				huh.NewInput().
					Title("Password").
					Key("pwd").
					Placeholder("Your password"),
			),
		),
	}
}

func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	return m, cmd
}

func (m LoginModel) View() string {
	return m.form.View()
}
