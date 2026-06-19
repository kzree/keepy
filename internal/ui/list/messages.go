package list

import "github.com/tobischo/gokeepasslib/v3"

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
