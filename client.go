package commands

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"crypto/rand"
	"encoding/hex"
)

// ClientOption configures a CommandClient.
type ClientOption func(*clientConfig)

type clientConfig struct {
	defaultTimeout time.Duration
	logger         Logger
}

// WithTimeout sets the default timeout for all commands.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *clientConfig) {
		c.defaultTimeout = d
	}
}

// WithClientLogger sets the logger for the client.
func WithClientLogger(l Logger) ClientOption {
	return func(c *clientConfig) {
		c.logger = l
	}
}

// SendOption configures a single Send call.
type SendOption func(*sendConfig)

type sendConfig struct {
	timeout time.Duration
}

// WithSendTimeout overrides the default timeout for a single Send call.
func WithSendTimeout(d time.Duration) SendOption {
	return func(c *sendConfig) {
		c.timeout = d
	}
}

// CommandSender is the interface exposed for mocking in tests.
type CommandSender interface {
	Send(ctx context.Context, cmd Command, opts ...SendOption) (Future, error)
	Close(ctx context.Context) error
}

// CommandClient sends commands and receives replies.
type CommandClient struct {
	transport      Transport
	codec          Codec
	logger         Logger
	defaultTimeout time.Duration

	pending sync.Map // correlationID -> *future
	opened  atomic.Bool
	closed  atomic.Bool
}

// NewCommandClient creates a new CommandClient.
func NewCommandClient(transport Transport, codec Codec, opts ...ClientOption) (*CommandClient, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}
	if codec == nil {
		return nil, fmt.Errorf("codec is required")
	}

	cfg := &clientConfig{
		defaultTimeout: 30 * time.Second,
		logger:         noopLogger{},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return &CommandClient{
		transport:      transport,
		codec:          codec,
		logger:         cfg.logger,
		defaultTimeout: cfg.defaultTimeout,
	}, nil
}

// Open subscribes to the reply queue.
func (c *CommandClient) Open(ctx context.Context) error {
	if !c.opened.CompareAndSwap(false, true) {
		return ErrAlreadyOpened
	}

	return c.transport.SubscribeReplies(ctx, replyHandlerFunc(c.handleReply))
}

// Close cancels all pending futures and closes the transport.
func (c *CommandClient) Close(ctx context.Context) error {
	if !c.opened.Load() {
		return ErrNotOpened
	}
	if !c.closed.CompareAndSwap(false, true) {
		return ErrClientClosed
	}

	// Complete all pending futures with error.
	c.pending.Range(func(key, value any) bool {
		if f, ok := value.(*future); ok {
			f.completeWithError(ErrClientClosed)
		}
		c.pending.Delete(key)
		return true
	})

	return c.transport.Close(ctx)
}

// Send sends a command and returns a Future.
func (c *CommandClient) Send(ctx context.Context, cmd Command, opts ...SendOption) (Future, error) {
	if c.closed.Load() {
		return nil, ErrClientClosed
	}
	if !c.opened.Load() {
		return nil, ErrNotOpened
	}

	cfg := &sendConfig{
		timeout: c.defaultTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	payload, err := c.codec.Encode(cmd)
	if err != nil {
		return nil, fmt.Errorf("encode command: %w", err)
	}

	correlationID := generateID()

	f := newFuture(func(env ReplyEnvelope) (Result, error) {
		return nil, nil // will be overridden by typed Send
	})

	// Set up timeout.
	timer := time.AfterFunc(cfg.timeout, func() {
		if _, loaded := c.pending.LoadAndDelete(correlationID); loaded {
			f.completeWithError(ErrTimeout)
		}
	})

	// Store future before sending to avoid race with reply.
	c.pending.Store(correlationID, f)

	env := CommandEnvelope{
		MessageID:     generateID(),
		CorrelationID: correlationID,
		CommandName:   cmd.CommandName(),
		ReplyTo:       c.transport.ReplyAddress(),
		Payload:       payload,
	}

	if err := c.transport.Send(ctx, env); err != nil {
		c.pending.Delete(correlationID)
		timer.Stop()
		return nil, fmt.Errorf("send command: %w", err)
	}

	return f, nil
}

func (c *CommandClient) handleReply(_ context.Context, env ReplyEnvelope) {
	val, ok := c.pending.LoadAndDelete(env.CorrelationID)
	if !ok {
		c.logger.Warn("reply for unknown correlation",
			"correlation_id", env.CorrelationID,
		)
		return
	}

	f := val.(*future)
	f.complete(env)
}

// Send sends a command and returns a TypedFuture with type-safe result deserialization.
func Send[R Result](ctx context.Context, c *CommandClient, cmd Command, opts ...SendOption) (TypedFuture[R], error) {
	if c.closed.Load() {
		return nil, ErrClientClosed
	}
	if !c.opened.Load() {
		return nil, ErrNotOpened
	}

	cfg := &sendConfig{
		timeout: c.defaultTimeout,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	payload, err := c.codec.Encode(cmd)
	if err != nil {
		return nil, fmt.Errorf("encode command: %w", err)
	}

	correlationID := generateID()

	f := newFuture(func(env ReplyEnvelope) (Result, error) {
		var r R
		if err := c.codec.Decode(env.Payload, &r); err != nil {
			return nil, fmt.Errorf("decode result: %w", err)
		}
		return r, nil
	})

	timer := time.AfterFunc(cfg.timeout, func() {
		if _, loaded := c.pending.LoadAndDelete(correlationID); loaded {
			f.completeWithError(ErrTimeout)
		}
	})

	c.pending.Store(correlationID, f)

	env := CommandEnvelope{
		MessageID:     generateID(),
		CorrelationID: correlationID,
		CommandName:   cmd.CommandName(),
		ReplyTo:       c.transport.ReplyAddress(),
		Payload:       payload,
	}

	if err := c.transport.Send(ctx, env); err != nil {
		c.pending.Delete(correlationID)
		timer.Stop()
		return nil, fmt.Errorf("send command: %w", err)
	}

	return &typedFuture[R]{inner: f}, nil
}

// replyHandlerFunc is an adapter to use a function as ReplyHandler.
type replyHandlerFunc func(ctx context.Context, env ReplyEnvelope)

func (f replyHandlerFunc) Handle(ctx context.Context, env ReplyEnvelope) {
	f(ctx, env)
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
