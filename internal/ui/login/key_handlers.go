package login

import (
	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/util"
)

func (m LoginModel) handleTogglePathFields() (LoginModel, tea.Cmd) {
	m.showPathFields = !m.showPathFields
	m.form = util.Ternary(m.showPathFields, newLoginForm(m.formValues), newShortLoginForm(m.formValues))

	return m, m.form.Init()
}
