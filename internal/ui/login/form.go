package login

import (
	"errors"

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
	useKeyFile  bool
}

func newShortLoginForm(v *loginFormValues) *huh.Form {
	t := textprovider.New(locale.En, namespace.Login)

	return huh.NewForm(
		huh.NewGroup(
			passwordInput(v, t),
		),
	).WithTheme(util.GetFormTheme())
}

func newLoginForm(v *loginFormValues) *huh.Form {
	t := textprovider.New(locale.En, namespace.Login)

	fields := []huh.Field{passwordInput(v, t), huh.NewInput().
		Title(t.Text(key.LabelDBPath)).
		Key(dbPathKey).
		Prompt(": ").
		Inline(true).
		Value(&v.dbPath).Validate(requiredPath),
	}
	if v.useKeyFile {
		fields = append(fields, huh.NewInput().
			Title(t.Text(key.LabelKeyFilePath)).
			Key(keyFilePathKey).
			Prompt(": ").
			Inline(true).
			Value(&v.keyFilePath).Validate(requiredPath))
	}
	return huh.NewForm(huh.NewGroup(fields...)).WithTheme(util.GetFormTheme())
}

func passwordInput(v *loginFormValues, t *textprovider.TextProvider) *huh.Input {
	return huh.NewInput().Title(t.Text(key.LabelPassword)).Key(passwordKey).
		Prompt(": ").Inline(true).Value(&v.password).EchoMode(huh.EchoModePassword)
}

func requiredPath(path string) error {
	if path == "" {
		return errors.New("select a file or enter its path")
	}
	return nil
}
