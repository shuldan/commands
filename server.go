package commands

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

type ServerOption func(*serverConfig)

type serverConfig struct {
	logger Logger
}

func WithServerLogger(l Logger) ServerOption {
	return func(c *serverConfig) {
		c.logger = l
	}
}

type CommandServer struct {
	transport Transport
	codec     Codec
	logger    Logger
	routes    map[string]CommandHandler
	opened    atomic.Bool
	wg        sync.WaitGroup
}

func NewCommandServer(transport Transport, codec Codec, opts ...ServerOption) (*CommandServer, error) {
	if transport == nil {
		return nil, fmt.Errorf("transport is required")
	}
	if codec == nil {
		return nil, fmt.Errorf("codec is required")
	}

	cfg := &serverConfig{
		logger: noopLogger{},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return &CommandServer{
		transport: transport,
		codec:     codec,
		logger:    cfg.logger,
		routes:    make(map[string]CommandHandler),
	}, nil
}

func (s *CommandServer) Open(ctx context.Context) error {
	if !s.opened.CompareAndSwap(false, true) {
		return ErrAlreadyOpened
	}

	return s.transport.Subscribe(ctx, &dispatcher{
		routes: s.routes,
		logger: s.logger,
		wg:     &s.wg,
	})
}

func (s *CommandServer) Close(ctx context.Context) error {
	if !s.opened.Load() {
		return ErrNotOpened
	}

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return s.transport.Close(ctx)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func Register[C Command](s *CommandServer, handler Handler[C]) error {
	if s.opened.Load() {
		return ErrServerStarted
	}

	var zero C
	name := zero.CommandName()

	if _, exists := s.routes[name]; exists {
		return fmt.Errorf("%w: %s", ErrAlreadyRegistered, name)
	}

	s.routes[name] = &commandHandlerAdapter[C]{
		handler:   handler,
		codec:     s.codec,
		transport: s.transport,
		logger:    s.logger,
	}

	return nil
}

type dispatcher struct {
	routes map[string]CommandHandler
	logger Logger
	wg     *sync.WaitGroup
}

func (d *dispatcher) Handle(ctx context.Context, env CommandEnvelope) {
	handler, ok := d.routes[env.CommandName]
	if !ok {
		d.logger.Warn("no handler registered",
			"command", env.CommandName,
			"correlation_id", env.CorrelationID,
		)
		return
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		handler.Handle(ctx, env)
	}()
}

type commandHandlerAdapter[C Command] struct {
	handler   Handler[C]
	codec     Codec
	transport Transport
	logger    Logger
}

func (a *commandHandlerAdapter[C]) Handle(ctx context.Context, env CommandEnvelope) {
	var cmd C
	if err := a.codec.Decode(env.Payload, &cmd); err != nil {
		a.logger.Error("failed to decode command",
			"command", env.CommandName,
			"error", err,
		)
		a.sendInternalError(ctx, env)
		return
	}

	reply := NewReplySender(a.transport, a.codec, ReplyAddress{
		CorrelationID: env.CorrelationID,
		ReplyTo:       env.ReplyTo,
	})

	if err := a.handler.Handle(ctx, cmd, reply); err != nil {
		a.logger.Error("handler failed",
			"command", env.CommandName,
			"error", err,
		)
		a.sendInternalError(ctx, env)
	}
}

func (a *commandHandlerAdapter[C]) sendInternalError(ctx context.Context, env CommandEnvelope) {
	replyEnv := ReplyEnvelope{
		CorrelationID: env.CorrelationID,
		Error:         ErrInternal,
	}
	if err := a.transport.Reply(ctx, replyEnv); err != nil {
		a.logger.Error("failed to send error reply",
			"command", env.CommandName,
			"error", err,
		)
	}
}
