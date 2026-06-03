// Package style contains lipgloss style
package style

import (
	"charm.land/lipgloss/v2"
)

var Frame = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.White).
	Foreground(lipgloss.Color("15")).
	Padding(1, 2)
