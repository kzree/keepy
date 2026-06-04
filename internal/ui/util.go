package ui

import "kzree.com/keepy/internal/style"

func (r *RootModel) renderFrame(content string) string {
	return style.Frame.
		Width(max(0, r.width)).
		Height(max(0, r.height)).
		Render(content)
}
