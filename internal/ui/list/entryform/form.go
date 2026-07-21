// Package entryform provides a form for creating and editing entries in the list UI.
package entryform

import (
	"charm.land/huh/v2"
	"kzree.com/keepy/internal/textprovider"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/util"
)

const (
	titleKey    = "title"
	usernameKey = "username"
	passwordKey = "password"
	urlKey      = "url"
)

type entryFormValues struct {
	Title    string
	Username string
	URL      string
	Password string
}

func getEmptyFormValues() *entryFormValues {
	return &entryFormValues{
		Title:    "",
		Username: "",
		URL:      "",
		Password: "",
	}
}

func newEntryForm(values *entryFormValues) *huh.Form {
	t := textprovider.New(locale.En, namespace.List)
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(t.Text(key.LabelTitle)).
				Key(titleKey).
				Value(&values.Title).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title(t.Text(key.LabelUsername)).
				Key(usernameKey).
				Value(&values.Username).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title(t.Text(key.LabelURL)).
				Key(urlKey).
				Value(&values.URL).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title(t.Text(key.LabelPassword)).
				Key(passwordKey).
				Value(&values.Password).
				Prompt(": ").
				Inline(true),
		),
	).WithTheme(util.GetFormTheme())
}
