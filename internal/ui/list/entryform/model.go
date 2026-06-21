package entryform

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
)

type EntryFormModel struct {
	form *huh.Form
}

func NewEntryFormModel() EntryFormModel {
	return EntryFormModel{
		form: newEntryForm(),
	}
}

func (m EntryFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m EntryFormModel) Update(msg tea.Msg) (EntryFormModel, tea.Cmd) {
	var cmd tea.Cmd

	form, cmd := m.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		m.form = f
	}

	return m, cmd
}

func (m EntryFormModel) View() string {
	body := m.form.View()
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
