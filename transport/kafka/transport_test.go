package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/commands"
)

// ── New / newWithDialer ──────────────────────────────────────────────

func TestNew_ValidationError(t *testing.T) {
	t.Parallel()
	_, err := newWithDialer(Config{}, &mockDialer{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNew_AutoCreateTopics_Success(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.AutoCreateTopics = true

	tr, err := newWithDialer(cfg, &mockDialer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestNew_AutoCreateTopics_Error(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.AutoCreateTopics = true

	_, err := newWithDialer(cfg, &mockDialer{ensureErr: fmt.Errorf("cannot create")})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ensure topics") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNew_CheckTopics_Success(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.AutoCreateTopics = false

	tr, err := newWithDialer(cfg, &mockDialer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestNew_CheckTopics_Error(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.AutoCreateTopics = false

	_, err := newWithDialer(cfg, &mockDialer{checkErr: fmt.Errorf("topic not found")})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNew_AppliesDefaults(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Brokers:      []string{"localhost:9092"},
		CommandTopic: "cmd",
		ReplyTopic:   "reply",
	}
	tr, err := newWithDialer(cfg, &mockDialer{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.cfg.NumPartitions != 1 {
		t.Errorf("expected NumPartitions=1, got %d", tr.cfg.NumPartitions)
	}
	if tr.cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout=10s, got %v", tr.cfg.WriteTimeout)
	}
}

func TestNewTransport_WithNilDeps(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.withDefaults()

	// Calling with nil deps should not panic — it creates real kafka objects.
	// We just verify the transport is constructed and has non-nil fields.
	tr := newTransport(cfg, nil, nil, nil)
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
	if tr.writer == nil {
		t.Error("expected non-nil writer")
	}
	if tr.newCommandReader == nil {
		t.Error("expected non-nil newCommandReader")
	}
	if tr.newReplyReader == nil {
		t.Error("expected non-nil newReplyReader")
	}
}

// ── Send ─────────────────────────────────────────────────────────────

func TestSend_WhenClosed(t *testing.T) {
	t.Parallel()
	w := &mockWriter{}
	tr := newTestTransport(testConfig(), w, nil, nil)
	tr.closed = true

	err := tr.Send(context.Background(), commands.CommandEnvelope{})
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestSend_WriterError(t *testing.T) {
	t.Parallel()
	w := &mockWriter{err: fmt.Errorf("write failed")}
	tr := newTestTransport(testConfig(), w, nil, nil)

	err := tr.Send(context.Background(), commands.CommandEnvelope{})
	if err == nil || err.Error() != "write failed" {
		t.Errorf("expected write failed error, got %v", err)
	}
}

func TestSend_Success(t *testing.T) {
	t.Parallel()
	w := &mockWriter{}
	tr := newTestTransport(testConfig(), w, nil, nil)

	env := commands.CommandEnvelope{
		MessageID:     "m1",
		CorrelationID: "c1",
		CommandName:   "Cmd",
		ReplyTo:       "reply",
		Payload:       []byte("data"),
	}
	err := tr.Send(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(w.messages))
	}
	if w.messages[0].Topic != "test-commands" {
		t.Errorf("expected topic=test-commands, got %s", w.messages[0].Topic)
	}
}

// ── Reply ────────────────────────────────────────────────────────────

func TestReply_WhenClosed(t *testing.T) {
	t.Parallel()
	w := &mockWriter{}
	tr := newTestTransport(testConfig(), w, nil, nil)
	tr.closed = true

	err := tr.Reply(context.Background(), commands.ReplyEnvelope{})
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestReply_Success(t *testing.T) {
	t.Parallel()
	w := &mockWriter{}
	tr := newTestTransport(testConfig(), w, nil, nil)

	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		ResultName:    "Res",
		Payload:       []byte("result"),
	}
	err := tr.Reply(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(w.messages))
	}
}

func TestReply_WriterError(t *testing.T) {
	t.Parallel()
	w := &mockWriter{err: fmt.Errorf("write failed")}
	tr := newTestTransport(testConfig(), w, nil, nil)

	env := commands.ReplyEnvelope{
		CorrelationID: "c1",
		ResultName:    "Res",
		Payload:       []byte("result"),
	}
	err := tr.Reply(context.Background(), env)
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── Subscribe ────────────────────────────────────────────────────────

func TestSubscribe_WhenClosed(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(testConfig(), &mockWriter{}, newMockReader(), nil)
	tr.closed = true

	err := tr.Subscribe(context.Background(), newMockCommandHandler())
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestSubscribe_MissingConsumerGroup(t *testing.T) {
	t.Parallel()
	cfg := testConfig()
	cfg.ConsumerGroup = ""
	tr := newTestTransport(cfg, &mockWriter{}, newMockReader(), nil)

	err := tr.Subscribe(context.Background(), newMockCommandHandler())
	if !errors.Is(err, ErrMissingConsumerGroup) {
		t.Errorf("expected ErrMissingConsumerGroup, got %v", err)
	}
}

func TestSubscribe_AlreadySubscribed(t *testing.T) {
	t.Parallel()
	cmdReader := newMockReader()
	tr := newTestTransport(testConfig(), &mockWriter{}, cmdReader, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tr.Subscribe(ctx, newMockCommandHandler()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := tr.Subscribe(ctx, newMockCommandHandler())
	if !errors.Is(err, ErrCommandHandlerAlreadySubscribed) {
		t.Errorf("expected ErrCommandHandlerAlreadySubscribed, got %v", err)
	}

	cancel()
	tr.wg.Wait()
}

func TestSubscribe_DeliversCommands(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("payload"),
		Headers: []kafkago.Header{
			{Key: headerMessageID, Value: []byte("m1")},
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerCommandName, Value: []byte("Cmd")},
			{Key: headerReplyTo, Value: []byte("reply")},
		},
	}
	cmdReader := newMockReader(msg)
	handler := newMockCommandHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, cmdReader, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tr.Subscribe(ctx, handler); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case env := <-handler.ch:
		if env.CommandName != "Cmd" {
			t.Errorf("expected CommandName=Cmd, got %s", env.CommandName)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for command")
	}

	cancel()
	tr.wg.Wait()
}

// ── SubscribeReplies ─────────────────────────────────────────────────

func TestSubscribeReplies_WhenClosed(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, newMockReader())
	tr.closed = true

	err := tr.SubscribeReplies(context.Background(), newMockReplyHandler())
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestSubscribeReplies_AlreadySubscribed(t *testing.T) {
	t.Parallel()
	replyReader := newMockReader()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, replyReader)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tr.SubscribeReplies(ctx, newMockReplyHandler()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := tr.SubscribeReplies(ctx, newMockReplyHandler())
	if !errors.Is(err, ErrReplyHandlerAlreadySubscribed) {
		t.Errorf("expected ErrReplyHandlerAlreadySubscribed, got %v", err)
	}

	cancel()
	tr.wg.Wait()
}

func TestSubscribeReplies_DeliversReplies(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("result"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerResultName, Value: []byte("Res")},
		},
	}
	replyReader := newMockReader(msg)
	handler := newMockReplyHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, replyReader)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := tr.SubscribeReplies(ctx, handler); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case env := <-handler.ch:
		if env.CorrelationID != "c1" {
			t.Errorf("expected CorrelationID=c1, got %s", env.CorrelationID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for reply")
	}

	cancel()
	tr.wg.Wait()
}

// ── ReplyAddress ─────────────────────────────────────────────────────

func TestReplyAddress(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, nil)
	if tr.ReplyAddress() != "test-replies" {
		t.Errorf("expected test-replies, got %s", tr.ReplyAddress())
	}
}

// ── Close ────────────────────────────────────────────────────────────

func TestClose_WhenAlreadyClosed(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, nil)
	tr.closed = true

	err := tr.Close(context.Background())
	if !errors.Is(err, ErrTransportClosed) {
		t.Errorf("expected ErrTransportClosed, got %v", err)
	}
}

func TestClose_StopsReadLoops(t *testing.T) {
	t.Parallel()
	cmdReader := newMockReader()
	replyReader := newMockReader()
	tr := newTestTransport(testConfig(), &mockWriter{}, cmdReader, replyReader)

	ctx := context.Background()
	if err := tr.Subscribe(ctx, newMockCommandHandler()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := tr.SubscribeReplies(ctx, newMockReplyHandler()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := tr.Close(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cmdReader.mu.Lock()
	cmdClosed := cmdReader.closed
	cmdReader.mu.Unlock()
	if !cmdClosed {
		t.Error("command reader should be closed")
	}

	replyReader.mu.Lock()
	replyClosed := replyReader.closed
	replyReader.mu.Unlock()
	if !replyClosed {
		t.Error("reply reader should be closed")
	}
}

func TestClose_ReturnsReaderErrors(t *testing.T) {
	t.Parallel()
	cmdReader := newMockReader()
	cmdReader.closeErr = fmt.Errorf("cmd close err")
	replyReader := newMockReader()
	replyReader.closeErr = fmt.Errorf("reply close err")
	tr := newTestTransport(testConfig(), &mockWriter{}, cmdReader, replyReader)

	ctx := context.Background()
	_ = tr.Subscribe(ctx, newMockCommandHandler())
	_ = tr.SubscribeReplies(ctx, newMockReplyHandler())

	err := tr.Close(ctx)
	if err == nil {
		t.Fatal("expected error from close")
	}
}

func TestClose_NoReadersNil(t *testing.T) {
	t.Parallel()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, nil)

	if err := tr.Close(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClose_WriterError(t *testing.T) {
	t.Parallel()
	w := &mockWriter{closeErr: fmt.Errorf("writer close err")}
	tr := newTestTransport(testConfig(), w, nil, nil)

	err := tr.Close(context.Background())
	if err == nil {
		t.Fatal("expected error from writer close")
	}
}

// ── readCommands edge cases ──────────────────────────────────────────

func TestReadCommands_SkipOnFetchError(t *testing.T) {
	t.Parallel()
	reader := newMockReader()
	reader.fetchErr = fmt.Errorf("transient")
	handler := newMockCommandHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, reader, nil)
	tr.commandHandler = handler
	tr.commandReader = reader

	ctx, cancel := context.WithCancel(context.Background())
	tr.wg.Add(1)
	go tr.readCommands(ctx)

	time.Sleep(150 * time.Millisecond)
	cancel()
	tr.wg.Wait()

	handler.mu.Lock()
	count := len(handler.received)
	handler.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 commands delivered, got %d", count)
	}
}

func TestReadCommands_CommitError_ContinuesProcessing(t *testing.T) {
	t.Parallel()
	msg1 := kafkago.Message{
		Value: []byte("p1"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerCommandName, Value: []byte("Cmd1")},
		},
	}
	msg2 := kafkago.Message{
		Value: []byte("p2"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c2")},
			{Key: headerCommandName, Value: []byte("Cmd2")},
		},
	}
	reader := newMockReader(msg1, msg2)
	reader.commitErr = fmt.Errorf("commit failed")

	handler := newMockCommandHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, reader, nil)
	tr.commandHandler = handler
	tr.commandReader = reader

	ctx, cancel := context.WithCancel(context.Background())
	tr.wg.Add(1)
	go tr.readCommands(ctx)

	for i := range 2 {
		select {
		case <-handler.ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for command %d", i+1)
		}
	}

	cancel()
	tr.wg.Wait()

	handler.mu.Lock()
	count := len(handler.received)
	handler.mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 commands, got %d", count)
	}
}

func TestReadCommands_CommitError_ContextCancelled(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("p1"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerCommandName, Value: []byte("Cmd")},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	reader := newMockReader(msg)
	reader.commitErr = fmt.Errorf("commit failed")
	reader.cancelCtx = cancel
	reader.cancelAfterCommitErrs = 1

	handler := newMockCommandHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, reader, nil)
	tr.commandHandler = handler
	tr.commandReader = reader

	tr.wg.Add(1)
	go tr.readCommands(ctx)

	tr.wg.Wait() // should exit because context gets cancelled on commit error
}

// ── readReplies edge cases ───────────────────────────────────────────

func TestReadReplies_SkipOnFetchError(t *testing.T) {
	t.Parallel()
	reader := newMockReader()
	reader.fetchErr = fmt.Errorf("transient")
	handler := newMockReplyHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, reader)
	tr.replyHandler = handler
	tr.replyReader = reader

	ctx, cancel := context.WithCancel(context.Background())
	tr.wg.Add(1)
	go tr.readReplies(ctx)

	time.Sleep(150 * time.Millisecond)
	cancel()
	tr.wg.Wait()

	handler.mu.Lock()
	count := len(handler.received)
	handler.mu.Unlock()
	if count != 0 {
		t.Errorf("expected 0 replies delivered, got %d", count)
	}
}

func TestReadReplies_SkipMalformed(t *testing.T) {
	t.Parallel()
	malformed := kafkago.Message{
		Value: []byte("data"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerError, Value: []byte("not-json")},
		},
	}
	valid := kafkago.Message{
		Value: []byte("ok"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c2")},
			{Key: headerResultName, Value: []byte("Res")},
		},
	}
	reader := newMockReader(malformed, valid)
	handler := newMockReplyHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, reader)
	tr.replyHandler = handler
	tr.replyReader = reader

	ctx, cancel := context.WithCancel(context.Background())
	tr.wg.Add(1)
	go tr.readReplies(ctx)

	select {
	case env := <-handler.ch:
		if env.CorrelationID != "c2" {
			t.Errorf("expected c2, got %s", env.CorrelationID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for valid reply")
	}

	cancel()
	tr.wg.Wait()
}

func TestReadReplies_CommitError_ContinuesProcessing(t *testing.T) {
	t.Parallel()
	msg1 := kafkago.Message{
		Value: []byte("r1"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerResultName, Value: []byte("Res1")},
		},
	}
	msg2 := kafkago.Message{
		Value: []byte("r2"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c2")},
			{Key: headerResultName, Value: []byte("Res2")},
		},
	}
	reader := newMockReader(msg1, msg2)
	reader.commitErr = fmt.Errorf("commit failed")

	handler := newMockReplyHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, reader)
	tr.replyHandler = handler
	tr.replyReader = reader

	ctx, cancel := context.WithCancel(context.Background())
	tr.wg.Add(1)
	go tr.readReplies(ctx)

	for i := range 2 {
		select {
		case <-handler.ch:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for reply %d", i+1)
		}
	}

	cancel()
	tr.wg.Wait()

	handler.mu.Lock()
	count := len(handler.received)
	handler.mu.Unlock()
	if count != 2 {
		t.Errorf("expected 2 replies, got %d", count)
	}
}

func TestReadReplies_CommitError_ContextCancelled(t *testing.T) {
	t.Parallel()
	msg := kafkago.Message{
		Value: []byte("r1"),
		Headers: []kafkago.Header{
			{Key: headerCorrelationID, Value: []byte("c1")},
			{Key: headerResultName, Value: []byte("Res")},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	reader := newMockReader(msg)
	reader.commitErr = fmt.Errorf("commit failed")
	reader.cancelCtx = cancel
	reader.cancelAfterCommitErrs = 1

	handler := newMockReplyHandler()
	tr := newTestTransport(testConfig(), &mockWriter{}, nil, reader)
	tr.replyHandler = handler
	tr.replyReader = reader

	tr.wg.Add(1)
	go tr.readReplies(ctx)

	tr.wg.Wait() // should exit because context gets cancelled on commit error
}

// ── generateInstanceID ───────────────────────────────────────────────

func TestGenerateInstanceID_Unique(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool)
	for range 100 {
		id := generateInstanceID()
		if len(id) != 16 {
			t.Errorf("expected length 16, got %d", len(id))
		}
		if seen[id] {
			t.Errorf("duplicate id: %s", id)
		}
		seen[id] = true
	}
}
