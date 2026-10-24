package login

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/style"
)

type pickerTarget uint8

const (
	pickerNone pickerTarget = iota
	pickerDatabase
	pickerKeyFile
)

// Every directory command is scoped to the picker session and request. Cancelling
// or switching pickers must not apply a late directory listing to a newer view.
type pickerResultMsg struct {
	session, request uint64
	directory        string
	message          tea.Msg
	err              error
	initial          bool
}

func (m LoginModel) openPicker(target pickerTarget) (LoginModel, tea.Cmd) {
	m.submitIntent = false
	m.pickerSession++
	m.pickerRequest++
	m.picking, m.pickerLoading, m.pickerError = target, true, nil
	m.newPicker(".")
	session, request := m.pickerSession, m.pickerRequest
	return m, func() tea.Msg {
		dir, err := os.UserHomeDir()
		if err == nil {
			dir, err = filepath.Abs(dir)
		}
		return pickerResultMsg{session: session, request: request, directory: dir, err: err, initial: true}
	}
}

func (m *LoginModel) newPicker(directory string) {
	m.picker = filepicker.New()
	m.picker.CurrentDirectory = directory
	m.picker.KeyMap = control.LoginPickerKeyMap()
	m.picker.AutoHeight = false
	m.picker.ShowHidden = true
	m.picker.ShowPermissions, m.picker.ShowSize = false, false
	m.picker.Styles.Cursor = lipgloss.NewStyle().Foreground(lipgloss.BrightBlue)
	m.picker.Styles.Selected = m.picker.Styles.Cursor.Bold(true)
	m.picker.Styles.EmptyDirectory = lipgloss.NewStyle().SetString("No files found, or directory unavailable. Ctrl+R retries.")
	m.picker.SetHeight(max(1, m.height-3))
}

// Bubbles keeps directory-read errors private. Probe one entry in the worker to
// surface access failures without depending on its private message types. The
// original command still provides the listing consumed by the component.
func pickerDirectoryAccess(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	_, err = file.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	return errors.Join(err, file.Close())
}

func (m *LoginModel) readPickerDirectory() tea.Cmd {
	// A fresh listing prevents a private Bubbles read error from leaving entries
	// from the previous directory selectable under a different path.
	m.newPicker(m.picker.CurrentDirectory)
	cmd := m.picker.Init()
	m.pickerLoading, m.pickerError = true, nil
	m.pickerRequest++
	session, request, directory := m.pickerSession, m.pickerRequest, m.picker.CurrentDirectory
	return func() tea.Msg {
		if err := pickerDirectoryAccess(directory); err != nil {
			return pickerResultMsg{session: session, request: request, err: err}
		}
		return pickerResultMsg{session: session, request: request, message: cmd()}
	}
}

func (m LoginModel) handlePickerResult(msg pickerResultMsg) (LoginModel, tea.Cmd) {
	if m.picking == pickerNone || msg.session != m.pickerSession || msg.request != m.pickerRequest {
		return m, nil
	}
	m.pickerLoading = false
	m.pickerError = msg.err
	if msg.err != nil {
		return m, nil
	}
	if msg.initial {
		m.picker.CurrentDirectory = msg.directory
		cmd := m.readPickerDirectory()
		return m, cmd
	}
	m.picker, _ = m.picker.Update(msg.message)
	return m, nil
}

func (m LoginModel) updatePicker(msg tea.KeyPressMsg) (LoginModel, tea.Cmd) {
	if msg.String() == control.CloseEntryForm {
		m.picking = pickerNone
		m.pickerSession++
		m.pickerError = nil
		return m, m.form.Init()
	}
	if m.pickerLoading {
		return m, nil
	}
	if msg.String() == control.ReloadVault {
		if m.picker.CurrentDirectory == "." {
			return m.openPicker(m.picking)
		}
		cmd := m.readPickerDirectory()
		return m, cmd
	}
	if m.pickerError != nil && msg.String() != "left" && msg.String() != "backspace" && msg.String() != "h" {
		return m, nil
	}
	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if selected, path := m.picker.DidSelectFile(msg); selected {
		info, err := os.Stat(path)
		if err != nil {
			m.pickerError = err
			return m, nil
		}
		if !info.Mode().IsRegular() {
			m.pickerError = errors.New("select a regular file")
			return m, nil
		}
		if m.picking == pickerDatabase {
			m.formValues.dbPath = path
		} else {
			m.formValues.keyFilePath = path
			m.formValues.useKeyFile = true
		}
		m.picking = pickerNone
		m.pickerSession++
		m.authError = nil
		cmd = m.rebuildForm()
		return m, cmd
	}
	if cmd != nil {
		cmd = m.readPickerDirectory()
	}
	return m, cmd
}

func (m LoginModel) pickerView() string {
	title := "Choose database"
	if m.picking == pickerKeyFile {
		title = "Choose key file"
	}
	body := m.picker.View()
	if m.pickerLoading {
		body = "Reading directory..."
	}
	if m.pickerError != nil {
		body = style.ErrorText.Render(m.pickerError.Error())
	}
	footer := "Esc cancel · Enter select/open · Left parent · Ctrl+R retry · F1 help"
	if m.width > 0 {
		footer = ansi.Truncate(footer, m.width, "…")
		title = ansi.Truncate(title, m.width, "…")
	}
	if m.height == 1 {
		return footer
	}
	if m.height == 2 {
		return title + "\n" + footer
	}
	header := title
	headerHeight := 1
	if m.height <= 0 || m.height > 3 {
		directory := m.picker.CurrentDirectory
		if m.width > 0 {
			directory = ansi.Truncate(directory, m.width, "…")
		}
		header += "\n" + directory
		headerHeight++
	}
	if m.height > 0 {
		body = lipgloss.NewStyle().MaxHeight(max(1, m.height-headerHeight-1)).Render(body)
	}
	if m.width > 0 {
		body = lipgloss.NewStyle().MaxWidth(m.width).Render(body)
	}
	return header + "\n" + body + "\n" + footer
}
