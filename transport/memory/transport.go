package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/shuldan/commands"
)

// Transport is an in-memory implementation of commands.Transport.
// Suitable for testing and single-process usage.
type Transport struct {
	mu             sync.RWMutex
	commandHandler commands.CommandHandler
	replyHandler   commands.ReplyHandler
	replyAddress   string
	closed         bool
}

// New creates a new in-memory transport.
func New() *Transport {
	return &Transport{
		replyAddress: "memory://reply",
	}
}

// Send delivers a command envelope to the subscribed command handler.
func (t *Transport) Send(ctx context.Context, env commands.CommandEnvelope) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	if t.commandHandler == nil {
		return fmt.Errorf("no command handler subscribed")
	}

	go t.commandHandler.Handle(context.WithoutCancel(ctx), env)
	return nil
}

// Subscribe registers a handler for incoming commands.
func (t *Transport) Subscribe(_ context.Context, handler commands.CommandHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	if t.commandHandler != nil {
		return fmt.Errorf("command handler already subscribed")
	}

	t.commandHandler = handler
	return nil
}

// Reply delivers a reply envelope to the subscribed reply handler.
func (t *Transport) Reply(ctx context.Context, env commands.ReplyEnvelope) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	if t.replyHandler == nil {
		return fmt.Errorf("no reply handler subscribed")
	}

	go t.replyHandler.Handle(context.WithoutCancel(ctx), env)
	return nil
}

// SubscribeReplies registers a handler for incoming replies.
func (t *Transport) SubscribeReplies(_ context.Context, handler commands.ReplyHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("transport is closed")
	}
	if t.replyHandler != nil {
		return fmt.Errorf("reply handler already subscribed")
	}

	t.replyHandler = handler
	return nil
}

// ReplyAddress returns the reply address for this transport instance.
func (t *Transport) ReplyAddress() string {
	return t.replyAddress
}

// Close shuts down the transport.
func (t *Transport) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.closed = true
	t.commandHandler = nil
	t.replyHandler = nil
	return nil
}
