// Package keepy holds the main logic of the application
package keepy

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"kzree.com/keepy/internal/ui"
)

type Keepy struct{}

func New() *Keepy {
	return &Keepy{}
}

func (k *Keepy) Run() {
	p := tea.NewProgram(ui.NewRootModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
