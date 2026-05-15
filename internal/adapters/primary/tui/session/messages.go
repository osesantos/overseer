package session

type sessionCreatedMsg struct {
	SessionID string
}

type sessionRenamedMsg struct {
	NewName string
}

type cancelFormMsg struct{}
