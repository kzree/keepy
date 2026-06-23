// Package entryform provides a form for creating new entries in the list UI.
package entryform

import (
	"charm.land/huh/v2"
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
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Key(titleKey).
				Value(&values.Title).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("Username/Email").
				Key(usernameKey).
				Value(&values.Username).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("URL").
				Key(urlKey).
				Value(&values.URL).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				Value(&values.Password).
				Prompt(": ").
				Inline(true),
		),
	).WithTheme(util.GetFormTheme())
}
