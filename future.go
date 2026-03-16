package commands

import (
	"context"
	"sync"
)

// Future represents a pending result of a command.
type Future interface {
	Await(ctx context.Context) (Result, error)
	Done() <-chan struct{}
	Result() (Result, error, bool)
}

// TypedFuture is a type-safe version of Future.
type TypedFuture[R Result] interface {
	Await(ctx context.Context) (R, error)
	Done() <-chan struct{}
	Result() (R, error, bool)
}

// future is the internal implementation of Future.
type future struct {
	done   chan struct{}
	once   sync.Once
	result Result
	err    error

	// decode converts raw reply payload into a Result.
	decode func(env ReplyEnvelope) (Result, error)
}

func newFuture(decode func(env ReplyEnvelope) (Result, error)) *future {
	return &future{
		done:   make(chan struct{}),
		decode: decode,
	}
}

func (f *future) complete(env ReplyEnvelope) {
	f.once.Do(func() {
		if env.Error != nil {
			f.err = env.Error
		} else {
			f.result, f.err = f.decode(env)
		}
		close(f.done)
	})
}

func (f *future) completeWithError(err error) {
	f.once.Do(func() {
		f.err = err
		close(f.done)
	})
}

// Await blocks until the result is available or the context is cancelled.
func (f *future) Await(ctx context.Context) (Result, error) {
	select {
	case <-f.done:
		return f.result, f.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Done returns a channel that is closed when the result is available.
func (f *future) Done() <-chan struct{} {
	return f.done
}

// Result returns the result if available. The third return value indicates
// whether the result is ready.
func (f *future) Result() (Result, error, bool) {
	select {
	case <-f.done:
		return f.result, f.err, true
	default:
		return nil, nil, false
	}
}

// typedFuture wraps a future and provides type-safe access to the result.
type typedFuture[R Result] struct {
	inner *future
}

// Await blocks until the result is available or the context is cancelled.
func (f *typedFuture[R]) Await(ctx context.Context) (R, error) {
	var zero R
	result, err := f.inner.Await(ctx)
	if err != nil {
		return zero, err
	}
	typed, ok := result.(R)
	if !ok {
		return zero, &ErrorPayload{
			Code:    "INVALID_RESULT_TYPE",
			Message: "unexpected result type",
		}
	}
	return typed, nil
}

// Done returns a channel that is closed when the result is available.
func (f *typedFuture[R]) Done() <-chan struct{} {
	return f.inner.Done()
}

// Result returns the result if available. The third return value indicates
// whether the result is ready.
func (f *typedFuture[R]) Result() (R, error, bool) {
	var zero R
	result, err, ok := f.inner.Result()
	if !ok {
		return zero, nil, false
	}
	if err != nil {
		return zero, err, true
	}
	typed, tOk := result.(R)
	if !tOk {
		return zero, &ErrorPayload{
			Code:    "INVALID_RESULT_TYPE",
			Message: "unexpected result type",
		}, true
	}
	return typed, nil, true
}
