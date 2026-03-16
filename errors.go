package commands

import (
	"errors"
	"fmt"
)

// ErrorPayload represents a business error sent through the command bus.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Error implements the error interface.
func (e *ErrorPayload) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

var (
	ErrInternal          = &ErrorPayload{Code: "INTERNAL", Message: "internal server error"}
	ErrCommandNotFound   = &ErrorPayload{Code: "COMMAND_NOT_FOUND", Message: "no handler registered"}
	ErrTimeout           = errors.New("command timeout")
	ErrReplyAlreadySent  = errors.New("reply already sent")
	ErrClientClosed      = errors.New("client is closed")
	ErrServerClosed      = errors.New("server is closed")
	ErrAlreadyOpened     = errors.New("already opened")
	ErrNotOpened         = errors.New("not opened")
	ErrAlreadyRegistered = errors.New("handler already registered for this command")
	ErrServerStarted     = errors.New("cannot register handler after server is opened")
)
