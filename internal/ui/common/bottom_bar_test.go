package common

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestBottomBarWrapsAndBoundsGlobalNotices(t *testing.T) {
	var bar BottomBarModel
	assert.Empty(t, bar.View(60, 12))
	bar.SetApplicationNotice(strings.Repeat("settingsfailure ", 12))
	bar.SetStatus("Save committed. Do not repeat the operation.")
	view := bar.View(60, 12)
	assert.Contains(t, view, "Save committed")
	assert.Contains(t, view, "settingsfailure")
	assert.LessOrEqual(t, lipgloss.Width(view), 60)
	assert.LessOrEqual(t, lipgloss.Height(view), 12)
	// Recovery status takes priority over unrelated application errors.
	assert.Contains(t, bar.View(60, 2), "Do not repeat")
	for _, size := range [][2]int{{0, 10}, {40, 0}, {1, 1}, {12, 2}} {
		view := bar.View(size[0], size[1])
		if size[0] == 0 || size[1] == 0 {
			assert.Empty(t, view)
		} else {
			assert.LessOrEqual(t, lipgloss.Width(view), size[0])
			assert.LessOrEqual(t, lipgloss.Height(view), size[1])
		}
	}
	bar.SetStatus("")
	bar.SetApplicationNotice("")
	assert.Empty(t, bar.View(60, 12))
}
