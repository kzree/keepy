package login

import (
	"errors"

	"charm.land/huh/v2"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/util"
)

type loginFormValues struct {
	password    string
	dbPath      string
	keyFilePath string
	useKeyFile  bool
}

func newShortLoginForm(v *loginFormValues, l locale.Locale, t *textprovider.TextProvider) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			passwordInput(v, t),
		),
	).WithTheme(util.GetFormTheme()).WithKeyMap(control.FormKeyMap(l))
}

func newLoginForm(v *loginFormValues, l locale.Locale, t *textprovider.TextProvider) *huh.Form {
	requiredPath := requiredPath(t)
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
	return huh.NewForm(huh.NewGroup(fields...)).WithTheme(util.GetFormTheme()).WithKeyMap(control.FormKeyMap(l))
}

func passwordInput(v *loginFormValues, t *textprovider.TextProvider) *huh.Input {
	return huh.NewInput().Title(t.Text(key.LabelPassword)).Key(passwordKey).
		Prompt(": ").Inline(true).Value(&v.password).EchoMode(huh.EchoModePassword)
}

func requiredPath(t *textprovider.TextProvider) func(string) error {
	return func(path string) error {
		if path == "" {
			return errors.New(t.Text(key.ErrorPathRequired))
		}
		return nil
	}
}
