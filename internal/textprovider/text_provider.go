// Package textprovider provides a simple interface for text fetching for correct locale
package textprovider

import (
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
	"kzree.com/keepy/internal/util"
)

type TextProvider struct {
	values map[string]string
}

func New(l locale.Locale, ns namespace.Namespace) *TextProvider {
	return &TextProvider{
		values: getNamespacedValues(l, ns),
	}
}

func (t TextProvider) Text(key string) string {
	value := t.values[key]
	return util.Ternary(value == "", key, value)
}

func getNamespacedValues(l locale.Locale, ns namespace.Namespace) map[string]string {
	switch l {
	case "en":
		return locale.EnMap[ns]
	}

	return make(map[string]string)
}
