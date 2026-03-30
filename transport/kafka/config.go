package kafka

import "time"

// Config configures the Kafka transport.
type Config struct {
	// Brokers is the list of Kafka broker addresses.
	Brokers []string

	// CommandTopic is the topic for command messages.
	CommandTopic string

	// ReplyTopic is the topic for reply messages.
	ReplyTopic string

	// ConsumerGroup is the consumer group for the server side (Subscribe).
	// Required only if Subscribe is called.
	ConsumerGroup string

	// AutoCreateTopics enables automatic topic creation on transport initialization.
	// Disabled by default — topics must exist or an error is returned.
	AutoCreateTopics bool

	// NumPartitions is the number of partitions for auto-created topics. Default: 1.
	NumPartitions int

	// ReplicationFactor is the replication factor for auto-created topics. Default: 1.
	ReplicationFactor int

	// MaxBytes is the maximum size of a message batch from the consumer. Default: 1MB.
	MaxBytes int

	// CommitInterval indicates the interval at which offsets are committed to
	// the broker. Default: 0 (synchronous commits after each message).
	CommitInterval time.Duration

	// WriteTimeout is the timeout for producer write operations. Default: 10s.
	WriteTimeout time.Duration
}

func (c *Config) validate() error {
	if len(c.Brokers) == 0 {
		return ErrMissingBrokers
	}
	if c.CommandTopic == "" {
		return ErrMissingCommandTopic
	}
	if c.ReplyTopic == "" {
		return ErrMissingReplyTopic
	}
	return nil
}

func (c *Config) withDefaults() {
	if c.NumPartitions <= 0 {
		c.NumPartitions = 1
	}
	if c.ReplicationFactor <= 0 {
		c.ReplicationFactor = 1
	}
	if c.MaxBytes <= 0 {
		c.MaxBytes = 1e6 // 1MB
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = 10 * time.Second
	}
}
