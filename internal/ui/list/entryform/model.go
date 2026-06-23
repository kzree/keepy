package entryform

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/style"
)

const savingText = "Saving entry..."

type EntryFormModel struct {
	form        *huh.Form
	loading     bool
	spinner     spinner.Model
	submitError error
}

func NewEntryFormModel() EntryFormModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.BrightRed)

	return EntryFormModel{
		form:    newEntryForm(),
		loading: false,
		spinner: s,
	}
}

func (m EntryFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m EntryFormModel) Update(msg tea.Msg) (EntryFormModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case SubmitSuccessMsg:
		m.loading = false
		m.form = newEntryForm()
		return m, func() tea.Msg {
			return CloseEntryForm{}
		}
	case SubmitFailedMsg:
		m.loading = false
		m.submitError = msg.Error
	}

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
	cmds = append(cmds, cmd)
	if m.form.State == huh.StateCompleted && !m.loading {
		m.loading = true
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

	body := m.form.View()

	if m.submitError != nil {
		body += "\n" + style.ErrorText.Render(m.submitError.Error())
	}

	return body
}

func (m *EntryFormModel) SetSize(width, height int) {
	m.form.WithWidth(width)
	m.form.WithHeight(height)
	form, _ := m.form.Update(tea.WindowSizeMsg{
		Width:  width,
		Height: height,
	})
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}
}

func (m *EntryFormModel) submitEntry() tea.Cmd {
	entry := service.NewVaultEntry{
		Title:    m.form.GetString(titleKey),
		Username: m.form.GetString(usernameKey),
		URL:      m.form.GetString(urlKey),
		Password: m.form.GetString(passwordKey),
	}

	return func() tea.Msg {
		return SubmitEntryMsg{
			entry,
		}
	}
}
