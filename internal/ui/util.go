package ui

import "kzree.com/keepy/internal/style"

func renderFrame(content string, width, height int) string {
	style := style.Frame
	frameWidth, frameHeight := style.GetFrameSize()
	contentWidth := max(0, width-frameWidth)
	contentHeight := max(0, height-frameHeight)
	return style.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}
