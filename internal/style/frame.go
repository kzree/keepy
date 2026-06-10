// Package style contains lipgloss style
package style

import (
	"charm.land/lipgloss/v2"
)

var Frame = lipgloss.NewStyle().
	Border(lipgloss.HiddenBorder()).
	BorderForeground(lipgloss.Black).
	Foreground(lipgloss.Black)
