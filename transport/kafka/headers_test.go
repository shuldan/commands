package kafka

import (
	"encoding/json"
	"reflect"
	"testing"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/commands"
)

func TestCommandEnvelopeToMessage_Basic(t *testing.T) {
	t.Parallel()
	env := commands.CommandEnvelope{
		MessageID:     "msg-1",
		CorrelationID: "corr-1",
		CommandName:   "DoSomething",
		ReplyTo:       "reply-topic",
		Payload:       []byte(`{"key":"value"}`),
	}
	msg := commandEnvelopeToMessage("commands", env)

	if msg.Topic != "commands" {
		t.Errorf("expected topic=commands, got %s", msg.Topic)
	}
	if string(msg.Value) != `{"key":"value"}` {
		t.Errorf("unexpected value: %s", string(msg.Value))
	}
	assertHeader(t, msg.Headers, headerMessageID, "msg-1")
	assertHeader(t, msg.Headers, headerCorrelationID, "corr-1")
	assertHeader(t, msg.Headers, headerCommandName, "DoSomething")
	assertHeader(t, msg.Headers, headerReplyTo, "reply-topic")
}

func TestCommandEnvelopeToMessage_WithCustomHeaders(t *testing.T) {
	t.Parallel()
	env := commands.CommandEnvelope{
		MessageID:     "msg-1",
		CorrelationID: "corr-1",
		CommandName:   "Cmd",
		ReplyTo:       "reply",
		Headers:       map[string]string{"tenant": "abc"},
		Payload:       []byte("p"),
	}
	msg := commandEnvelopeToMessage("t", env)
	assertHeader(t, msg.Headers, "x-tenant", "abc")
}

func TestMessageToCommandEnvelope_Basic(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("payload"),
		Headers: []kafkago.Header{
			{Key: headerMessageID, Value: []byte("m1")},
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerCommandName, Value: []byte("Cmd")},
			{Key: headerReplyTo, Value: []byte("rt")},
		},
	}
	env := messageToCommandEnvelope(msg)

	if env.MessageID != "m1" {
		t.Errorf("expected MessageID=m1, got %s", env.MessageID)
	}
	if env.CorrelationID != "c1" {
		t.Errorf("expected CorrelationID=c1, got %s", env.CorrelationID)
	}
	if env.CommandName != "Cmd" {
		t.Errorf("expected CommandName=Cmd, got %s", env.CommandName)
	}
	if env.ReplyTo != "rt" {
		t.Errorf("expected ReplyTo=rt, got %s", env.ReplyTo)
	}
	if string(env.Payload) != "payload" {
		t.Errorf("unexpected payload: %s", string(env.Payload))
	}
}

func TestMessageToCommandEnvelope_CustomHeaders(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: "x-foo", Value: []byte("bar")},
			{Key: "unknown", Value: []byte("ignored")},
		},
	}
	env := messageToCommandEnvelope(msg)

	if env.Headers == nil || env.Headers["foo"] != "bar" {
		t.Errorf("expected custom header foo=bar, got %v", env.Headers)
	}
	if _, ok := env.Headers["unknown"]; ok {
		t.Error("non-prefixed header should not appear in custom headers")
	}
}

func TestMessageToCommandEnvelope_NoCustomHeaders(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: headerMessageID, Value: []byte("m1")},
		},
	}
	env := messageToCommandEnvelope(msg)
	if env.Headers != nil {
		t.Errorf("expected nil headers, got %v", env.Headers)
	}
}

func TestReplyEnvelopeToMessage_Success(t *testing.T) {
	t.Parallel()
	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		ResultName:    "Result",
		Payload:       []byte("data"),
	}
	msg, err := replyEnvelopeToMessage("replies", env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Topic != "replies" {
		t.Errorf("expected topic=replies, got %s", msg.Topic)
	}
	assertHeader(t, msg.Headers, headerCorrelationID, "c1")
	assertHeader(t, msg.Headers, headerResultName, "Result")
	if string(msg.Value) != "data" {
		t.Errorf("unexpected value: %s", string(msg.Value))
	}
}

func TestReplyEnvelopeToMessage_WithError(t *testing.T) {
	t.Parallel()
	ep := &commands.ErrorPayload{Code: "ERR", Message: "fail"}
	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		Error:         ep,
	}
	msg, err := replyEnvelopeToMessage("replies", env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var found bool
	for _, h := range msg.Headers {
		if h.Key == headerError {
			found = true
			var decoded commands.ErrorPayload
			if err := json.Unmarshal(h.Value, &decoded); err != nil {
				t.Fatalf("unmarshal error header: %v", err)
			}
			if decoded.Code != "ERR" || decoded.Message != "fail" {
				t.Errorf("unexpected error payload: %+v", decoded)
			}
		}
	}
	if !found {
		t.Error("error header not found")
	}
}

