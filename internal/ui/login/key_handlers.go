package login

import (
	tea "charm.land/bubbletea/v2"
)

func (m LoginModel) handleTogglePathFields() (LoginModel, tea.Cmd) {
	m.showPathFields = !m.showPathFields
	if m.formValues.dbPath == "" || (m.formValues.useKeyFile && m.formValues.keyFilePath == "") {
		m.showPathFields = true
	}
	cmd := m.rebuildForm()
	return m, cmd
}
