package style

import "charm.land/lipgloss/v2"

var basePane = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	Foreground(lipgloss.White)

var ActivePane = lipgloss.NewStyle().
	Inherit(basePane).
	BorderForeground(lipgloss.BrightGreen)

var InactivePane = lipgloss.NewStyle().
	Inherit(basePane).
	BorderForeground(lipgloss.White)

func GetPaneStyle(isActive bool) lipgloss.Style {
	if isActive {
		return ActivePane
	}

	return InactivePane
}
