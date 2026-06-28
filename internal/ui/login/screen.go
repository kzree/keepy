// Package login implements the login screen with auth flow
package login

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	"kzree.com/keepy/internal/util"
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
	showPathFields   bool
	formValues       *loginFormValues
}

func NewLoginModel(cfg *service.Config) LoginModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	if cfg == nil {
		cfg = &service.Config{}
	}

	creds := &cfg.Credentials

	showPathFields := creds.DBPath == ""

	formValues := loginFormValues{
		password:    "",
		dbPath:      creds.DBPath,
		keyFilePath: creds.KeyFilePath,
	}

	return LoginModel{
		form:             util.Ternary(showPathFields, newLoginForm(&formValues), newShortLoginForm(&formValues)),
		spinner:          s,
		loading:          false,
		savedCredentials: creds,
		showPathFields:   showPathFields,
		formValues:       &formValues,
	}
}

func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case AuthenticationFailedMsg:
		return m.handleAuthenticationFailedMsg(msg)
	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+t":
			return m.handleTogglePathFields()
		}
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
	msg := LoginSubmitMsg{
		DBPath:      m.formValues.dbPath,
		KeyFilePath: m.formValues.keyFilePath,
		Password:    m.formValues.password,
	}

	return func() tea.Msg {
		return msg
	}
}
