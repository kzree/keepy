package textprovider

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/locale"
	"kzree.com/keepy/internal/textprovider/namespace"
)

func TestResolveNormalizesSupportedLocalesAndFallsBackToEnglish(t *testing.T) {
	for _, preference := range []string{"en", "En", " EN\n", "", "   ", "et", "fr-FR", "saved-locale"} {
		assert.Equal(t, locale.En, locale.Resolve(preference), preference)
	}
}

func TestTextFallsBackPerKeyToEnglishThenRawKey(t *testing.T) {
	locale.Catalogs["xx"] = locale.Catalog{namespace.Login: {key.LabelPassword: "Salasõna"}}
	t.Cleanup(func() { delete(locale.Catalogs, "xx") })
	assert.Equal(t, "xx", locale.Resolve(" XX "))

	text := New(" XX ", namespace.Login)
	assert.Equal(t, "Salasõna", text.Text(key.LabelPassword))
	assert.Equal(t, "Database path", text.Text(key.LabelDBPath), "missing translations use English")
	assert.Equal(t, "Title", New("xx", namespace.List).Text(key.LabelTitle), "missing namespaces use English")
	assert.Equal(t, "UNKNOWN_KEY", text.Text("UNKNOWN_KEY"))
	assert.Equal(t, "Password", New("unsupported", namespace.Login).Text(key.LabelPassword))
}
