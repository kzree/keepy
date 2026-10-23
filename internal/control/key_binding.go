// Package control holds the key binding definitions for the application.
package control

type KeyBinding string

const (
	QuitApplication    = "ctrl+c"
	QuitApplicationAlt = "q"
	LoginToggle        = "ctrl+t"
	CopyEntry          = "c"
	NewEntry           = "n"
	EditEntry          = "e"
	ClearFilter        = "x"
	Search             = "f"
	DeleteEntity       = "d"
	GeneratePassword   = "ctrl+g"
	CloseEntryForm     = "esc"
	ReloadVault        = "ctrl+r"
	Confirm            = "y"
	Cancel             = "n"
	ShowHelp           = "?"
	ShowHelpAnywhere   = "f1"
)
