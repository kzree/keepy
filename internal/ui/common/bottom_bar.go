package common

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// BottomBarModel renders global notices independently of the active screen.
// Status/recovery guidance takes priority over application notices when space is
// limited. An empty bar consumes no rows; the root reserves its rendered height.
type BottomBarModel struct {
	applicationNotice string
	statusNotice      string
}

func (m *BottomBarModel) SetApplicationNotice(text string) { m.applicationNotice = text }
func (m *BottomBarModel) SetStatus(text string)            { m.statusNotice = text }

func (m BottomBarModel) View(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	var notices []string
	if m.statusNotice != "" {
		notices = append(notices, m.statusNotice)
	}
	if m.applicationNotice != "" {
		notices = append(notices, m.applicationNotice)
	}
	if len(notices) == 0 {
		return ""
	}
	s := lipgloss.NewStyle().Foreground(lipgloss.Yellow).Width(width).MaxWidth(width).MaxHeight(height)
	if height > 1 {
		s = s.Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(lipgloss.BrightBlack)
	}
	return s.Render(strings.Join(notices, "\n"))
}
