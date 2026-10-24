package locale

import "strings"

// Supported locales

type Locale = string

const (
	En Locale = "en"
)

// Catalogs holds every supported locale. A locale may omit keys; the text
// provider falls back to English for each missing value.
var Catalogs = map[Locale]Catalog{
	En: EnMap,
}

// Resolve returns the effective locale for a saved preference. Values are
// matched case-insensitively (including legacy "En"); empty or unsupported
// preferences fall back to English without changing the saved value.
func Resolve(preference string) Locale {
	l := strings.ToLower(strings.TrimSpace(preference))
	if _, ok := Catalogs[l]; ok {
		return l
	}
	return En
}
