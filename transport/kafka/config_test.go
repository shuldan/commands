package kafka

import (
	"testing"
	"time"
)

func TestValidate_MissingBrokers(t *testing.T) {
	t.Parallel()
	cfg := Config{CommandTopic: "cmd", ReplyTopic: "reply"}
	err := cfg.validate()
	if err != ErrMissingBrokers {
		t.Errorf("expected ErrMissingBrokers, got %v", err)
	}
}

func TestValidate_MissingCommandTopic(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"b"}, ReplyTopic: "reply"}
	err := cfg.validate()
	if err != ErrMissingCommandTopic {
		t.Errorf("expected ErrMissingCommandTopic, got %v", err)
	}
}

func TestValidate_MissingReplyTopic(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"b"}, CommandTopic: "cmd"}
	err := cfg.validate()
	if err != ErrMissingReplyTopic {
		t.Errorf("expected ErrMissingReplyTopic, got %v", err)
	}
}

func TestValidate_Success(t *testing.T) {
	t.Parallel()
	cfg := Config{Brokers: []string{"b"}, CommandTopic: "cmd", ReplyTopic: "reply"}
	if err := cfg.validate(); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWithDefaults_SetsDefaults(t *testing.T) {
	t.Parallel()
	cfg := Config{}
	cfg.withDefaults()

	if cfg.NumPartitions != 1 {
		t.Errorf("expected NumPartitions=1, got %d", cfg.NumPartitions)
	}
	if cfg.ReplicationFactor != 1 {
		t.Errorf("expected ReplicationFactor=1, got %d", cfg.ReplicationFactor)
	}
	if cfg.MaxBytes != int(1e6) {
		t.Errorf("expected MaxBytes=1000000, got %d", cfg.MaxBytes)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout=10s, got %v", cfg.WriteTimeout)
	}
}

func TestWithDefaults_PreservesExplicit(t *testing.T) {
	t.Parallel()
	cfg := Config{
		NumPartitions:     5,
		ReplicationFactor: 3,
		MaxBytes:          2e6,
		WriteTimeout:      30 * time.Second,
	}
	cfg.withDefaults()

	if cfg.NumPartitions != 5 {
		t.Errorf("expected NumPartitions=5, got %d", cfg.NumPartitions)
	}
	if cfg.ReplicationFactor != 3 {
		t.Errorf("expected ReplicationFactor=3, got %d", cfg.ReplicationFactor)
	}
	if cfg.MaxBytes != int(2e6) {
		t.Errorf("expected MaxBytes=2000000, got %d", cfg.MaxBytes)
	}
	if cfg.WriteTimeout != 30*time.Second {
		t.Errorf("expected WriteTimeout=30s, got %v", cfg.WriteTimeout)
	}
}
