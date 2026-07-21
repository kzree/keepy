package entryform

import (
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
)

type SubmitEntryMsg struct {
	Entry service.NewVaultEntry
}

type SubmitEditEntryMsg struct {
	ID    gokeepasslib.UUID
	Entry service.NewVaultEntry
}

type SubmitSuccessMsg struct{}

type SubmitFailedMsg struct {
	Error error
}

type CloseEntryForm struct{}
