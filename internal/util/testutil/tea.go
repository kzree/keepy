package testutil

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// DrainTeaCommands drives finite lifecycle commands without a terminal. Batch and
// Sequence messages both wrap command slices; only periodic animation messages
// are ignored, so focus, navigation, completion, and application messages run.
func DrainTeaCommands(t *testing.T, cmd tea.Cmd, update func(tea.Msg) tea.Cmd) {
	t.Helper()
	commands := []tea.Cmd{cmd}
	for count := 0; len(commands) > 0; count++ {
		if count > 200 {
			t.Fatal("command lifecycle did not settle")
		}
		cmd, rest := commands[0], commands[1:]
		commands = rest
		if cmd == nil {
			continue
		}
		// Skip animation commands before they wait on timers. They do not carry
		// form/application state; focus is set synchronously by the field model.
		name := runtime.FuncForPC(reflect.ValueOf(cmd).Pointer()).Name()
		if strings.HasPrefix(name, "charm.land/bubbles/v2/cursor.") || strings.HasPrefix(name, "charm.land/bubbles/v2/spinner.") {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		// Bubble Tea's Sequence message is unexported, but has the same underlying
		// slice type as BatchMsg. No private fields or state are accessed here.
		value := reflect.ValueOf(msg)
		commandSlice := reflect.TypeOf([]tea.Cmd{})
		if value.Type().ConvertibleTo(commandSlice) {
			commands = append(value.Convert(commandSlice).Interface().([]tea.Cmd), commands...)
			continue
		}
		switch msg.(type) {
		case cursor.BlinkMsg, spinner.TickMsg:
			continue
		}
		commands = append([]tea.Cmd{update(msg)}, commands...)
	}
}
