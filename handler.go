package commands

import "context"

// Handler is a typed command handler that contains business logic.
type Handler[C Command] interface {
	Handle(ctx context.Context, cmd C, reply ReplySender) error
}

// ReplySender sends a reply back to the command sender.
type ReplySender interface {
	Send(ctx context.Context, result Result) error
	SendError(ctx context.Context, err error) error
	Address() ReplyAddress
}
