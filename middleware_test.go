package commands

import (
	"context"
	"testing"
)

func TestBuildChain_NoMiddleware(t *testing.T) {
	t.Parallel()

	called := false
	base := func(_ context.Context, _ Command) (Result, error) {
		called = true
		return nil, nil
	}

	chain := buildChain(base, nil)
	_, _ = chain(context.Background(), newTestCommand("test", "k"))

	if !called {
		t.Error("expected base handler to be called")
	}
}

func TestBuildChain_SingleMiddleware(t *testing.T) {
	t.Parallel()

	var order []string

	mw := func(next HandleFunc) HandleFunc {
		return func(ctx context.Context, cmd Command) (Result, error) {
			order = append(order, "before")
			r, err := next(ctx, cmd)
			order = append(order, "after")
			return r, err
		}
	}

	base := func(_ context.Context, _ Command) (Result, error) {
		order = append(order, "handler")
		return nil, nil
	}

	chain := buildChain(base, []Middleware{mw})
	_, _ = chain(context.Background(), newTestCommand("test", "k"))

	expected := []string{"before", "handler", "after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("step %d: expected %q, got %q", i, v, order[i])
		}
	}
}

func TestBuildChain_MultipleMiddleware_Order(t *testing.T) {
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

	base := func(_ context.Context, _ Command) (Result, error) {
		order = append(order, "handler")
		return nil, nil
	}

	chain := buildChain(base, []Middleware{makeMW("1"), makeMW("2")})
	_, _ = chain(context.Background(), newTestCommand("test", "k"))

	expected := []string{"1-before", "2-before", "handler", "2-after", "1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("step %d: expected %q, got %q", i, v, order[i])
		}
	}
}
