package commands

import (
	"context"
	"fmt"
	"testing"
)

func TestReplySender_Send_Success(t *testing.T) {
	t.Parallel()

	var captured ReplyEnvelope
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		captured = env
		return nil
	}
	codec := &testCodec{
		encodeFunc: func(v any) ([]byte, error) { return []byte(`"ok"`), nil },
	}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1", ReplyTo: "r1"})
	err := rs.Send(context.Background(), testResult{Value: "ok"})

	assertError(t, err, false)
	assertEqual(t, captured.CorrelationID, "c1")
	assertEqual(t, captured.ResultName, "test.result")
	assertEqual(t, string(captured.Payload), `"ok"`)
}

func TestReplySender_Send_EncodeError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{
		encodeFunc: func(v any) ([]byte, error) { return nil, fmt.Errorf("encode fail") },
	}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.Send(context.Background(), testResult{})

	assertError(t, err, true)
}

func TestReplySender_Send_TransportError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, _ ReplyEnvelope) error {
		return fmt.Errorf("transport fail")
	}
	codec := &testCodec{}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.Send(context.Background(), testResult{})

	assertError(t, err, true)
}

func TestReplySender_Send_CalledTwice(t *testing.T) {
	t.Parallel()

	var replyCount int
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, _ ReplyEnvelope) error {
		replyCount++
		return nil
	}
	codec := &testCodec{}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.Send(context.Background(), testResult{})
	assertError(t, err, false)

	err = rs.Send(context.Background(), testResult{})
	assertError(t, err, false)

	assertEqual(t, replyCount, 2)
}

func TestReplySender_SendError_Success(t *testing.T) {
	t.Parallel()

	var captured ReplyEnvelope
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		captured = env
		return nil
	}
	codec := &testCodec{}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.SendError(context.Background(), &ErrorPayload{Code: "BIZ", Message: "nope"})

	assertError(t, err, false)
	assertEqual(t, captured.Error.Code, "BIZ")
	assertEqual(t, captured.Error.Message, "nope")
}

func TestReplySender_SendError_NonErrorPayload(t *testing.T) {
	t.Parallel()

	var captured ReplyEnvelope
	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, env ReplyEnvelope) error {
		captured = env
		return nil
	}
	codec := &testCodec{}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.SendError(context.Background(), fmt.Errorf("plain error"))

	assertError(t, err, false)
	assertEqual(t, captured.Error.Code, "UNKNOWN")
	assertEqual(t, captured.Error.Message, "plain error")
}

func TestReplySender_SendError_TransportError(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	tr.replyFunc = func(_ context.Context, _ ReplyEnvelope) error {
		return fmt.Errorf("transport fail")
	}
	codec := &testCodec{}

	rs := NewReplySender(tr, codec, ReplyAddress{CorrelationID: "c1"})
	err := rs.SendError(context.Background(), fmt.Errorf("biz err"))

	assertError(t, err, true)
}

func TestReplySender_Address(t *testing.T) {
	t.Parallel()

	tr := newTestTransport()
	codec := &testCodec{}
	addr := ReplyAddress{CorrelationID: "c1", ReplyTo: "r1"}

	rs := NewReplySender(tr, codec, addr)
	got := rs.Address()

	assertEqual(t, got.CorrelationID, "c1")
	assertEqual(t, got.ReplyTo, "r1")
}