func TestReplyEnvelopeToMessage_NoResultName(t *testing.T) {
	t.Parallel()
	env := commands.ReplyEnvelope{CorrelationID: "c1"}
	msg, err := replyEnvelopeToMessage("replies", env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, h := range msg.Headers {
		if h.Key == headerResultName {
			t.Error("result_name header should not be present when empty")
		}
	}
}

func TestReplyEnvelopeToMessage_CustomHeaders(t *testing.T) {
	t.Parallel()
	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		Headers:       map[string]string{"trace": "123"},
	}
	msg, err := replyEnvelopeToMessage("replies", env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertHeader(t, msg.Headers, "x-trace", "123")
}

func TestMessageToReplyEnvelope_Success(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("result"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerResultName, Value: []byte("Res")},
		},
	}
	env, err := messageToReplyEnvelope(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.CorrelationID != "c1" {
		t.Errorf("expected CorrelationID=c1, got %s", env.CorrelationID)
	}
	if env.ResultName != "Res" {
		t.Errorf("expected ResultName=Res, got %s", env.ResultName)
	}
	if string(env.Payload) != "result" {
		t.Errorf("unexpected payload: %s", string(env.Payload))
	}
}

func TestMessageToReplyEnvelope_WithError(t *testing.T) {
	t.Parallel()
	ep := commands.ErrorPayload{Code: "BAD", Message: "bad"}
	errBytes, _ := json.Marshal(ep)
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerError, Value: errBytes},
		},
	}
	env, err := messageToReplyEnvelope(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Error == nil {
		t.Fatal("expected error payload")
	}
	if env.Error.Code != "BAD" {
		t.Errorf("expected code=BAD, got %s", env.Error.Code)
	}
}

func TestMessageToReplyEnvelope_MalformedError(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerError, Value: []byte("not-json")},
		},
	}
	_, err := messageToReplyEnvelope(msg)
	if err == nil {
		t.Error("expected error for malformed error header")
	}
}

func TestMessageToReplyEnvelope_CustomHeaders(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: "x-region", Value: []byte("us")},
		},
	}
	env, err := messageToReplyEnvelope(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Headers == nil || env.Headers["region"] != "us" {
		t.Errorf("expected custom header region=us, got %v", env.Headers)
	}
}

func TestRoundTrip_CommandEnvelope(t *testing.T) {
	t.Parallel()
	original := commands.CommandEnvelope{
		MessageID:     "msg-1",
		CorrelationID: "corr-1",
		CommandName:   "Cmd",
		ReplyTo:       "reply",
		Headers:       map[string]string{"k": "v"},
		Payload:       []byte("payload"),
	}
	msg := commandEnvelopeToMessage("topic", original)
	restored := messageToCommandEnvelope(msg)

	if !reflect.DeepEqual(original, restored) {
		t.Errorf("round trip failed:\noriginal: %+v\nrestored: %+v", original, restored)
	}
}

func TestRoundTrip_ReplyEnvelope(t *testing.T) {
	t.Parallel()
	original := commands.ReplyEnvelope{
		CorrelationID: "c1",
		ResultName:    "Res",
		Headers:       map[string]string{"k": "v"},
		Payload:       []byte("data"),
	}
	msg, err := replyEnvelopeToMessage("topic", original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	restored, err := messageToReplyEnvelope(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Errorf("round trip failed:\noriginal: %+v\nrestored: %+v", original, restored)
	}
}

func TestRoundTrip_ReplyEnvelopeWithError(t *testing.T) {
	t.Parallel()
	original := commands.ReplyEnvelope{
		CorrelationID: "c1",
		Error:         &commands.ErrorPayload{Code: "E", Message: "m"},
	}
	msg, err := replyEnvelopeToMessage("topic", original)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	restored, err := messageToReplyEnvelope(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Errorf("round trip failed:\noriginal: %+v\nrestored: %+v", original, restored)
	}
}

func assertHeader(t *testing.T, headers []kafkago.Header, key, expected string) {
	t.Helper()
	for _, h := range headers {
		if h.Key == key {
			if string(h.Value) != expected {
				t.Errorf("header %s: expected %s, got %s", key, expected, string(h.Value))
			}
			return
		}
	}
	t.Errorf("header %s not found", key)
}
