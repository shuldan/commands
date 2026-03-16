package commands

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func decodeFuncOK(_ ReplyEnvelope) (Result, error) {
	return testResult{Value: "ok"}, nil
}

func decodeFuncError(_ ReplyEnvelope) (Result, error) {
	return nil, fmt.Errorf("decode failed")
}

func TestFuture_Await_Success(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	go func() {
		f.complete(ReplyEnvelope{Payload: []byte(`{}`)})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := f.Await(ctx)
	assertError(t, err, false)
	r, ok := result.(testResult)
	if !ok {
		t.Fatal("expected testResult type")
	}
	assertEqual(t, r.Value, "ok")
}

func TestFuture_Await_ErrorReply(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	go func() {
		f.complete(ReplyEnvelope{
			Error: &ErrorPayload{Code: "BIZ", Message: "fail"},
		})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := f.Await(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	assertEqual(t, err.Error(), "BIZ: fail")
}

func TestFuture_Await_DecodeError(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncError)
	go func() {
		f.complete(ReplyEnvelope{Payload: []byte(`{}`)})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := f.Await(ctx)
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestFuture_Await_ContextCancelled(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.Await(ctx)
	assertErrorIs(t, err, context.Canceled)
}

func TestFuture_CompleteWithError(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	f.completeWithError(ErrTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := f.Await(ctx)
	assertErrorIs(t, err, ErrTimeout)
}

func TestFuture_Done_ClosedAfterComplete(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)

	select {
	case <-f.Done():
		t.Fatal("done should not be closed yet")
	default:
	}

	f.complete(ReplyEnvelope{})

	select {
	case <-f.Done():
	case <-time.After(time.Second):
		t.Fatal("done should be closed after complete")
	}
}

func TestFuture_Result_NotReady(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	_, _, ok := f.Result()
	if ok {
		t.Fatal("expected not ready")
	}
}

func TestFuture_Result_Ready(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	f.complete(ReplyEnvelope{Payload: []byte(`{}`)})

	result, err, ok := f.Result()
	if !ok {
		t.Fatal("expected ready")
	}
	assertError(t, err, false)
	assertNotNil(t, result, "result")
}

func TestFuture_DoubleComplete(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	f.complete(ReplyEnvelope{Payload: []byte(`{}`)})
	f.completeWithError(ErrTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := f.Await(ctx)
	assertError(t, err, false)
}

func TestTypedFuture_Await_Success(t *testing.T) {
	t.Parallel()

	f := newFuture(func(_ ReplyEnvelope) (Result, error) {
		return testResult{Value: "typed"}, nil
	})
	tf := &typedFuture[testResult]{inner: f}

	go func() { f.complete(ReplyEnvelope{Payload: []byte(`{}`)}) }()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	result, err := tf.Await(ctx)
	assertError(t, err, false)
	assertEqual(t, result.Value, "typed")
}

func TestTypedFuture_Await_WrongType(t *testing.T) {
	t.Parallel()

	type otherResult struct{ X int }

	f := newFuture(func(_ ReplyEnvelope) (Result, error) {
		return testResult{Value: "wrong"}, nil
	})
	tf := &typedFuture[*wrongResult]{inner: f}

	f.complete(ReplyEnvelope{Payload: []byte(`{}`)})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := tf.Await(ctx)
	if err == nil {
		t.Fatal("expected type mismatch error")
	}
}

func TestTypedFuture_Await_Error(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	tf := &typedFuture[testResult]{inner: f}
	f.completeWithError(ErrTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := tf.Await(ctx)
	assertErrorIs(t, err, ErrTimeout)
}

func TestTypedFuture_Result_NotReady(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	tf := &typedFuture[testResult]{inner: f}

	_, _, ok := tf.Result()
	if ok {
		t.Fatal("expected not ready")
	}
}

func TestTypedFuture_Result_Ready(t *testing.T) {
	t.Parallel()

	f := newFuture(func(_ ReplyEnvelope) (Result, error) {
		return testResult{Value: "ready"}, nil
	})
	tf := &typedFuture[testResult]{inner: f}
	f.complete(ReplyEnvelope{})

	result, err, ok := tf.Result()
	if !ok {
		t.Fatal("expected ready")
	}
	assertError(t, err, false)
	assertEqual(t, result.Value, "ready")
}

func TestTypedFuture_Result_Error(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	tf := &typedFuture[testResult]{inner: f}
	f.completeWithError(ErrTimeout)

	_, err, ok := tf.Result()
	if !ok {
		t.Fatal("expected ready")
	}
	assertErrorIs(t, err, ErrTimeout)
}

func TestTypedFuture_Result_WrongType(t *testing.T) {
	t.Parallel()

	f := newFuture(func(_ ReplyEnvelope) (Result, error) {
		return testResult{Value: "x"}, nil
	})
	tf := &typedFuture[*wrongResult]{inner: f}
	f.complete(ReplyEnvelope{})

	_, err, ok := tf.Result()
	if !ok {
		t.Fatal("expected ready")
	}
	if err == nil {
		t.Fatal("expected type error")
	}
}

func TestTypedFuture_Done(t *testing.T) {
	t.Parallel()

	f := newFuture(decodeFuncOK)
	tf := &typedFuture[testResult]{inner: f}

	select {
	case <-tf.Done():
		t.Fatal("should not be done")
	default:
	}

	f.complete(ReplyEnvelope{})

	select {
	case <-tf.Done():
	case <-time.After(time.Second):
		t.Fatal("should be done")
	}
}
