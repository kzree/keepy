package entryform

import "kzree.com/keepy/internal/service"

type SubmitEntryMsg struct {
	entry service.NewVaultEntry
}

type SubmitSuccessMsg struct{}

type SubmitFailedMsg struct {
	error error
}

type CloseEntryForm struct{}
