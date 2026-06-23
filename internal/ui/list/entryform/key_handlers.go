package entryform

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/service"
)

func (m EntryFormModel) handleGeneratePassword() (EntryFormModel, tea.Cmd) {
	pwd := service.GenerateRandomPassword(24)

	m.formValues.Password = pwd
	m.form = newEntryForm(m.formValues)

	return m, m.form.Init()
}
