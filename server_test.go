package commands

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCommandServer_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}

	s, err := NewCommandServer(tr, codec)
	assertError(t, err, false)
	assertNotNil(t, s, "server")
}

func TestNewCommandServer_NilTransport(t *testing.T) {
	t.Parallel()

	_, err := NewCommandServer(nil, &testCodec{})
	assertError(t, err, true)
}

func TestNewCommandServer_NilCodec(t *testing.T) {
	t.Parallel()

	_, err := NewCommandServer(newTestTransport(), nil)
	assertError(t, err, true)
}

func TestNewCommandServer_WithLogger(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	logger := &noopLogger{}

	s, err := NewCommandServer(tr, codec, WithServerLogger(logger))
	assertError(t, err, false)
	assertNotNil(t, s, "server")
}

func TestRegister_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return nil
	}}
	err := Register[testCommand](s, h)
	assertError(t, err, false)
}

func TestRegister_Duplicate(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return nil
	}}
	_ = Register[testCommand](s, h)
	err := Register[testCommand](s, h)

	assertError(t, err, true)
}

func TestRegister_AfterOpen(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	_ = s.Open(context.Background())

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return nil
	}}
	err := Register[testCommand](s, h)
	assertErrorIs(t, err, ErrServerStarted)
}

func TestCommandServer_Open_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	err := s.Open(context.Background())
	assertError(t, err, false)
}

func TestCommandServer_Open_Twice(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	_ = s.Open(context.Background())
	err := s.Open(context.Background())
	assertErrorIs(t, err, ErrAlreadyOpened)
}

func TestCommandServer_Open_SubscribeError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.subscribeFunc = func(_ context.Context, _ CommandHandler) error {
		return fmt.Errorf("subscribe fail")
	}
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	err := s.Open(context.Background())
	assertError(t, err, true)
}

func TestCommandServer_Close_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)
	_ = s.Open(context.Background())

	err := s.Close(context.Background())
	assertError(t, err, false)
}

func TestCommandServer_Close_NotOpened(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	err := s.Close(context.Background())
	assertErrorIs(t, err, ErrNotOpened)
}

func TestCommandServer_Close_WaitsForHandlers(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	var handlerDone atomic.Bool
	handlerStarted := make(chan struct{})

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		close(handlerStarted)
		time.Sleep(50 * time.Millisecond)
		handlerDone.Store(true)
		return nil
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.Send(context.Background(), CommandEnvelope{
		CommandName: testCommand{}.CommandName(),
		Payload:     []byte(`{}`),
	})

	<-handlerStarted

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.Close(ctx)

	if !handlerDone.Load() {
		t.Fatal("expected handler to finish before close returns")
	}
}

func TestCommandServer_Close_ContextExpired(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	handlerStarted := make(chan struct{})

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		close(handlerStarted)
		time.Sleep(5 * time.Second)
		return nil
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.Send(context.Background(), CommandEnvelope{
		CommandName: testCommand{}.CommandName(),
		Payload:     []byte(`{}`),
	})

	<-handlerStarted

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := s.Close(ctx)

	assertErrorIs(t, err, context.DeadlineExceeded)
}

func TestDispatcher_UnknownCommand(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)
	_ = s.Open(context.Background())

	done := make(chan struct{})
	originalHandler := tr.cmdHandler
	wrappedHandler := cmdHandlerFuncWrapper(func(ctx context.Context, env CommandEnvelope) {
		originalHandler.Handle(ctx, env)
		close(done)
	})

	tr.mu.Lock()
	tr.cmdHandler = wrappedHandler
	tr.mu.Unlock()

	tr.Send(context.Background(), CommandEnvelope{
		CommandName: "unknown.command",
		Payload:     []byte(`{}`),
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for dispatcher")
	}
}

func TestAdapter_DecodeError(t *testing.T) {
	t.Parallel()

	replyCh := make(chan ReplyEnvelope, 1)
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		replyCh <- env
		return nil
	}
	codec := &testCodec{
		decodeFunc: func(_ []byte, _ any) error { return fmt.Errorf("decode fail") },
	}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return nil
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		Payload:       []byte(`invalid`),
	})

	select {
	case env := <-replyCh:
		if env.Error == nil || env.Error.Code != "INTERNAL" {
			t.Fatal("expected INTERNAL error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error reply")
	}
}

