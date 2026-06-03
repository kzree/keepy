package ui

import "kzree.com/keepy/internal/style"

func (r *RootModel) renderFrame(content string) string {
	style := style.Frame
	frameWidth, frameHeight := style.GetFrameSize()
	contentWidth := max(0, r.width-frameWidth)
	contentHeight := max(0, r.height-frameHeight)
	return style.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}
