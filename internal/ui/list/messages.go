package list

import (
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
)

type copyFlashDoneMsg struct {
	id int
}

type CopyPasswordRequestMsg struct {
	ID gokeepasslib.UUID
}

type CopyPasswordSuccessMsg struct{}

type CopyPasswordFailureMsg struct {
	Error error
}

type DeleteEntryRequestMsg struct {
	ID gokeepasslib.UUID
}

type EditEntryRequestMsg struct {
	Entry service.VaultEntry
}

type OpenEditEntryMsg struct {
	Entry    service.VaultEntry
	Password string
}
