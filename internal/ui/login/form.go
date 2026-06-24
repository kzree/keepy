package login

import (
	"charm.land/huh/v2"
	"kzree.com/keepy/internal/util"
)

func newLoginForm(dbPath, keyFilePath string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				Prompt(": ").
				Inline(true).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("Database path").
				Key(dbPathKey).
				Prompt(": ").
				Inline(true).
				Value(&dbPath),
			huh.NewInput().
				Title("Key file path").
				Key(keyFilePathKey).
				Prompt(": ").
				Inline(true).
				Value(&keyFilePath),
		),
	).WithTheme(util.GetFormTheme())
}
