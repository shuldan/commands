package commands

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewCommandClient_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}

	c, err := NewCommandClient(tr, codec)
	assertError(t, err, false)
	assertNotNil(t, c, "client")
}

func TestNewCommandClient_NilTransport(t *testing.T) {
	t.Parallel()

	_, err := NewCommandClient(nil, &testCodec{})
	assertError(t, err, true)
}

func TestNewCommandClient_NilCodec(t *testing.T) {
	t.Parallel()

	_, err := NewCommandClient(newTestTransport(), nil)
	assertError(t, err, true)
}

func TestNewCommandClient_WithOptions(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}

	c, err := NewCommandClient(tr, codec,
		WithTimeout(5*time.Second),
		WithClientLogger(noopLogger{}),
	)
	assertError(t, err, false)
	assertEqual(t, c.defaultTimeout, 5*time.Second)
}

func TestCommandClient_Open_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	err := c.Open(context.Background())
	assertError(t, err, false)
}

func TestCommandClient_Open_Twice(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	_ = c.Open(context.Background())
	err := c.Open(context.Background())
	assertErrorIs(t, err, ErrAlreadyOpened)
}

func TestCommandClient_Open_SubscribeError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.subRepliesFunc = func(_ context.Context, _ ReplyHandler) error {
		return fmt.Errorf("subscribe fail")
	}
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	err := c.Open(context.Background())
	assertError(t, err, true)
}

func TestCommandClient_Send_NotOpened(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	_, err := c.Send(context.Background(), testCommand{Name: "test"})
	assertErrorIs(t, err, ErrNotOpened)
}

func TestCommandClient_Send_Closed(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())
	_ = c.Close(context.Background())

	_, err := c.Send(context.Background(), testCommand{Name: "test"})
	assertErrorIs(t, err, ErrClientClosed)
}

func TestCommandClient_Send_EncodeError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{
		encodeFunc: func(_ any) ([]byte, error) { return nil, fmt.Errorf("encode fail") },
	}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	_, err := c.Send(context.Background(), testCommand{Name: "test"})
	assertError(t, err, true)
}

func TestCommandClient_Send_TransportError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.sendFunc = func(_ context.Context, _ CommandEnvelope) error {
		return fmt.Errorf("send fail")
	}
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	_, err := c.Send(context.Background(), testCommand{Name: "test"})
	assertError(t, err, true)
}

func TestCommandClient_Send_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	var captured CommandEnvelope
	tr.sendFunc = func(_ context.Context, env CommandEnvelope) error {
		captured = env
		return nil
	}
	codec := &testCodec{
		encodeFunc: func(_ any) ([]byte, error) { return []byte(`"cmd"`), nil },
	}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	f, err := c.Send(context.Background(), testCommand{Name: "test.cmd"})
	assertError(t, err, false)
	assertNotNil(t, f, "future")
	assertEqual(t, captured.CommandName, "test.cmd")
	assertEqual(t, captured.ReplyTo, "test://reply")
}

func TestCommandClient_Send_WithTimeout(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.sendFunc = func(_ context.Context, _ CommandEnvelope) error { return nil }
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec, WithTimeout(10*time.Second))
	_ = c.Open(context.Background())

	f, err := c.Send(context.Background(), testCommand{Name: "test"},
		WithSendTimeout(50*time.Millisecond))
	assertError(t, err, false)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = f.Await(ctx)
	assertErrorIs(t, err, ErrTimeout)
}

func TestCommandClient_Close_NotOpened(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	err := c.Close(context.Background())
	assertErrorIs(t, err, ErrNotOpened)
}

func TestCommandClient_Close_Twice(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())
	_ = c.Close(context.Background())

	err := c.Close(context.Background())
	assertErrorIs(t, err, ErrClientClosed)
}

func TestCommandClient_Close_CancelsPendingFutures(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.sendFunc = func(_ context.Context, _ CommandEnvelope) error { return nil }
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	f, _ := c.Send(context.Background(), testCommand{Name: "test"})
	_ = c.Close(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := f.Await(ctx)
	assertErrorIs(t, err, ErrClientClosed)
}

func TestCommandClient_HandleReply_UnknownCorrelation(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	c.handleReply(context.Background(), ReplyEnvelope{CorrelationID: "unknown"})
}

func TestTypedSend_Success(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{
		encodeFunc: func(_ any) ([]byte, error) { return []byte(`{}`), nil },
		decodeFunc: func(_ []byte, v any) error {
			if r, ok := v.(*testResult); ok {
				r.Value = "decoded"
			}
			return nil
		},
	}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	tr.sendFunc = func(_ context.Context, env CommandEnvelope) error {
		go c.handleReply(context.Background(), ReplyEnvelope{
			CorrelationID: env.CorrelationID,
			ResultName:    "test.result",
			Payload:       []byte(`{}`),
		})
		return nil
	}

	f, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertError(t, err, false)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := f.Await(ctx)
	assertError(t, err, false)
	assertEqual(t, result.Value, "decoded")
}

func TestTypedSend_NotOpened(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)

	_, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertErrorIs(t, err, ErrNotOpened)
}

func TestTypedSend_Closed(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())
	_ = c.Close(context.Background())

	_, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertErrorIs(t, err, ErrClientClosed)
}

func TestTypedSend_EncodeError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{
		encodeFunc: func(_ any) ([]byte, error) { return nil, fmt.Errorf("fail") },
	}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	_, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertError(t, err, true)
}

func TestTypedSend_TransportError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.sendFunc = func(_ context.Context, _ CommandEnvelope) error {
		return fmt.Errorf("send fail")
	}
	codec := &testCodec{}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	_, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertError(t, err, true)
}

func TestTypedSend_ErrorReply(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{
		encodeFunc: func(_ any) ([]byte, error) { return []byte(`{}`), nil },
	}
	c, _ := NewCommandClient(tr, codec)
	_ = c.Open(context.Background())

	tr.sendFunc = func(_ context.Context, env CommandEnvelope) error {
		go c.handleReply(context.Background(), ReplyEnvelope{
			CorrelationID: env.CorrelationID,
			Error:         &ErrorPayload{Code: "BIZ", Message: "no"},
		})
		return nil
	}

	f, err := Send[testResult](context.Background(), c, testCommand{Name: "test"})
	assertError(t, err, false)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = f.Await(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	assertEqual(t, err.Error(), "BIZ: no")
}

func TestGenerateID_Unique(t *testing.T) {
	t.Parallel()

	ids := make(map[string]struct{}, 1000)
	for i := 0; i < 1000; i++ {
		id := generateID()
		if len(id) != 32 {
			t.Fatalf("expected 32 chars, got %d", len(id))
		}
		if _, exists := ids[id]; exists {
			t.Fatal("duplicate ID generated")
		}
		ids[id] = struct{}{}
	}
}
