package commands

import "context"

// CommandHandler handles raw command envelopes from the transport layer.
type CommandHandler interface {
	Handle(ctx context.Context, env CommandEnvelope)
}

// ReplyHandler handles raw reply envelopes from the transport layer.
type ReplyHandler interface {
	Handle(ctx context.Context, env ReplyEnvelope)
}

// Transport is the abstraction over a message broker.
type Transport interface {
	Send(ctx context.Context, env CommandEnvelope) error
	Subscribe(ctx context.Context, handler CommandHandler) error
	Reply(ctx context.Context, env ReplyEnvelope) error
	SubscribeReplies(ctx context.Context, handler ReplyHandler) error
	ReplyAddress() string
	Close(ctx context.Context) error
}
