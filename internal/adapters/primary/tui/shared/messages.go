package shared

import "github.com/dnlopes/overseer/internal/core/domain"

type SummaryPanelSelected struct{}

type SessionsPanelSelected struct{}

type SessionCreatedMsg struct{ Session domain.Session }

type SessionRenamedMsg struct{ Session domain.Session }

type SessionSelectedMsg struct{ ID string }

type SessionsLoadedMsg struct {
	Sessions []domain.Session
	Err      error
}

type NewSessionPopupCloseMsg struct{}
