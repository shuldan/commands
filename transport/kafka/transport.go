package kafka

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/shuldan/commands"
)

type Transport struct {
	cfg    Config
	writer messageWriter

	mu             sync.Mutex
	commandReader  messageReader
	replyReader    messageReader
	commandHandler commands.CommandHandler
	replyHandler   commands.ReplyHandler

	cancelFuncs []context.CancelFunc
	wg          sync.WaitGroup
	closed      bool

	newCommandReader func() messageReader
	newReplyReader   func(groupID string) messageReader
}

func New(cfg Config) (*Transport, error) {
	cfg.withDefaults()
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.AutoCreateTopics {
		if err := ensureTopics(cfg); err != nil {
			return nil, fmt.Errorf("ensure topics: %w", err)
		}
	} else {
		if err := checkTopicsExist(cfg); err != nil {
			return nil, err
		}
	}

	return newTransport(cfg), nil
}

func newTransport(cfg Config) *Transport {
	writer := &kafkago.Writer{
		Addr:         kafkago.TCP(cfg.Brokers...),
		Balancer:     &kafkago.RoundRobin{},
		WriteTimeout: cfg.WriteTimeout,
		RequiredAcks: kafkago.RequireAll,
		Async:        false,
	}

	t := &Transport{
		cfg:    cfg,
		writer: writer,
	}

	t.newCommandReader = func() messageReader {
		return kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        cfg.Brokers,
			Topic:          cfg.CommandTopic,
			GroupID:        cfg.ConsumerGroup,
			MaxBytes:       cfg.MaxBytes,
			CommitInterval: cfg.CommitInterval,
			StartOffset:    kafkago.LastOffset,
		})
	}

	t.newReplyReader = func(groupID string) messageReader {
		return kafkago.NewReader(kafkago.ReaderConfig{
			Brokers:        cfg.Brokers,
			Topic:          cfg.ReplyTopic,
			GroupID:        groupID,
			MaxBytes:       cfg.MaxBytes,
			CommitInterval: cfg.CommitInterval,
			StartOffset:    kafkago.LastOffset,
		})
	}

	return t
}

func (t *Transport) Send(ctx context.Context, env commands.CommandEnvelope) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()

	msg := commandEnvelopeToMessage(t.cfg.CommandTopic, env)
	return t.writer.WriteMessages(ctx, msg)
}

func (t *Transport) Subscribe(ctx context.Context, handler commands.CommandHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}
	if t.commandHandler != nil {
		return ErrCommandHandlerAlreadySubscribed
	}
	if t.cfg.ConsumerGroup == "" {
		return ErrMissingConsumerGroup
	}

	t.commandHandler = handler
	t.commandReader = t.newCommandReader()

	loopCtx, cancel := context.WithCancel(ctx)
	t.cancelFuncs = append(t.cancelFuncs, cancel)

	t.wg.Add(1)
	go t.readCommands(loopCtx)

	return nil
}

func (t *Transport) Reply(ctx context.Context, env commands.ReplyEnvelope) error {
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return ErrTransportClosed
	}
	t.mu.Unlock()

	msg, err := replyEnvelopeToMessage(t.cfg.ReplyTopic, env)
	if err != nil {
		return fmt.Errorf("build reply message: %w", err)
	}

	return t.writer.WriteMessages(ctx, msg)
}

func (t *Transport) SubscribeReplies(ctx context.Context, handler commands.ReplyHandler) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}
	if t.replyHandler != nil {
		return ErrReplyHandlerAlreadySubscribed
	}

	t.replyHandler = handler

	replyGroup := fmt.Sprintf("%s-%s", t.cfg.ReplyTopic, generateInstanceID())
	t.replyReader = t.newReplyReader(replyGroup)

	loopCtx, cancel := context.WithCancel(ctx)
	t.cancelFuncs = append(t.cancelFuncs, cancel)

	t.wg.Add(1)
	go t.readReplies(loopCtx)

	return nil
}

func (t *Transport) ReplyAddress() string {
	return t.cfg.ReplyTopic
}

func (t *Transport) Close(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrTransportClosed
	}
	t.closed = true

	for _, cancel := range t.cancelFuncs {
		cancel()
	}

	t.wg.Wait()

	var errs []error

	if t.commandReader != nil {
		if err := t.commandReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close command reader: %w", err))
		}
	}

	if t.replyReader != nil {
		if err := t.replyReader.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close reply reader: %w", err))
		}
	}

	if err := t.writer.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close writer: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("close transport: %v", errs)
	}

	return nil
}

func (t *Transport) readCommands(ctx context.Context) {
	defer t.wg.Done()

	for {
		msg, err := t.commandReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		env := messageToCommandEnvelope(msg)
		t.commandHandler.Handle(ctx, env)

		if err := t.commandReader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (t *Transport) readReplies(ctx context.Context) {
	defer t.wg.Done()

	for {
		msg, err := t.replyReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		env, err := messageToReplyEnvelope(msg)
		if err != nil {
			_ = t.replyReader.CommitMessages(ctx, msg)
			continue
		}

		t.replyHandler.Handle(ctx, env)

		if err := t.replyReader.CommitMessages(ctx, msg); err != nil {
			if ctx.Err() != nil {
				return
			}
		}
	}
}

func ensureTopics(cfg Config) error {
	conn, err := kafkago.Dial("tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer func() { _ = conn.Close() }()

	controller, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("get controller: %w", err)
	}

	controllerConn, err := kafkago.Dial("tcp", net.JoinHostPort(controller.Host, strconv.Itoa(controller.Port)))
	if err != nil {
		return fmt.Errorf("dial controller: %w", err)
	}
	defer func() { _ = controllerConn.Close() }()

	topics := []kafkago.TopicConfig{
		{
			Topic:             cfg.CommandTopic,
			NumPartitions:     cfg.NumPartitions,
			ReplicationFactor: cfg.ReplicationFactor,
		},
		{
			Topic:             cfg.ReplyTopic,
			NumPartitions:     cfg.NumPartitions,
			ReplicationFactor: cfg.ReplicationFactor,
		},
	}

	return controllerConn.CreateTopics(topics...)
}

func checkTopicsExist(cfg Config) error {
	conn, err := kafkago.Dial("tcp", cfg.Brokers[0])
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer func() { _ = conn.Close() }()

	partitions, err := conn.ReadPartitions()
	if err != nil {
		return fmt.Errorf("read partitions: %w", err)
	}

	existing := make(map[string]bool)
	for _, p := range partitions {
		existing[p.Topic] = true
	}

	if !existing[cfg.CommandTopic] {
		return fmt.Errorf("%w: %s", ErrTopicNotFound, cfg.CommandTopic)
	}
	if !existing[cfg.ReplyTopic] {
		return fmt.Errorf("%w: %s", ErrTopicNotFound, cfg.ReplyTopic)
	}

	return nil
}

func generateInstanceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
