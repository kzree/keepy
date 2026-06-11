// Package login implements the login screen with auth flow
package login

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
)

const authenticationText = "Authenticating..."

const (
	dbPathKey      = "dbPath"
	keyFilePathKey = "keyFilePath"
	passwordKey    = "pwd"
)

type LoginModel struct {
	form             *huh.Form
	spinner          spinner.Model
	loading          bool
	savedCredentials *service.Credentials
	authError        error
}

func newLoginForm(dbPath, keyFilePath string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Database path").
				Key(dbPathKey).
				Value(&dbPath),
			huh.NewInput().
				Title("Key file path").
				Key(keyFilePathKey).
				Value(&keyFilePath),
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				EchoMode(huh.EchoModePassword),
		),
	)
}

func NewLoginModel() LoginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	creds, _ := service.LoadSavedCredentials()

	return LoginModel{
		form:             newLoginForm(creds.DBPath, creds.KeyFilePath),
		spinner:          s,
		loading:          false,
		savedCredentials: creds,
	}
}

func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case AuthenticationFailedMsg:
		m.loading = false
		m.form = newLoginForm(m.savedCredentials.DBPath, m.savedCredentials.KeyFilePath)
		m.authError = msg.Error
		return m, m.form.Init()
	}

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

	body := m.form.View()

	if m.authError != nil {
		body += "\n" + style.ErrorText.Render(m.authError.Error())
	}

	return body
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
