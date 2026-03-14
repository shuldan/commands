package commands

import (
	"context"
	"errors"
	"testing"
)

type createCmd struct {
	BaseCommand
	ID string
}

func (c createCmd) CommandName() string { return "create" }

type createResult struct {
	BaseResult
	ID string
}

type createHandler struct {
	result Result
	err    error
}

func (h *createHandler) Handle(_ context.Context, _ createCmd) (Result, error) {
	return h.result, h.err
}

func TestHandle_StructHandler(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	h := &createHandler{
		result: createResult{BaseResult: BaseResult{Name: "created"}, ID: "1"},
	}

	err := Handle[createCmd](d, h)
	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	err = d.Send(context.Background(), createCmd{ID: "1"})
	if err != nil {
		t.Fatalf("send error: %v", err)
	}
}

func TestHandle_DuplicateRegistration(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	h := &createHandler{}
	_ = Handle[createCmd](d, h)

	err := Handle[createCmd](d, h)
	if !errors.Is(err, ErrHandlerExists) {
		t.Errorf("expected ErrHandlerExists, got %v", err)
	}
}

func TestHandleFunc2_Success(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	var called bool
	err := HandleFn(d, func(_ context.Context, c createCmd) (Result, error) {
		called = true
		return BaseResult{Name: "ok"}, nil
	})

	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	_ = d.Send(context.Background(), createCmd{ID: "1"})

	if !called {
		t.Error("expected handler to be called")
	}
}

func TestHandleFunc2_DuplicateRegistration(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	fn := func(_ context.Context, _ createCmd) (Result, error) {
		return nil, nil
	}

	_ = HandleFn(d, fn)

	err := HandleFn(d, fn)
	if !errors.Is(err, ErrHandlerExists) {
		t.Errorf("expected ErrHandlerExists, got %v", err)
	}
}

func TestOnResult_Success(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ createCmd) (Result, error) {
		return createResult{
			BaseResult: BaseResult{Name: "created"},
			ID:         "42",
		}, nil
	})

	var gotID string
	err := OnResultFn[createResult](d, "create",
		func(_ context.Context, r createResult, err error) error {
			if err != nil {
				return err
			}
			gotID = r.ID
			return nil
		})

	if err != nil {
		t.Fatalf("register result handler error: %v", err)
	}

	_ = d.Send(context.Background(), createCmd{ID: "42"})

	if gotID != "42" {
		t.Errorf("expected ID '42', got %q", gotID)
	}
}

func TestOnResult_DuplicateRegistration(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	fn := func(_ context.Context, _ createResult, _ error) error { return nil }

	_ = OnResultFn[createResult](d, "create", fn)

	err := OnResultFn[createResult](d, "create", fn)
	if !errors.Is(err, ErrResultHandlerExists) {
		t.Errorf("expected ErrResultHandlerExists, got %v", err)
	}
}

func TestOnResult_WithError(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	handlerErr := errors.New("create failed")
	_ = HandleFn(d, func(_ context.Context, _ createCmd) (Result, error) {
		return nil, handlerErr
	})

	var gotErr error
	_ = OnResultFn[createResult](d, "create",
		func(_ context.Context, _ createResult, err error) error {
			gotErr = err
			return nil
		})

	_ = d.Send(context.Background(), createCmd{ID: "1"})

	if !errors.Is(gotErr, handlerErr) {
		t.Errorf("expected handler error, got %v", gotErr)
	}
}

func TestOnResult_Struct(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ createCmd) (Result, error) {
		return createResult{
			BaseResult: BaseResult{Name: "created"},
			ID:         "99",
		}, nil
	})

	rh := &createResultHandler{}
	err := OnResult[createResult](d, "create", rh)

	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	_ = d.Send(context.Background(), createCmd{ID: "99"})

	if !rh.called {
		t.Error("expected result handler to be called")
	}
}

type createResultHandler struct {
	called bool
}

func (h *createResultHandler) Handle(_ context.Context, _ createResult, _ error) error {
	h.called = true
	return nil
}

func TestOnResultFunc_TypeMismatch(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ createCmd) (Result, error) {
		return BaseResult{Name: "base"}, nil
	})

	var errCaught bool
	eh := &testErrorHandler{onError: func(_ ErrorContext) { errCaught = true }}

	d.opts.errorHandler = eh

	_ = OnResultFn[createResult](d, "create",
		func(_ context.Context, _ createResult, _ error) error {
			return nil
		})

	_ = d.Send(context.Background(), createCmd{ID: "1"})

	if !errCaught {
		t.Error("expected type mismatch to be caught by error handler")
	}
}
