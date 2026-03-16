package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/shuldan/commands"
)

type cmdHandlerFunc func(ctx context.Context, env commands.CommandEnvelope)

func (f cmdHandlerFunc) Handle(ctx context.Context, env commands.CommandEnvelope) {
	f(ctx, env)
}

type replyHandlerFunc func(ctx context.Context, env commands.ReplyEnvelope)

func (f replyHandlerFunc) Handle(ctx context.Context, env commands.ReplyEnvelope) {
	f(ctx, env)
}

func TestNew(t *testing.T) {
	t.Parallel()

	tr := New()
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestTransport_ReplyAddress(t *testing.T) {
	t.Parallel()

	tr := New()
	addr := tr.ReplyAddress()
	if addr != "memory://reply" {
		t.Fatalf("expected 'memory://reply', got '%s'", addr)
	}
}

func TestTransport_Subscribe_Success(t *testing.T) {
	t.Parallel()

	tr := New()
	err := tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransport_Subscribe_Twice(t *testing.T) {
	t.Parallel()

	tr := New()
	h := cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {})
	_ = tr.Subscribe(context.Background(), h)
	err := tr.Subscribe(context.Background(), h)
	if err == nil {
		t.Fatal("expected error on double subscribe")
	}
}

func TestTransport_Subscribe_Closed(t *testing.T) {
	t.Parallel()

	tr := New()
	_ = tr.Close(context.Background())
	err := tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {}))
	if err == nil {
		t.Fatal("expected error on closed transport")
	}
}

func TestTransport_SubscribeReplies_Success(t *testing.T) {
	t.Parallel()

	tr := New()
	err := tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransport_SubscribeReplies_Twice(t *testing.T) {
	t.Parallel()

	tr := New()
	h := replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {})
	_ = tr.SubscribeReplies(context.Background(), h)
	err := tr.SubscribeReplies(context.Background(), h)
	if err == nil {
		t.Fatal("expected error on double subscribe")
	}
}

func TestTransport_SubscribeReplies_Closed(t *testing.T) {
	t.Parallel()

	tr := New()
	_ = tr.Close(context.Background())
	err := tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {}))
	if err == nil {
		t.Fatal("expected error on closed transport")
	}
}

func TestTransport_Send_Success(t *testing.T) {
	t.Parallel()

	tr := New()
	var wg sync.WaitGroup
	var received commands.CommandEnvelope

	wg.Add(1)
	_ = tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, env commands.CommandEnvelope) {
		received = env
		wg.Done()
	}))

	env := commands.CommandEnvelope{
		MessageID:   "m1",
		CommandName: "test.cmd",
		Payload:     []byte(`{}`),
	}
	err := tr.Send(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wg.Wait()
	if received.MessageID != "m1" {
		t.Fatalf("expected message ID 'm1', got '%s'", received.MessageID)
	}
}

func TestTransport_Send_NoHandler(t *testing.T) {
	t.Parallel()

	tr := New()
	err := tr.Send(context.Background(), commands.CommandEnvelope{})
	if err == nil {
		t.Fatal("expected error when no handler subscribed")
	}
}

func TestTransport_Send_Closed(t *testing.T) {
	t.Parallel()

	tr := New()
	_ = tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {}))
	_ = tr.Close(context.Background())

	err := tr.Send(context.Background(), commands.CommandEnvelope{})
	if err == nil {
		t.Fatal("expected error on closed transport")
	}
}

func TestTransport_Reply_Success(t *testing.T) {
	t.Parallel()

	tr := New()
	var wg sync.WaitGroup
	var received commands.ReplyEnvelope

	wg.Add(1)
	_ = tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, env commands.ReplyEnvelope) {
		received = env
		wg.Done()
	}))

	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		ResultName:    "test.result",
	}
	err := tr.Reply(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wg.Wait()
	if received.CorrelationID != "c1" {
		t.Fatalf("expected correlation ID 'c1', got '%s'", received.CorrelationID)
	}
}

func TestTransport_Reply_NoHandler(t *testing.T) {
	t.Parallel()

	tr := New()
	err := tr.Reply(context.Background(), commands.ReplyEnvelope{})
	if err == nil {
		t.Fatal("expected error when no reply handler subscribed")
	}
}

func TestTransport_Reply_Closed(t *testing.T) {
	t.Parallel()

	tr := New()
	_ = tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {}))
	_ = tr.Close(context.Background())

	err := tr.Reply(context.Background(), commands.ReplyEnvelope{})
	if err == nil {
		t.Fatal("expected error on closed transport")
	}
}

func TestTransport_Close_Success(t *testing.T) {
	t.Parallel()

	tr := New()
	err := tr.Close(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransport_Send_AsyncDelivery(t *testing.T) {
	t.Parallel()

	tr := New()
	delivered := make(chan struct{})

	_ = tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {
		close(delivered)
	}))

	_ = tr.Send(context.Background(), commands.CommandEnvelope{CommandName: "test"})

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("expected async delivery within 1 second")
	}
}

func TestTransport_Reply_AsyncDelivery(t *testing.T) {
	t.Parallel()

	tr := New()
	delivered := make(chan struct{})

	_ = tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {
		close(delivered)
	}))

	_ = tr.Reply(context.Background(), commands.ReplyEnvelope{CorrelationID: "c1"})

	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("expected async delivery within 1 second")
	}
}

func TestTransport_Close_ClearsHandlers(t *testing.T) {
	t.Parallel()

	tr := New()
	_ = tr.Subscribe(context.Background(), cmdHandlerFunc(func(_ context.Context, _ commands.CommandEnvelope) {}))
	_ = tr.SubscribeReplies(context.Background(), replyHandlerFunc(func(_ context.Context, _ commands.ReplyEnvelope) {}))
	_ = tr.Close(context.Background())

	err := tr.Send(context.Background(), commands.CommandEnvelope{})
	if err == nil {
		t.Fatal("expected error after close")
	}
	err = tr.Reply(context.Background(), commands.ReplyEnvelope{})
	if err == nil {
		t.Fatal("expected error after close")
	}
}
