package commands

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

type testCommand struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func (c testCommand) CommandName() string { return c.Name }

type testResult struct {
	Value string `json:"value"`
}

func (r testResult) ResultName() string { return "test.result" }

type wrongResult struct{}

func (r *wrongResult) ResultName() string { return "wrong" }

type testHandler struct {
	handleFunc func(ctx context.Context, cmd testCommand, reply ReplySender) error
}

func (h *testHandler) Handle(ctx context.Context, cmd testCommand, reply ReplySender) error {
	return h.handleFunc(ctx, cmd, reply)
}

type testCodec struct {
	encodeFunc func(v any) ([]byte, error)
	decodeFunc func(data []byte, v any) error
}

func (c *testCodec) Encode(v any) ([]byte, error) {
	if c.encodeFunc != nil {
		return c.encodeFunc(v)
	}
	return []byte(`{}`), nil
}

func (c *testCodec) Decode(data []byte, v any) error {
	if c.decodeFunc != nil {
		return c.decodeFunc(data, v)
	}
	return nil
}

type testTransport struct {
	mu             sync.Mutex
	cmdHandler     CommandHandler
	replyHandler   ReplyHandler
	sendFunc       func(ctx context.Context, env CommandEnvelope) error
	replyFunc      func(ctx context.Context, env ReplyEnvelope) error
	subscribeFunc  func(ctx context.Context, handler CommandHandler) error
	subRepliesFunc func(ctx context.Context, handler ReplyHandler) error
	closeFunc      func(ctx context.Context) error
	replyAddr      string
}

func newTestTransport() *testTransport {
	return &testTransport{replyAddr: "test://reply"}
}

func (t *testTransport) Send(ctx context.Context, env CommandEnvelope) error {
	if t.sendFunc != nil {
		return t.sendFunc(ctx, env)
	}
	t.mu.Lock()
	h := t.cmdHandler
	t.mu.Unlock()
	if h != nil {
		go h.Handle(context.Background(), env)
	}
	return nil
}

func (t *testTransport) Subscribe(_ context.Context, handler CommandHandler) error {
	if t.subscribeFunc != nil {
		return t.subscribeFunc(context.Background(), handler)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cmdHandler = handler
	return nil
}

func (t *testTransport) Reply(ctx context.Context, env ReplyEnvelope) error {
	if t.replyFunc != nil {
		return t.replyFunc(ctx, env)
	}
	t.mu.Lock()
	h := t.replyHandler
	t.mu.Unlock()
	if h != nil {
		go h.Handle(context.Background(), env)
	}
	return nil
}

func (t *testTransport) SubscribeReplies(_ context.Context, handler ReplyHandler) error {
	if t.subRepliesFunc != nil {
		return t.subRepliesFunc(context.Background(), handler)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.replyHandler = handler
	return nil
}

func (t *testTransport) ReplyAddress() string {
	return t.replyAddr
}

func (t *testTransport) Close(_ context.Context) error {
	if t.closeFunc != nil {
		return t.closeFunc(context.Background())
	}
	return nil
}

func assertError(t *testing.T, err error, want bool) {
	t.Helper()
	if want && err == nil {
		t.Fatal("expected error, got nil")
	}
	if !want && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertErrorIs(t *testing.T, err, target error) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %v, got nil", target)
	}
	if !errorContains(err, target) {
		t.Fatalf("expected error %v, got %v", target, err)
	}
}

func errorContains(err, target error) bool {
	if err == target {
		return true
	}
	for e := err; e != nil; {
		if e.Error() == target.Error() {
			return true
		}
		u, ok := e.(interface{ Unwrap() error })
		if !ok {
			break
		}
		e = u.Unwrap()
	}
	return false
}

func assertNil(t *testing.T, v any) {
	t.Helper()
	if v != nil {
		t.Fatalf("expected nil, got %v", v)
	}
}

func assertNotNil(t *testing.T, v any, msg string) {
	t.Helper()
	if v == nil {
		t.Fatalf("expected non-nil: %s", msg)
	}
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

type cmdHandlerFuncWrapper func(ctx context.Context, env CommandEnvelope)

func (f cmdHandlerFuncWrapper) Handle(ctx context.Context, env CommandEnvelope) {
	f(ctx, env)
}