func TestAdapter_HandlerError(t *testing.T) {
	t.Parallel()

	replyCh := make(chan ReplyEnvelope, 1)
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		replyCh <- env
		return nil
	}
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return fmt.Errorf("handler fail")
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		Payload:       []byte(`{}`),
	})

	select {
	case env := <-replyCh:
		if env.Error == nil || env.Error.Code != "INTERNAL" {
			t.Fatal("expected INTERNAL error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for error reply")
	}
}

func TestAdapter_HandlerSuccess_NoReply(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	replyCount := atomic.Int32{}
	tr.replyFunc = func(_ context.Context, _ ReplyEnvelope) error {
		replyCount.Add(1)
		return nil
	}
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	done := make(chan struct{})
	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		defer close(done)
		// Команда принята, ответ будет позже.
		return nil
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		Payload:       []byte(`{}`),
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler")
	}

	time.Sleep(20 * time.Millisecond)

	if replyCount.Load() != 0 {
		t.Fatalf("expected 0 replies (deferred reply), got %d", replyCount.Load())
	}
}

func TestAdapter_HandlerSendsReplyImmediately(t *testing.T) {
	t.Parallel()

	replyCh := make(chan ReplyEnvelope, 1)
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		replyCh <- env
		return nil
	}
	codec := &testCodec{
		encodeFunc: func(v any) ([]byte, error) { return []byte(`"done"`), nil },
	}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(ctx context.Context, _ testCommand, reply ReplySender) error {
		return reply.Send(ctx, testResult{Value: "done"})
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		Payload:       []byte(`{}`),
	})

	select {
	case env := <-replyCh:
		assertEqual(t, env.CorrelationID, "c1")
		assertEqual(t, string(env.Payload), `"done"`)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reply")
	}
}

func TestAdapter_HandlerSavesAddressForDeferredReply(t *testing.T) {
	t.Parallel()

	replyCh := make(chan ReplyEnvelope, 1)
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		replyCh <- env
		return nil
	}
	codec := &testCodec{
		encodeFunc: func(v any) ([]byte, error) { return []byte(`"approved"`), nil },
	}
	s, _ := NewCommandServer(tr, codec)

	addrCh := make(chan ReplyAddress, 1)
	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, reply ReplySender) error {
		addrCh <- reply.Address()
		return nil
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		ReplyTo:       "reply-queue",
		Payload:       []byte(`{}`),
	})

	var savedAddr ReplyAddress
	select {
	case savedAddr = <-addrCh:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for handler")
	}

	assertEqual(t, savedAddr.CorrelationID, "c1")
	assertEqual(t, savedAddr.ReplyTo, "reply-queue")

	// Позже — отправляем ответ из сохранённого адреса.
	deferred := NewReplySender(tr, codec, savedAddr)
	err := deferred.Send(context.Background(), testResult{Value: "approved"})
	assertError(t, err, false)

	select {
	case env := <-replyCh:
		assertEqual(t, env.CorrelationID, "c1")
		assertEqual(t, string(env.Payload), `"approved"`)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for deferred reply")
	}
}

func TestAdapter_SendInternalError_TransportFail(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, _ ReplyEnvelope) error {
		defer func() { close(done) }()
		return fmt.Errorf("transport fail")
	}
	codec := &testCodec{}
	s, _ := NewCommandServer(tr, codec)

	h := &testHandler{handleFunc: func(_ context.Context, _ testCommand, _ ReplySender) error {
		return fmt.Errorf("handler fail")
	}}
	_ = Register[testCommand](s, h)
	_ = s.Open(context.Background())

	tr.cmdHandler.Handle(context.Background(), CommandEnvelope{
		CommandName:   testCommand{}.CommandName(),
		CorrelationID: "c1",
		Payload:       []byte(`{}`),
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}
