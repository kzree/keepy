package common

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

// KeymapHelpModel is a read-only presentation: the root retains and continues
// updating the underlying screen and any accepted background operation.
type KeymapHelpModel struct {
	viewport      viewport.Model
	width, height int
	context       string
	busy          bool
	groups        []control.KeymapGroup
	t             *textprovider.TextProvider
}

func NewKeymapHelpModel(l locale.Locale) KeymapHelpModel {
	v := viewport.New()
	v.KeyMap = control.HelpViewportKeyMap()
	return KeymapHelpModel{viewport: v, groups: control.KeymapGroups(l), t: textprovider.New(l, namespace.Help)}
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
	for _, group := range m.groups {
		if group.ID == m.context {
			body.WriteString(fmt.Sprintf(m.t.Text(key.HelpCurrentContext), group.Title) + "\n")
		}
	}
	if m.busy {
		body.WriteString(m.t.Text(key.HelpBusy) + "\n")
	}
	for _, group := range m.groups {
		title := group.Title
		if group.ID == m.context {
			title = fmt.Sprintf(m.t.Text(key.HelpCurrentGroup), title)
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
	footer := ansi.Truncate(strings.Join([]string{
		control.Hint(m.t.Text(key.HintClose), "esc", control.ShowHelp, control.ShowHelpAnywhere),
		control.Hint(m.t.Text(key.HintScroll), "up", "down"),
		control.Hint(m.t.Text(key.HintPage), "pgup", "pgdown"),
		control.Hint(m.t.Text(key.HintFirstLast), "home", "end"),
	}, " · "), m.width, "…")
	if m.height == 1 {
		return footer
	}
	lines := []string{ansi.Truncate(m.t.Text(key.HelpTitle), m.width, "…")}
	if m.height > 2 {
		lines = append(lines, m.viewport.View())
	}
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}
