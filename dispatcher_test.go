package commands

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	d := New()
	defer d.Close(context.Background())

	if d.closed {
		t.Error("expected dispatcher to not be closed")
	}

	if d.handlers == nil {
		t.Error("expected non-nil handlers map")
	}
}

func TestNew_SyncMode(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	if d.opts.asyncMode {
		t.Error("expected sync mode")
	}

	if d.sharedChan != nil {
		t.Error("expected no shared channel in sync mode")
	}
}

func TestNew_AsyncShared(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(2))
	defer d.Close(context.Background())

	if d.sharedChan == nil {
		t.Error("expected shared channel in async mode")
	}
}

func TestNew_AsyncOrdered(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(3), WithOrderedDelivery())
	defer d.Close(context.Background())

	if len(d.workerChans) != 3 {
		t.Errorf("expected 3 worker channels, got %d", len(d.workerChans))
	}
}

func TestSend_NilCommand(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	err := d.Send(context.Background(), nil)
	if !errors.Is(err, ErrNilCommand) {
		t.Errorf("expected ErrNilCommand, got %v", err)
	}
}

func TestSend_ClosedDispatcher(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	_ = d.Close(context.Background())

	err := d.Send(context.Background(), newTestCommand("test", "k"))
	if !errors.Is(err, ErrDispatcherClosed) {
		t.Errorf("expected ErrDispatcherClosed, got %v", err)
	}
}

func TestSend_HandlerNotFound(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	err := d.Send(context.Background(), newTestCommand("unknown", "k"))
	if !errors.Is(err, ErrHandlerNotFound) {
		t.Errorf("expected ErrHandlerNotFound, got %v", err)
	}
}

func TestSend_SyncSuccess(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	var called bool
	err := HandleFn(d, func(_ context.Context, c testCommand) (Result, error) {
		called = true
		return BaseResult{Name: "ok"}, nil
	})

	if err != nil {
		t.Fatalf("register error: %v", err)
	}

	err = d.Send(context.Background(), newTestCommand("", "key1"))
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	if !called {
		t.Error("expected handler to be called")
	}
}

func TestSend_AsyncSuccess(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(2))

	var wg sync.WaitGroup
	wg.Add(1)

	_ = HandleFn(d, func(_ context.Context, c testCommand) (Result, error) {
		wg.Done()
		return nil, nil
	})

	err := d.Send(context.Background(), newTestCommand("", "key1"))
	if err != nil {
		t.Fatalf("send error: %v", err)
	}

	wg.Wait()
	_ = d.Close(context.Background())
}

func TestSend_AsyncOrdered(t *testing.T) {
	t.Parallel()

	d := New(
		WithAsyncMode(),
		WithWorkerCount(4),
		WithOrderedDelivery(),
	)

	var mu sync.Mutex
	var order []string
	var wg sync.WaitGroup

	_ = HandleFn(d, func(_ context.Context, c testCommand) (Result, error) {
		mu.Lock()
		order = append(order, c.BaseCommand.IdemKey)
		mu.Unlock()
		wg.Done()
		return nil, nil
	})

	wg.Add(3)
	_ = d.Send(context.Background(), newTestCommand("", "ordered-1"))
	_ = d.Send(context.Background(), newTestCommand("", "ordered-2"))
	_ = d.Send(context.Background(), newTestCommand("", "ordered-3"))

	wg.Wait()
	_ = d.Close(context.Background())
}

func TestSend_IdempotencyDuplicate(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	var count int32
	_ = HandleFn(d, func(_ context.Context, c testCommand) (Result, error) {
		atomic.AddInt32(&count, 1)
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "dup-key"))
	_ = d.Send(context.Background(), newTestCommand("", "dup-key"))

	if atomic.LoadInt32(&count) != 1 {
		t.Errorf("expected handler called once, got %d", count)
	}
}

