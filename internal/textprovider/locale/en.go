// Package locale holds locale maps
package locale

import (
	"kzree.com/keepy/internal/textprovider/key"
	"kzree.com/keepy/internal/textprovider/namespace"
)

var EnMap = map[namespace.Namespace]map[string]string{
	namespace.Login: enLogin,
	namespace.List:  enList,
}

var enLogin = map[string]string{
	key.LabelPassword:    "Password",
	key.LabelDBPath:      "Database path",
	key.LabelKeyFilePath: "Key file path",
}

var enList = map[string]string{
	key.LabelTitle:    "Title",
	key.LabelUsername: "Username/Email",
	key.LabelURL:      "URL",
	key.LabelPassword: "Password",
	key.LabelSearch:   " Search: %s\n",
	key.ColumnTitle:   "Title",
	key.ColumnEmail:   "Email",
	key.ColumnURL:     "URL",
	key.ColumnGroup:   "Group",
}
