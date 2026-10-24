package common

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/textprovider/locale"
)

func TestKeymapHelpScrollsWrapsAndKeepsCloseHintsVisible(t *testing.T) {
	m := NewKeymapHelpModel(locale.En, control.DefaultKeymap(), false)
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

func customizedHelp(t *testing.T, width int) KeymapHelpModel {
	t.Helper()
	keys, err := control.ResolveKeymap(map[string][]string{"copy_entry": {"c", "ctrl+y"}})
	require.NoError(t, err)
	m := NewKeymapHelpModel(locale.En, keys, false)
	m.SetContext(control.ListKeys, false)
	m.SetSize(width, 200)
	return m
}

func TestKeymapHelpMarksOnlyCustomizedBindings(t *testing.T) {
	view := ansi.Strip(customizedHelp(t, 120).View())
	assert.Contains(t, view, "✱ customized in config")
	assert.Contains(t, view, "✱ c / ctrl+y — copy selected password (clipboard must be ready) default: c")
	assert.Regexp(t, `(?m)^  n — create entry\s*$`, view, "unchanged bindings are unmarked")
	assert.Equal(t, 2, strings.Count(view, "✱"), "legend plus the one customized binding")

	narrow := ansi.Strip(customizedHelp(t, 60).View())
	assert.Contains(t, narrow, "✱ c / ctrl+y — copy")
	assert.NotContains(t, narrow, "default: c", "the default is dropped before the binding wraps")
}

func TestKeymapHelpHighlightsOnlyKeysThatDifferFromDefaults(t *testing.T) {
	m := customizedHelp(t, 120)
	var copyEntry control.HelpEntry
	for _, group := range m.groups {
		if group.ID == control.ListKeys {
			copyEntry = group.Bindings[0]
		}
	}
	line := m.entryLine(copyEntry)
	assert.Contains(t, line, customStyle.Render("ctrl+y"))
	assert.Contains(t, line, "c / ", "the retained default key is not highlighted")
	assert.NotContains(t, line, customStyle.Render("c")+" / ")
}

func TestKeymapHelpWithoutOverridesOrWithRejectedOverridesHasNoMarkers(t *testing.T) {
	for _, ignored := range []bool{false, true} {
		m := NewKeymapHelpModel(locale.En, control.DefaultKeymap(), ignored)
		m.SetContext(control.ListKeys, false)
		m.SetSize(120, 200)
		view := ansi.Strip(m.View())
		assert.NotContains(t, view, "✱")
		if ignored {
			assert.Contains(t, view, "Key binding overrides ignored; showing defaults.")
		} else {
			assert.NotContains(t, view, "overrides ignored")
		}
	}
}
