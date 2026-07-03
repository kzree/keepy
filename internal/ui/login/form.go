package login

import (
	"charm.land/huh/v2"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/util"
)

type loginFormValues struct {
	password    string
	dbPath      string
	keyFilePath string
}

func newShortLoginForm(v *loginFormValues) *huh.Form {
	t := textprovider.New(locale.En, namespace.Login)

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(t.Text(key.LabelPassword)).
				Key(passwordKey).
				Prompt(": ").
				Inline(true).
				Value(&v.password).
				EchoMode(huh.EchoModePassword),
		),
	).WithTheme(util.GetFormTheme())
}

func newLoginForm(v *loginFormValues) *huh.Form {
	t := textprovider.New(locale.En, namespace.Login)

	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(t.Text(key.LabelPassword)).
				Key(passwordKey).
				Prompt(": ").
				Inline(true).
				Value(&v.password).
				EchoMode(huh.EchoModePassword),
			huh.NewInput().
				Title(t.Text(key.LabelDBPath)).
				Key(dbPathKey).
				Prompt(": ").
				Inline(true).
				Value(&v.dbPath),
			huh.NewInput().
				Title(t.Text(key.LabelKeyFilePath)).
				Key(keyFilePathKey).
				Prompt(": ").
				Inline(true).
				Value(&v.keyFilePath),
		),
	).WithTheme(util.GetFormTheme())
}
