// Package screens contains different screens for the application
package screens

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
)

const authenticationText = "Authenticating..."

const (
	dbPathKey      = "dbPath"
	keyFilePathKey = "keyFilePath"
	passwordKey    = "pwd"
)

type LoginSubmitMsg struct {
	DBPath      string
	KeyFilePath string
	Password    string
}

type LoginModel struct {
	form    *huh.Form
	spinner spinner.Model
	loading bool
}

func NewLoginModel() LoginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	creds, _ := service.LoadSavedCredentials()

	return LoginModel{
		form: huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Database path").
					Key(dbPathKey).
					Value(&creds.DBPath),
				huh.NewInput().
					Title("Key file path").
					Key(keyFilePathKey).
					Value(&creds.KeyFilePath),
				huh.NewInput().
					Title("Password").
					Key(passwordKey).
					EchoMode(huh.EchoModePassword),
			),
		),
		spinner: s,
		loading: false,
	}
}

func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmds []tea.Cmd

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	cmds = append(cmds, cmd)

	if m.form.State == huh.StateCompleted && !m.loading {
		m.loading = true
		return m, tea.Batch(cmd, m.spinner.Tick, m.submitAuth())
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

func (m LoginModel) View() string {
	if m.loading {
		return m.spinner.View() + authenticationText
	}

	return m.form.View()
}

func (m *LoginModel) submitAuth() tea.Cmd {
	return func() tea.Msg {
		return LoginSubmitMsg{
			DBPath:      m.form.GetString(dbPathKey),
			KeyFilePath: m.form.GetString(keyFilePathKey),
			Password:    m.form.GetString(passwordKey),
		}
	}
}
