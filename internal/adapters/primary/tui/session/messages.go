package session

import domainsession "github.com/dnlopes/overseer/internal/core/domain/session"

type sessionCreatedMsg struct{ session domainsession.Session }

type sessionRenamedMsg struct{ session domainsession.Session }

type cancelFormMsg struct{}
