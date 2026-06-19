package util

import (
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

func GetFormTheme() huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		t := huh.ThemeBase(isDark)
		fg := lipgloss.White
		dim := lipgloss.BrightBlack
		accent := lipgloss.BrightBlue
		err := lipgloss.BrightRed

		t.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		t.Focused.Base = lipgloss.NewStyle()
		t.Blurred.Base = lipgloss.NewStyle()
		t.Focused.Title = lipgloss.NewStyle().Foreground(accent).Bold(true)
		t.Blurred.Title = lipgloss.NewStyle().Foreground(dim)
		t.Focused.TextInput.Prompt = lipgloss.NewStyle().Foreground(accent)
		t.Focused.TextInput.Text = lipgloss.NewStyle().Foreground(fg)
		t.Focused.TextInput.Cursor = lipgloss.NewStyle().Foreground(lipgloss.White)
		t.Focused.TextInput.Placeholder = lipgloss.NewStyle().Foreground(dim)
		t.Blurred.TextInput.Prompt = lipgloss.NewStyle().Foreground(dim)
		t.Blurred.TextInput.Text = lipgloss.NewStyle().Foreground(fg)
		t.Blurred.TextInput.Placeholder = lipgloss.NewStyle().Foreground(dim)
		t.Focused.ErrorIndicator = lipgloss.NewStyle().Foreground(err).SetString(" *")
		t.Focused.ErrorMessage = lipgloss.NewStyle().Foreground(err).SetString(" *")
		t.Blurred.ErrorIndicator = t.Focused.ErrorIndicator
		t.Blurred.ErrorMessage = t.Focused.ErrorMessage
		return t
	})
}
