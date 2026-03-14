package commands

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSend_ConcurrentSend(t *testing.T) {
	t.Parallel()

	d := New(WithAsyncMode(), WithWorkerCount(4))

	var count int32
	var wg sync.WaitGroup

	_ = HandleFunc2(d, func(_ context.Context, _ testCommand) (Result, error) {
		atomic.AddInt32(&count, 1)
		wg.Done()
		return nil, nil
	})

	n := 100
	wg.Add(n)

	for i := range n {
		go func(i int) {
			key := fmt.Sprintf("conc-%d", i)
			_ = d.Send(context.Background(), newTestCommand("", key))
		}(i)
	}

	wg.Wait()
	_ = d.Close(context.Background())

	got := atomic.LoadInt32(&count)
	if got != int32(n) {
		t.Errorf("expected %d handled, got %d", n, got)
	}
}

func TestSend_ConcurrentRegisterAndSend(t *testing.T) {
	t.Parallel()

	d := New(WithSyncMode())
	defer d.Close(context.Background())

	_ = HandleFunc2(d, func(_ context.Context, _ testCommand) (Result, error) {
		return nil, nil
	})

	var wg sync.WaitGroup

	for i := range 50 {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("par-%d", i)
			_ = d.Send(context.Background(), newTestCommand("", key))
		}(i)
	}

	wg.Wait()
}
