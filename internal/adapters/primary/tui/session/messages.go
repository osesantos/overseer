package session

import domainsession "github.com/dnlopes/overseer/internal/core/domain/session"

type SessionCreatedMsg struct{ Session domainsession.Session }

type SessionRenamedMsg struct{ Session domainsession.Session }

type CancelFormMsg struct{}
