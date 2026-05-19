package shared

import "github.com/dnlopes/overseer/internal/core/domain"

type SessionCreatedMsg struct{ Session domain.Session }

type SessionSelectedMsg struct{ ID string }

type SessionsLoadedMsg struct {
	Sessions []domain.Session
	Err      error
}

type NewSessionPopupCloseMsg struct{}

type SessionCreateErrMsg struct{ Err error }

type ProjectsLoadedMsg struct {
	Projects []domain.Project
	Err      error
}

type ProjectRegisteredMsg struct{ Project domain.Project }

type ProjectRegisterErrMsg struct{ Err error }

type ProjectSelectedMsg struct{ ID string }

type NewProjectPopupCloseMsg struct{}

type LeftPaneTab int

const (
	LeftPaneTabSessions LeftPaneTab = iota
	LeftPaneTabProjects
)

type LeftPaneTabChangedMsg struct{ Tab LeftPaneTab }
