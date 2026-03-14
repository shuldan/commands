package commands

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestSend_IdempotencyStoreError(t *testing.T) {
	t.Parallel()

	store := &failingIdempotencyStore{
		existsErr: errors.New("store error"),
	}

	var errCaught bool
	eh := &testErrorHandler{onError: func(_ ErrorContext) { errCaught = true }}

	d := New(
		WithSyncMode(),
		WithIdempotencyStore(store),
		WithErrorHandler(eh),
	)
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "store-err"))

	if !errCaught {
		t.Error("expected error handler to catch store error")
	}
}

func TestSend_IdempotencyMarkError(t *testing.T) {
	t.Parallel()

	store := &failingIdempotencyStore{
		markErr: errors.New("mark error"),
	}

	var errCaught bool
	eh := &testErrorHandler{onError: func(ctx ErrorContext) {
		if ctx.Err != nil {
			errCaught = true
		}
	}}

	d := New(
		WithSyncMode(),
		WithIdempotencyStore(store),
		WithErrorHandler(eh),
	)
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "mark-err"))

	if !errCaught {
		t.Error("expected error handler to catch mark error")
	}
}

func TestPartitionIndex_EmptyKey(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	idx := d.partitionIndex("")
	if idx != 0 {
		t.Errorf("expected 0 for empty key, got %d", idx)
	}
}

func TestPartitionIndex_Deterministic(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode(), WithWorkerCount(8))
	defer d.Close(context.Background())

	idx1 := d.partitionIndex("test-key")
	idx2 := d.partitionIndex("test-key")

	if idx1 != idx2 {
		t.Errorf("expected same index, got %d and %d", idx1, idx2)
	}
}

func TestPartitionIndex_DifferentKeys(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode(), WithWorkerCount(1000))
	defer d.Close(context.Background())

	indices := make(map[int]bool)
	for i := 0; i < 100; i++ {
		idx := d.partitionIndex(fmt.Sprintf("key-%d", i))
		indices[idx] = true
	}

	if len(indices) < 2 {
		t.Error("expected different keys to map to different partitions")
	}
}

func TestSend_ResultHandlerError(t *testing.T) {
	t.Parallel()

	var errCaught bool
	eh := &testErrorHandler{onError: func(_ ErrorContext) { errCaught = true }}

	d := New(WithSyncMode(), WithErrorHandler(eh))
	defer d.Close(context.Background())

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return BaseResult{Name: "ok"}, nil
	})

	_ = OnResultFunc[BaseResult](d, "", func(_ context.Context, _ BaseResult, _ error) error {
		return errors.New("result handler failed")
	})

	_ = d.Send(context.Background(), newTestCommand("", "rh-err"))

	if !errCaught {
		t.Error("expected error handler for result handler error")
	}
}

func TestSend_BufferSize_DefaultCalculation(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(4))
	defer d.Close(context.Background())

	if d.opts.bufferSize != 40 {
		t.Errorf("expected bufferSize 40, got %d", d.opts.bufferSize)
	}
}

func TestSend_AsyncTimeout(t *testing.T) {
	t.Parallel()

	blocker := make(chan struct{})

	d := New(
		WithAsyncMode(),
		WithWorkerCount(1),
		WithBufferSize(1),
		WithPublishTimeout(10*time.Millisecond),
	)

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		<-blocker
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "fill-buf-1"))
	_ = d.Send(context.Background(), newTestCommand("", "fill-buf-2"))

	err := d.Send(context.Background(), newTestCommand("", "timeout-key"))
	if err == nil {
		t.Error("expected timeout error")
	}

	close(blocker)
	_ = d.Close(context.Background())
}

func TestClose_AsyncOrdered(t *testing.T) {
	t.Parallel()

	d := New(
		WithAsyncMode(),
		WithWorkerCount(3),
		WithOrderedDelivery(),
	)

	_ = HandleFn(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, nil
	})

	_ = d.Send(context.Background(), newTestCommand("", "ord-1"))
	_ = d.Send(context.Background(), newTestCommand("", "ord-2"))

	err := d.Close(context.Background())
	if err != nil {
		t.Errorf("Close error: %v", err)
	}
}

type failingIdempotencyStore struct {
	existsErr error
	markErr   error
}

func (f *failingIdempotencyStore) Exists(_ context.Context, _ string) (bool, error) {
	if f.existsErr != nil {
		return false, f.existsErr
	}

	return false, nil
}

func (f *failingIdempotencyStore) Mark(_ context.Context, _ string, _ time.Duration) error {
	return f.markErr
}
