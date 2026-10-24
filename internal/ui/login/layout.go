package login

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
	tk "kzree.com/keepy/internal/textprovider/key"
)

func (m *LoginModel) setForm() {
	m.submitIntent = false
	if m.showPathFields {
		m.form = newLoginForm(m.formValues, m.locale, m.t)
	} else {
		m.form = newShortLoginForm(m.formValues, m.locale, m.t)
	}
	m.resizeForm()
}

func (m *LoginModel) rebuildForm() tea.Cmd {
	m.setForm()
	return m.form.Init()
}

func (m *LoginModel) SetSize(width, height int) {
	if m.width == width && m.height == height {
		return
	}
	m.width, m.height = width, height
	if m.form != nil {
		m.resizeForm()
	}
	if m.picking != pickerNone {
		m.picker.SetHeight(max(1, height-3))
	}
}

func (m LoginModel) summaryView() string {
	database := m.formValues.dbPath
	if database == "" {
		database = m.t.Text(tk.DatabaseNotSelected)
	}
	keyFile := fmt.Sprintf(m.t.Text(tk.KeyFileDisabled), control.Label(m.keys.LoginToggleKeyFile.Keys()...))
	if m.formValues.useKeyFile {
		if m.formValues.keyFilePath == "" {
			keyFile = fmt.Sprintf(m.t.Text(tk.KeyFileEnabledNoPath), control.Label(m.keys.LoginBrowseKeyFile.Keys()...))
		} else {
			keyFile = fmt.Sprintf(m.t.Text(tk.KeyFileEnabled), m.formValues.keyFilePath)
		}
	}
	text := fmt.Sprintf(m.t.Text(tk.SummaryDatabase), database) + "\n" + fmt.Sprintf(m.t.Text(tk.SummaryKeyFile), keyFile)
	return m.fit(text, max(0, m.height-lipgloss.Height(m.footerView())-2))
}

// authErrorText explains an authentication failure. Missing and unreadable
// files need no technical cause; the summary above already shows their paths.
func (m LoginModel) authErrorText() string {
	err := m.authError
	var fileErr *service.AuthFileError
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		return m.t.Text(tk.ErrorAuthCredentials)
	case errors.Is(err, service.ErrNotDatabase):
		return m.t.Text(tk.ErrorAuthNotDatabase)
	case errors.As(err, &fileErr) && fileErr.File == service.AuthKeyFile:
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return m.t.Text(tk.ErrorAuthKeyFileMissing)
		case errors.Is(err, fs.ErrPermission):
			return m.t.Text(tk.ErrorAuthKeyFilePermission)
		}
		return fmt.Sprintf(m.t.Text(tk.ErrorAuthKeyFile), err)
	case errors.As(err, &fileErr):
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return m.t.Text(tk.ErrorAuthDatabaseMissing)
		case errors.Is(err, fs.ErrPermission):
			return m.t.Text(tk.ErrorAuthDatabasePermission)
		}
		return fmt.Sprintf(m.t.Text(tk.ErrorAuthDatabase), err)
	}
	return err.Error()
}

func (m LoginModel) footerView() string {
	var parts []string
	if m.authError != nil {
		parts = append(parts, style.ErrorText.Render(m.authErrorText()))
	}
	parts = append(parts, strings.Join([]string{
		control.Hint(m.t.Text(tk.HintDatabase), m.keys.LoginBrowseDatabase.Keys()...),
		control.Hint(m.t.Text(tk.HintKeyFile), m.keys.LoginBrowseKeyFile.Keys()...),
		control.Hint(m.t.Text(tk.HintToggleKeyFile), m.keys.LoginToggleKeyFile.Keys()...),
		control.Hint(m.t.Text(tk.HintManualPaths), m.keys.LoginTogglePaths.Keys()...),
		control.Hint(m.t.Text(tk.HintHelp), m.keys.ShowHelpAnywhere.Keys()...),
	}, " · "))
	budget := 0
	if m.height > 0 {
		budget = min(3, m.height/4)
	}
	if m.height > 0 && budget == 0 {
		return ""
	}
	return m.fit(strings.Join(parts, "\n"), budget)
}

func (m LoginModel) fit(text string, height int) string {
	s := lipgloss.NewStyle()
	if m.width > 0 {
		s = s.Width(m.width).MaxWidth(m.width)
	}
	if m.height > 0 {
		if height <= 0 {
			return ""
		}
		s = s.MaxHeight(height)
	}
	return s.Render(text)
}

func (m LoginModel) formHeight() int {
	if m.height <= 0 {
		return 0
	}
	return max(1, m.height-lipgloss.Height(m.summaryView())-lipgloss.Height(m.footerView()))
}

func (m *LoginModel) resizeForm() {
	height := m.formHeight()
	m.form.WithShowHelp(m.height <= 0 || height >= 4).WithWidth(m.width).WithHeight(height)
	if m.width > 0 && m.height > 0 {
		// Rebuild Huh's viewport after resizing so its focused field stays visible.
		_, _ = m.form.Update(tea.WindowSizeMsg{Width: m.width, Height: height})
	}
}

func (m LoginModel) loginView() string {
	var parts []string
	if summary := m.summaryView(); summary != "" {
		parts = append(parts, summary)
	}
	body := m.form.View()
	if m.height > 0 {
		body = lipgloss.NewStyle().MaxHeight(m.formHeight()).Render(body)
	}
	parts = append(parts, body)
	if footer := m.footerView(); footer != "" {
		parts = append(parts, footer)
	}
	return strings.Join(parts, "\n")
}

func (m LoginModel) loadingView() string {
	body := m.spinner.View() + m.t.Text(tk.Authenticating)
	if summary := m.summaryView(); summary != "" {
		return summary + "\n" + body
	}
	return body
}
