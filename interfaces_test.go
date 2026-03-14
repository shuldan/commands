package commands

import (
	"context"
	"errors"
	"testing"
)

type stubHandler struct {
	result Result
	err    error
}

func (h *stubHandler) Handle(_ context.Context, _ testCommand) (Result, error) {
	return h.result, h.err
}

func TestHandlerFunc_Handle(t *testing.T) {
	t.Parallel()

	called := false
	fn := HandlerFunc[testCommand](func(_ context.Context, _ testCommand) (Result, error) {
		called = true
		return BaseResult{Name: "ok"}, nil
	})

	result, err := fn.Handle(context.Background(), testCommand{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected handler func to be called")
	}

	if result.ResultName() != "ok" {
		t.Errorf("expected result name 'ok', got %q", result.ResultName())
	}
}

func TestHandlerFunc_Handle_Error(t *testing.T) {
	t.Parallel()

	expected := errors.New("handler error")
	fn := HandlerFunc[testCommand](func(_ context.Context, _ testCommand) (Result, error) {
		return nil, expected
	})

	_, err := fn.Handle(context.Background(), testCommand{})
	if !errors.Is(err, expected) {
		t.Errorf("expected %v, got %v", expected, err)
	}
}

type stubResultHandler struct {
	called bool
	gotErr error
}

func (h *stubResultHandler) Handle(_ context.Context, _ BaseResult, err error) error {
	h.called = true
	h.gotErr = err
	return nil
}

func TestResultHandlerFunc_Handle(t *testing.T) {
	t.Parallel()

	called := false
	fn := ResultHandlerFunc[BaseResult](func(_ context.Context, r BaseResult, err error) error {
		called = true
		if err != nil {
			return err
		}

		if r.ResultName() != "test" {
			t.Errorf("expected result 'test', got %q", r.ResultName())
		}
		return nil
	})

	err := fn.Handle(context.Background(), BaseResult{Name: "test"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("expected result handler func to be called")
	}
}

func TestResultHandlerFunc_Handle_WithError(t *testing.T) {
	t.Parallel()

	expected := errors.New("cmd error")
	fn := ResultHandlerFunc[BaseResult](func(_ context.Context, _ BaseResult, err error) error {
		return err
	})

	err := fn.Handle(context.Background(), BaseResult{}, expected)
	if !errors.Is(err, expected) {
		t.Errorf("expected %v, got %v", expected, err)
	}
}
