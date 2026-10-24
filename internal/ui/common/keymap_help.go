package common

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"kzree.com/keepy/internal/control"
)

// KeymapHelpModel is a read-only presentation: the root retains and continues
// updating the underlying screen and any accepted background operation.
type KeymapHelpModel struct {
	viewport      viewport.Model
	width, height int
	context       string
	busy          bool
}

func NewKeymapHelpModel() KeymapHelpModel {
	v := viewport.New()
	v.KeyMap = control.HelpViewportKeyMap()
	return KeymapHelpModel{viewport: v}
}

func (m *KeymapHelpModel) SetSize(width, height int) {
	if m.width == width && m.height == height {
		return
	}
	m.width, m.height = width, height
	m.viewport.SetWidth(max(1, width))
	m.viewport.SetHeight(max(1, height-2))
	m.updateContent()
}

func (m *KeymapHelpModel) SetContext(context string, busy bool) {
	if m.context == context && m.busy == busy {
		return
	}
	m.context, m.busy = context, busy
	m.updateContent()
}

func (m *KeymapHelpModel) updateContent() {
	var body strings.Builder
	body.WriteString("Current context: " + m.context + "\n")
	if m.busy {
		body.WriteString("Vault operation in progress: actions and form input wait; help and resizing remain available.\n")
	}
	for _, group := range control.KeymapGroups() {
		title := group.Title
		if group.Title == m.context {
			title += " (current)"
		}
		body.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.BrightBlue).Bold(true).Render(title) + "\n")
		for _, binding := range group.Bindings {
			body.WriteString("  " + strings.Join(binding.Keys(), " / ") + " — " + binding.Help().Desc + "\n")
		}
	}
	m.viewport.SetContent(ansi.Wrap(body.String(), max(1, m.width), " "))
}

func (m KeymapHelpModel) Update(msg tea.Msg) (KeymapHelpModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "home":
			m.viewport.GotoTop()
			return m, nil
		case "end":
			m.viewport.GotoBottom()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m KeymapHelpModel) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	footer := ansi.Truncate("Esc/?/F1 close · ↑/↓ scroll · PgUp/PgDn · Home/End", m.width, "…")
	if m.height == 1 {
		return footer
	}
	lines := []string{ansi.Truncate("Key bindings", m.width, "…")}
	if m.height > 2 {
		lines = append(lines, m.viewport.View())
	}
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}
