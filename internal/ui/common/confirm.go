// Package common contains common components for the UI.
package common

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/control"
)

type ConfirmPromptModel struct {
	confirmCommand tea.Cmd
	msg            string
}

type OpenConfirmMsg struct {
	Msg string
	Cmd tea.Cmd
}

type CloseConfirmMsg struct {
	Cmd     tea.Cmd
	Success bool
}

func NewConfirmPrompt() ConfirmPromptModel {
	return ConfirmPromptModel{}
}

func (m ConfirmPromptModel) Init() tea.Cmd {
	return nil
}

func (m ConfirmPromptModel) Update(msg tea.Msg) (ConfirmPromptModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case control.Confirm:
			if m.confirmCommand != nil {
				return m, func() tea.Msg {
					return CloseConfirmMsg{
						Cmd:     m.confirmCommand,
						Success: true,
					}
				}
			}
		case control.Cancel:
			return m, func() tea.Msg {
				return CloseConfirmMsg{
					Cmd:     nil,
					Success: false,
				}
			}
		}
	case OpenConfirmMsg:
		m.msg = msg.Msg
		m.confirmCommand = msg.Cmd
	}

	return m, cmd
}

func (m ConfirmPromptModel) View() string {
	return fmt.Sprintf("%s (y/n)", m.msg)
}
