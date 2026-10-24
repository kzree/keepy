// Package login implements the login screen with auth flow
package login

import (
	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

const (
	dbPathKey      = "dbPath"
	keyFilePathKey = "keyFilePath"
	passwordKey    = "pwd"
)

type LoginModel struct {
	form           *huh.Form
	spinner        spinner.Model
	loading        bool
	authError      error
	showPathFields bool
	formValues     *loginFormValues
	requestID      uint64
	submitIntent   bool
	width, height  int
	picker         filepicker.Model
	picking        pickerTarget
	pickerSession  uint64
	pickerRequest  uint64
	pickerLoading  bool
	pickerError    error
	locale         locale.Locale
	t              *textprovider.TextProvider
	keys           control.Keymap
}

func NewLoginModel(cfg *service.Config, l locale.Locale, keys control.Keymap) LoginModel {
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
		useKeyFile:  creds.KeyFilePath != "",
	}

	m := LoginModel{
		spinner:        s,
		loading:        false,
		showPathFields: showPathFields,
		formValues:     &formValues,
		locale:         l,
		t:              textprovider.New(l, namespace.Login),
		keys:           keys,
	}
	m.setForm()
	return m
}

func (m LoginModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case pickerResultMsg:
		return m.handlePickerResult(msg)
	case AuthenticationFailedMsg:
		return m.handleAuthenticationFailedMsg(msg)
	case tea.KeyPressMsg:
		if m.loading {
			return m, nil
		}
		if m.picking != pickerNone {
			return m.updatePicker(msg)
		}
		switch {
		case key.Matches(msg, m.keys.LoginTogglePaths):
			return m.handleTogglePathFields()
		case key.Matches(msg, m.keys.LoginBrowseDatabase):
			return m.openPicker(pickerDatabase)
		case key.Matches(msg, m.keys.LoginBrowseKeyFile):
			return m.openPicker(pickerKeyFile)
		case key.Matches(msg, m.keys.LoginToggleKeyFile):
			m.formValues.useKeyFile = !m.formValues.useKeyFile
			if m.formValues.useKeyFile && m.formValues.keyFilePath == "" {
				m.showPathFields = true
			}
			cmd := m.rebuildForm()
			return m, cmd
		}
		if msg.String() == "enter" && m.lastFieldReady() {
			m.submitIntent = true
		}
	}
	if m.picking != pickerNone {
		return m, nil
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	m.resizeForm()
	cmds = append(cmds, cmd)

	if m.form.State == huh.StateCompleted && !m.loading {
		// Huh completes navigation through commands. Browsing or rebuilding can
		// invalidate an earlier Enter; a late completion must not authenticate.
		if !m.submitIntent {
			cmd := m.rebuildForm()
			return m, cmd
		}
		m.loading = true
		m.requestID++
		m.authError = nil
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
	if m.picking != pickerNone {
		return m.pickerView()
	}
	if m.loading {
		return m.loadingView()
	}

	return m.loginView()
}

func (m LoginModel) lastFieldReady() bool {
	last := passwordKey
	if m.showPathFields {
		last = dbPathKey
		if m.formValues.useKeyFile {
			last = keyFilePathKey
		}
	}
	field := m.form.GetFocusedField()
	return field != nil && field.GetKey() == last && m.formValues.dbPath != "" && (!m.formValues.useKeyFile || m.formValues.keyFilePath != "")
}

func (m *LoginModel) submitAuth() tea.Cmd {
	msg := LoginSubmitMsg{
		RequestID:   m.requestID,
		DBPath:      m.formValues.dbPath,
		KeyFilePath: m.formValues.keyFilePath,
		Password:    m.formValues.password,
	}
	if !m.formValues.useKeyFile {
		msg.KeyFilePath = ""
	}

	return func() tea.Msg {
		return msg
	}
}

func (m LoginModel) AcceptsSubmission(id uint64) bool { return m.loading && m.requestID == id }

func (m LoginModel) IsTextInputActive() bool { return m.picking == pickerNone }

func (m LoginModel) KeymapContext() string {
	if m.picking != pickerNone {
		return control.FilePickerKeys
	}
	return control.LoginKeys
}
