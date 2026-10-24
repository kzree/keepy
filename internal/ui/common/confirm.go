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
	requestID      uint64
}

type OpenConfirmMsg struct {
	RequestID uint64
	Msg       string
	Cmd       tea.Cmd
}

type CloseConfirmMsg struct {
	RequestID uint64
	Cmd       tea.Cmd
	Success   bool
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
						RequestID: m.requestID,
						Cmd:       m.confirmCommand,
						Success:   true,
					}
				}
			}
		case control.Cancel:
			return m, func() tea.Msg {
				return CloseConfirmMsg{
					RequestID: m.requestID,
					Cmd:       nil,
					Success:   false,
				}
			}
		}
	case OpenConfirmMsg:
		m.requestID = msg.RequestID
		m.msg = msg.Msg
		m.confirmCommand = msg.Cmd
	}

	return m, cmd
}

func (m ConfirmPromptModel) View() string {
	return fmt.Sprintf("%s (%s/%s)", m.msg, control.Confirm, control.Cancel)
}
