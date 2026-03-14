package commands

import "errors"

var (
	ErrDispatcherClosed    = errors.New("commands: dispatcher is closed")
	ErrHandlerExists       = errors.New("commands: handler already registered for this command")
	ErrHandlerNotFound     = errors.New("commands: no handler registered for this command")
	ErrResultHandlerExists = errors.New("commands: result handler already registered for this command")
	ErrNilHandler          = errors.New("commands: handler cannot be nil")
	ErrNilCommand          = errors.New("commands: command cannot be nil")
	ErrTypeMismatch        = errors.New("commands: command type mismatch in handler")
	ErrDuplicateCommand    = errors.New("commands: duplicate command (idempotency)")
	ErrNilBackoff          = errors.New("commands: backoff strategy cannot be nil")
	ErrNilErrorHandler     = errors.New("commands: error handler cannot be nil")
	ErrNilPanicHandler     = errors.New("commands: panic handler cannot be nil")
)
