package login

type LoginSubmitMsg struct {
	DBPath      string
	KeyFilePath string
	Password    string
}

type AuthenticationFailedMsg struct {
	Error error
}
