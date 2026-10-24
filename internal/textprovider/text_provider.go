// Package textprovider provides a simple interface for text fetching for correct locale
package textprovider

import (
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

type TextProvider struct {
	values   map[string]string
	fallback map[string]string
}

// New resolves l, so empty or unsupported locales use English.
func New(l locale.Locale, ns namespace.Namespace) *TextProvider {
	return &TextProvider{
		values:   locale.Catalogs[locale.Resolve(l)][ns],
		fallback: locale.Catalogs[locale.En][ns],
	}
}

// Text falls back per key to English, then to the raw key.
func (t TextProvider) Text(key string) string {
	if value := t.values[key]; value != "" {
		return value
	}
	if value := t.fallback[key]; value != "" {
		return value
	}
	return key
}
