// Package search contains the screen for searching vault entries.
package search

import (
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
}

type ClearSearchMsg struct{}

func NewSearchModel(t *textprovider.TextProvider) SearchModel {
	input := textinput.New()
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
	return fmt.Sprintf(
		m.t.Text(key.LabelSearch),
		m.input.View(),
	)
}

func (m SearchModel) Value() string {
	return m.value
}
