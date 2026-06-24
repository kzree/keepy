package login

import (
	"charm.land/huh/v2"
	"kzree.com/keepy/internal/util"
)

type loginFormValues struct {
	password    string
	dbPath      string
	keyFilePath string
}

func newShortLoginForm(v *loginFormValues) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				Prompt(": ").
				Inline(true).
				Value(&v.password).
				EchoMode(huh.EchoModePassword),
		),
	).WithTheme(util.GetFormTheme())
}

func newLoginForm(v *loginFormValues) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				Prompt(": ").
				Inline(true).
				Value(&v.password).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title("Database path").
				Key(dbPathKey).
				Prompt(": ").
				Inline(true).
				Value(&v.dbPath),
			huh.NewInput().
				Title("Key file path").
				Key(keyFilePathKey).
				Prompt(": ").
				Inline(true).
				Value(&v.keyFilePath),
		),
	).WithTheme(util.GetFormTheme())
}
