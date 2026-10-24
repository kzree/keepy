package login

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"kzree.com/keepy/internal/control"
	"kzree.com/keepy/internal/service"
	"kzree.com/keepy/internal/textprovider/locale"
)

func TestAuthenticationFailuresExplainTheCause(t *testing.T) {
	missing := &fs.PathError{Op: "lstat", Path: "/vaults/x", Err: fs.ErrNotExist}
	denied := &fs.PathError{Op: "open", Path: "/vaults/x", Err: fs.ErrPermission}
	for _, scenario := range []struct {
		err      error
		want     string
		unwanted string
	}{
		{fmt.Errorf("%w (%w)", service.ErrInvalidCredentials, errors.New("Wrong password? Database integrity check failed")), "Wrong password or key file, or the database is damaged.", "integrity"},
		{service.ErrNotDatabase, "not a KeePass (KDBX) database", "EOF"},
		{&service.AuthFileError{File: service.AuthDatabase, Err: missing}, "Database file not found.", "lstat"},
		{&service.AuthFileError{File: service.AuthKeyFile, Err: missing}, "Key file not found.", "lstat"},
		{&service.AuthFileError{File: service.AuthDatabase, Err: denied}, "No permission to read the database file.", "open /vaults"},
		{&service.AuthFileError{File: service.AuthKeyFile, Err: denied}, "No permission to read the key file.", "open /vaults"},
		{&service.AuthFileError{File: service.AuthKeyFile, Err: errors.New("invalid key file XML")}, "Key file could not be read: invalid key file XML", ""},
		{&service.AuthFileError{File: service.AuthDatabase, Err: errors.New("database path must refer to a regular file")}, "Database could not be opened: database path must refer to a regular file", ""},
	} {
		m := NewLoginModel(&service.Config{Credentials: service.Credentials{DBPath: "/vaults/x"}}, locale.En, control.DefaultKeymap())
		m.SetSize(200, 30)
		m, _ = m.Update(AuthenticationFailedMsg{Error: scenario.err})
		view := m.View()
		assert.Contains(t, view, scenario.want)
		if scenario.unwanted != "" {
			assert.NotContains(t, view, scenario.unwanted)
		}
	}
}