func TestSend_HandlerPanic(t *testing.T) {
	t.Parallel()

	var panicCaught bool
	ph := &testPanicHandler{onPanic: func(_ PanicContext) { panicCaught = true }}

	d := New(WithSyncMode(), WithPanicHandler(ph))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		panic("boom")
	})

	_ = d.Send(context.Background(), newTestCommand("", "panic-key"))

	if !panicCaught {
		t.Error("expected panic to be caught")
	}
}

func TestSend_HandlerError(t *testing.T) {
	t.Parallel()

	var errCaught bool
	eh := &testErrorHandler{onError: func(_ ErrorContext) { errCaught = true }}

	d := New(WithSyncMode(), WithErrorHandler(eh))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, errors.New("fail")
	})

	_ = d.Send(context.Background(), newTestCommand("", "err-key"))

	if !errCaught {
		t.Error("expected error handler to be called")
	}
}

func TestClose_IdempotentCalls(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())

	err1 := d.Close(context.Background())
	err2 := d.Close(context.Background())

	if err1 != nil {
		t.Errorf("first Close error: %v", err1)
	}

	if err2 != nil {
		t.Errorf("second Close error: %v", err2)
	}
}

func TestClose_AsyncGraceful(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(2))

	var count int32
	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		time.Sleep(10 * time.Millisecond)
		atomic.AddInt32(&count, 1)
		return nil, nil
	})

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key-%d", i)
		_ = d.Send(context.Background(), newTestCommand("", key))
	}

	err := d.Close(context.Background())
	if err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestClose_ContextTimeout(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})

	d := New(WithAsyncMode(), WithWorkerCount(1), WithBufferSize(1))

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		<-blocker
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "slow"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := d.Close(ctx)
	if err == nil {
		t.Error("expected context deadline error")
	}

	close(blocker)
}

func TestSend_MetricsDispatched(t *testing.T) {
	t.Parallel()

	m := &recordingMetrics{}
	d := New(WithSyncMode(), WithMetrics(m))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "met-key"))

	if m.dispatched != 1 {
		t.Errorf("expected 1 dispatched, got %d", m.dispatched)
	}
}

func TestSend_ResultDelivery(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return BaseResult{Name: "created"}, nil
	})

	var gotResult Result
	_ = OnResultFn[BaseResult](d, "", func(_ context.Context, r BaseResult, _ error) error {
		gotResult = r
		return nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "res-key"))

	if gotResult == nil {
		t.Fatal("expected result to be delivered")
	}

	if gotResult.ResultName() != "created" {
		t.Errorf("expected 'created', got %q", gotResult.ResultName())
	}
}

func TestSend_ResultDelivery_Error(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	handlerErr := errors.New("handler fail")
	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, handlerErr
	})

	var gotErr error
	_ = OnResultFn[BaseResult](d, "", func(_ context.Context, _ BaseResult, err error) error {
		gotErr = err
		return nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "res-err-key"))

	if !errors.Is(gotErr, handlerErr) {
		t.Errorf("expected handler error, got %v", gotErr)
	}
}

func TestSend_ContextCancelled_Async(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})

	d := New(
		WithAsyncMode(),
		WithWorkerCount(1),
		WithBufferSize(1),
	)

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		<-blocker
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "fill-1"))
	_ = d.Send(context.Background(), newTestCommand("", "fill-2"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := d.Send(ctx, newTestCommand("", "cancelled"))
	if err == nil {
		t.Error("expected error for cancelled context")
	}

	close(blocker)
	_ = d.Close(context.Background())
}

type testPanicHandler struct {
	onPanic func(PanicContext)
}

func (h *testPanicHandler) Handle(ctx PanicContext) {
	if h.onPanic != nil {
		h.onPanic(ctx)
	}
}

type testErrorHandler struct {
	mu      sync.Mutex
	onError func(ErrorContext)
}

func (h *testErrorHandler) Handle(ctx ErrorContext) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.onError != nil {
		h.onError(ctx)
	}
}
