package commands

import (
	"context"
	"errors"
	"testing"
)

func TestSend_WithMiddleware(t *testing.T) {
	t.Parallel()

	var order []string

	mw := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, cmd Command) (Result, error) {
			order = append(order, "mw-before")
			r, err := next(ctx, cmd)
			order = append(order, "mw-after")
			return r, err
		}
	}

	d := New(WithSyncMode(), WithMiddleware(mw))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		order = append(order, "handler")
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "mw-key"))

	expected := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d steps, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("step %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestSend_MiddlewareCanShortCircuit(t *testing.T) {
	t.Parallel()

	shortErr := errors.New("blocked by middleware")
	mw := func(_ HandleFunc) HandleFunc {
		return func(_ context.Context, _ Command) (Result, error) {
			return nil, shortErr
		}
	}

	var errCaught error
	eh := &testErrorHandler{onError: func(ctx ErrorContext) { errCaught = ctx.Err }}

	d := New(WithSyncMode(), WithMiddleware(mw), WithErrorHandler(eh))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		t.Fatal("handler should not be called")
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "blocked"))

	if !errors.Is(errCaught, shortErr) {
		t.Errorf("expected short circuit error, got %v", errCaught)
	}
}

func TestSend_MultipleMiddleware(t *testing.T) {
	t.Parallel()

	var order []string

	makeMW := func(name string) Middleware {
		return func(next HandleFunc) HandleFunc {
			return func(ctx context.Context, cmd Command) (Result, error) {
				order = append(order, name+"-before")
				r, err := next(ctx, cmd)
				order = append(order, name+"-after")
				return r, err
			}
		}
	}

	d := New(WithSyncMode(), WithMiddleware(makeMW("A"), makeMW("B")))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		order = append(order, "handler")
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "multi-mw"))

	expected := []string{
		"A-before", "B-before", "handler", "B-after", "A-after",
	}

	if len(order) != len(expected) {
		t.Fatalf("expected %d steps, got %d: %v", len(expected), len(order), order)
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("step %d: expected %q, got %q", i, v, order[i])
		}
	}
}
