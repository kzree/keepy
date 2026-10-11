package entryform

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"errors"
	"fmt"
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
)

const savingText = "Saving entry..."

type EntryFormModel struct {
	form        *huh.Form
	loading     bool
	spinner     spinner.Model
	submitError error
	formValues  *entryFormValues
	editingID   *gokeepasslib.UUID
	conflict    bool
	reviewDraft bool
	width       int
	height      int
	requestID   uint64
}

func NewEntryFormModel() EntryFormModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	values := getEmptyFormValues()
	return EntryFormModel{
		formValues: values,
		form:       newEntryForm(values),
		loading:    false,
		spinner:    s,
	}
}

func (m EntryFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m EntryFormModel) Update(msg tea.Msg) (EntryFormModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case SubmitSuccessMsg:
		m.reset()
		return m, m.form.Init()
	case SubmitFailedMsg:
		m.loading = false
		m.submitError = msg.Error
		var outcome *service.SaveError
		m.conflict = errors.As(msg.Error, &outcome) && outcome.Kind == service.SaveConflict
		cmd := m.restartForm()
		return m, cmd
	case ReloadSuccessMsg:
		m.conflict = false
		m.submitError = nil
		m.reviewDraft = true
		cmd := m.restartForm()
		return m, cmd
	case ReloadFailedMsg:
		m.submitError = fmt.Errorf("reload failed: %w", msg.Error)
		return m, nil
	case tea.KeyPressMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case control.GeneratePassword:
			return m.handleGeneratePassword()
		}
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	cmds = append(cmds, cmd)
	if m.form.State == huh.StateCompleted && !m.loading && !m.conflict {
		m.loading = true
		m.requestID++
		m.submitError = nil
		m.reviewDraft = false
		return m, tea.Batch(cmd, m.spinner.Tick, m.submitEntry())
	}

	if m.loading {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, tea.Batch(cmds...)
}

func (m EntryFormModel) View() string {
	if m.loading {
		return m.spinner.View() + savingText
	}

	status := m.statusView()
	body := m.form.View()
	if m.height > 0 {
		available := max(0, m.height-lipgloss.Height(status))
		if available == 0 {
			body = ""
		} else {
			body = lipgloss.NewStyle().MaxHeight(available).Render(body)
		}
	}
	if status != "" {
		return lipgloss.JoinVertical(lipgloss.Left, body, status)
	}
	return body
}

func (m EntryFormModel) statusView() string {
	var body string
	if m.conflict {
		body = "ctrl+r: reload latest database. Your draft is kept for review."
	} else if m.reviewDraft {
		body = "Database reloaded. Review your draft against the latest entries before submitting."
	} else if m.submitError != nil {
		body = "Not saved. Review your draft and submit again to retry."
	}
	if m.submitError != nil {
		if body != "" {
			body += "\n"
		}
		body += style.ErrorText.Render(m.submitError.Error())
	}
	if body == "" {
		return ""
	}
	s := lipgloss.NewStyle()
	if m.width > 0 {
		s = s.Width(m.width)
	}
	if m.height > 0 {
		s = s.MaxHeight(m.height)
	}
	return s.Render(body)
}

func (m *EntryFormModel) resizeForm() {
	height := m.height
	if height > 0 {
		height = max(1, height-lipgloss.Height(m.statusView()))
	}
	m.form.WithWidth(m.width).WithHeight(height)
}

func (m *EntryFormModel) SetSize(width, height int) {
	m.width, m.height = width, height
	m.resizeForm()
	form, _ := m.form.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
}

func (m *EntryFormModel) reset() {
	m.requestID++
	m.loading = false
	m.submitError = nil
	m.editingID = nil
	m.conflict = false
	m.reviewDraft = false
	m.formValues = getEmptyFormValues()
	m.form = newEntryForm(m.formValues).WithWidth(m.width).WithHeight(m.height)
}

// restartForm preserves draft values but resets completion/focus. A failed save
// must require a new user submission, never a retry triggered by another message.
func (m *EntryFormModel) restartForm() tea.Cmd {
	m.form = newEntryForm(m.formValues).WithWidth(m.width).WithHeight(m.height)
	m.resizeForm()
	return m.form.Init()
}

func (m *EntryFormModel) StartNew() tea.Cmd {
	m.reset()
	return m.form.Init()
}

func (m *EntryFormModel) Edit(id gokeepasslib.UUID, entry service.NewVaultEntry) tea.Cmd {
	m.reset()
	m.editingID = &id
	m.formValues = &entryFormValues{
		Title:    entry.Title,
		Username: entry.Username,
		URL:      entry.URL,
		Password: entry.Password,
	}
	return m.restartForm()
}

func (m *EntryFormModel) submitEntry() tea.Cmd {
	requestID := m.requestID
	entry := service.NewVaultEntry{
		Title:    m.formValues.Title,
		Username: m.formValues.Username,
		URL:      m.formValues.URL,
		Password: m.formValues.Password,
	}

	if m.editingID != nil {
		id := *m.editingID
		return func() tea.Msg {
			return SubmitEditEntryMsg{RequestID: requestID, ID: id, Entry: entry}
		}
	}
	return func() tea.Msg {
		return SubmitEntryMsg{RequestID: requestID, Entry: entry}
	}
}

func (m EntryFormModel) AcceptsSubmission(requestID uint64) bool {
	return m.loading && !m.conflict && m.requestID == requestID
}

func (m EntryFormModel) IsSaving() bool { return m.loading }
