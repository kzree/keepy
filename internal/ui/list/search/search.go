// Package search contains the screen for searching vault entries.
package search

import (
	"charm.land/lipgloss/v2"
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
)

type SearchModel struct {
	input textinput.Model
	value string
	t     *textprovider.TextProvider
	width int
}

type ClearSearchMsg struct{}

func NewSearchModel(t *textprovider.TextProvider) SearchModel {
	input := textinput.New()
	input.Prompt = ""
	input.Focus()

	return SearchModel{
		input: input,
		t:     t,
	}
}

func (m SearchModel) Init() tea.Cmd {
	return nil
}

func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.(type) {
	case ClearSearchMsg:
		m.input.SetValue("")
		m.value = ""
	}
	m.input, cmd = m.input.Update(msg)
	m.value = m.input.Value()

	return m, cmd
}

func (m SearchModel) View() string {
	if m.width > 0 && m.width < 12 {
		return m.input.View()
	}
	return fmt.Sprintf(
		m.t.Text(key.LabelSearch),
		m.input.View(),
	)
}

func (m *SearchModel) SetWidth(width int) {
	m.width = width
	prefix := lipgloss.Width(fmt.Sprintf(m.t.Text(key.LabelSearch), ""))
	if width < 12 {
		prefix = 0
	}
	m.input.SetWidth(max(1, width-prefix))
}

func (m SearchModel) Value() string {
	return m.value
}
