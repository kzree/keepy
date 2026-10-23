package common

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"kzree.com/keepy/internal/control"
)

func TestKeymapHelpScrollsWrapsAndKeepsCloseHintsVisible(t *testing.T) {
	m := NewKeymapHelpModel()
	m.SetContext(control.ListKeys, false)
	m.SetSize(70, 12)
	top := m.View()
	assert.Contains(t, top, "Key bindings")
	assert.Contains(t, top, "Global")
	assert.Contains(t, top, "Esc/?/F1")
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyPgDown}))
	assert.NotEqual(t, top, m.View())
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnd}))
	assert.Contains(t, m.View(), "first / last help page")
	m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyHome}))
	assert.Equal(t, top, m.View())
	for _, size := range [][2]int{{25, 8}, {8, 2}, {4, 1}, {0, 0}, {70, 12}} {
		m.SetSize(size[0], size[1])
		view := m.View()
		if size[0] == 0 || size[1] == 0 {
			assert.Empty(t, view)
			continue
		}
		assert.LessOrEqual(t, lipgloss.Width(view), size[0])
		assert.LessOrEqual(t, lipgloss.Height(view), size[1])
	}
}
