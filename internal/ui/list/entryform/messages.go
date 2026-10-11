package entryform

import (
	"github.com/tobischo/gokeepasslib/v3"
	"kzree.com/keepy/internal/service"
)

type SubmitEntryMsg struct {
	RequestID uint64
	Entry     service.NewVaultEntry
}

type SubmitEditEntryMsg struct {
	RequestID uint64
	ID        gokeepasslib.UUID
	Entry     service.NewVaultEntry
}

type SubmitSuccessMsg struct {
	Warning error
}

type SubmitFailedMsg struct {
	Error error
}

type ReloadSuccessMsg struct{}

type ReloadFailedMsg struct {
	Error error
}
