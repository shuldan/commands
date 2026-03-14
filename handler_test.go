package commands

import (
	"errors"
	"testing"
)

func TestDefaultErrorHandler_Handle(t *testing.T) {
	t.Parallel()

	h := &defaultErrorHandler{}
	h.Handle(ErrorContext{
		CommandName:    "test.cmd",
		IdempotencyKey: "key-1",
		Err:            errors.New("test error"),
	})
}

func TestDefaultPanicHandler_Handle(t *testing.T) {
	t.Parallel()

	h := &defaultPanicHandler{}
	h.Handle(PanicContext{
		CommandName:    "test.cmd",
		IdempotencyKey: "key-1",
		PanicValue:     "panic!",
		Stack:          []byte("stack trace"),
	})
}

func TestErrorContext_Fields(t *testing.T) {
	t.Parallel()

	ctx := ErrorContext{
		CommandName:    "cmd",
		IdempotencyKey: "key",
		Err:            errors.New("err"),
	}

	if ctx.CommandName != "cmd" {
		t.Errorf("expected 'cmd', got %q", ctx.CommandName)
	}

	if ctx.IdempotencyKey != "key" {
		t.Errorf("expected 'key', got %q", ctx.IdempotencyKey)
	}

	if ctx.Err == nil {
		t.Error("expected non-nil error")
	}
}

func TestPanicContext_Fields(t *testing.T) {
	t.Parallel()

	ctx := PanicContext{
		CommandName:    "cmd",
		IdempotencyKey: "key",
		PanicValue:     42,
		Stack:          []byte("trace"),
	}

	if ctx.PanicValue != 42 {
		t.Errorf("expected 42, got %v", ctx.PanicValue)
	}

	if len(ctx.Stack) == 0 {
		t.Error("expected non-empty stack")
	}
}
