package entryform

import "kzree.com/keepy/internal/service"

type SubmitEntryMsg struct {
	Entry service.NewVaultEntry
}

type SubmitSuccessMsg struct{}

type SubmitFailedMsg struct {
	Error error
}

type CloseEntryForm struct{}
