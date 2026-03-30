package kafka

import (
	"context"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/commands"
)

type mockWriter struct {
	mu       sync.Mutex
	messages []kafkago.Message
	err      error
	closed   bool
	closeErr error
}

func (w *mockWriter) WriteMessages(_ context.Context, msgs ...kafkago.Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	w.messages = append(w.messages, msgs...)
	return nil
}

func (w *mockWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return w.closeErr
}

type mockReader struct {
	mu       sync.Mutex
	messages []kafkago.Message
	idx      int
	fetchErr error
	commitCh chan kafkago.Message
	closed   bool
	closeErr error
}

func newMockReader(msgs ...kafkago.Message) *mockReader {
	return &mockReader{
		messages: msgs,
		commitCh: make(chan kafkago.Message, 100),
	}
}

func (r *mockReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	for {
		r.mu.Lock()
		if r.fetchErr != nil {
			err := r.fetchErr
			r.mu.Unlock()
			return kafkago.Message{}, err
		}
		if r.idx < len(r.messages) {
			msg := r.messages[r.idx]
			r.idx++
			r.mu.Unlock()
			return msg, nil
		}
		r.mu.Unlock()

		select {
		case <-ctx.Done():
			return kafkago.Message{}, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (r *mockReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	for _, m := range msgs {
		r.commitCh <- m
	}
	return nil
}

func (r *mockReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	return r.closeErr
}

type mockCommandHandler struct {
	mu       sync.Mutex
	received []commands.CommandEnvelope
	ch       chan commands.CommandEnvelope
}

func newMockCommandHandler() *mockCommandHandler {
	return &mockCommandHandler{
		ch: make(chan commands.CommandEnvelope, 100),
	}
}

func (h *mockCommandHandler) Handle(_ context.Context, env commands.CommandEnvelope) {
	h.mu.Lock()
	h.received = append(h.received, env)
	h.mu.Unlock()
	h.ch <- env
}

type mockReplyHandler struct {
	mu       sync.Mutex
	received []commands.ReplyEnvelope
	ch       chan commands.ReplyEnvelope
}

func newMockReplyHandler() *mockReplyHandler {
	return &mockReplyHandler{
		ch: make(chan commands.ReplyEnvelope, 100),
	}
}

func (h *mockReplyHandler) Handle(_ context.Context, env commands.ReplyEnvelope) {
	h.mu.Lock()
	h.received = append(h.received, env)
	h.mu.Unlock()
	h.ch <- env
}

func testConfig() Config {
	return Config{
		Brokers:       []string{"localhost:9092"},
		CommandTopic:  "test-commands",
		ReplyTopic:    "test-replies",
		ConsumerGroup: "test-group",
	}
}

func newTestTransport(cfg Config, w messageWriter, cmdReader, replyReader messageReader) *Transport {
	cfg.withDefaults()
	t := &Transport{
		cfg:    cfg,
		writer: w,
		newCommandReader: func() messageReader {
			return cmdReader
		},
		newReplyReader: func(_ string) messageReader {
			return replyReader
		},
	}
	return t
}
