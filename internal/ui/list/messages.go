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
	RequestID uint64
	ID        gokeepasslib.UUID
}

type EditEntryRequestMsg struct {
	RequestID uint64
	Entry     service.VaultEntry
}

type OpenEditEntryMsg struct {
	RequestID uint64
	Error     error
	Entry     service.VaultEntry
	Password  string
}

type SaveResultMsg struct {
	Error error
}

type ConfigSaveWarningMsg struct{ Error error }

type ReloadVaultRequestMsg struct{ RequestID uint64 }

type ReloadVaultResultMsg struct {
	Error error
}
