package login

type LoginSubmitMsg struct {
	RequestID   uint64
	DBPath      string
	KeyFilePath string
	Password    string
}

type AuthenticationFailedMsg struct {
	Error error
}
