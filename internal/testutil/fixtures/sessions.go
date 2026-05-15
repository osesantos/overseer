package fixtures

import "github.com/dnlopes/overseer/internal/core/domain/session"

func MakeSession(name, project string) session.Session {
	s, err := session.New(name, project)
	if err != nil {
		panic(err)
	}
	return s
}
