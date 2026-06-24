package login

import tea "charm.land/bubbletea/v2"

func (m LoginModel) handleAuthenticationFailedMsg(msg AuthenticationFailedMsg) (LoginModel, tea.Cmd) {
	m.loading = false
	m.form = newLoginForm(m.formValues)
	m.authError = msg.Error
	return m, m.form.Init()
}
