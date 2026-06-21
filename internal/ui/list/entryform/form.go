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

func newEntryForm() *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Key(titleKey).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("Username/Email").
				Key(usernameKey).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("URL").
				Key(urlKey).
				Prompt(": ").
				Inline(true),
			huh.NewInput().
				Title("Password").
				Key(passwordKey).
				Prompt(": ").
				Inline(true),
		),
	).WithTheme(util.GetFormTheme())
}
