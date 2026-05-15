package session

import "errors"

var (
	ErrEmptyName      = errors.New("session name cannot be empty")
	ErrNameTooLong    = errors.New("session name exceeds 100 characters")
	ErrEmptyProject   = errors.New("project name cannot be empty")
	ErrProjectTooLong = errors.New("project name exceeds 100 characters")
	ErrNotFound       = errors.New("session not found")
	ErrAlreadyExists  = errors.New("session already exists")
)
