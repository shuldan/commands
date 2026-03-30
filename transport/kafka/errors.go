package kafka

import "errors"

var (
	// ErrTopicNotFound is returned when a required topic does not exist and auto-creation is disabled.
	ErrTopicNotFound = errors.New("topic not found and auto-creation is disabled")

	// ErrMissingBrokers is returned when no brokers are configured.
	ErrMissingBrokers = errors.New("brokers are required")

	// ErrMissingCommandTopic is returned when command topic is not configured.
	ErrMissingCommandTopic = errors.New("command topic is required")

	// ErrMissingReplyTopic is returned when reply topic is not configured.
	ErrMissingReplyTopic = errors.New("reply topic is required")

	// ErrMissingConsumerGroup is returned when consumer group is not configured but Subscribe is called.
	ErrMissingConsumerGroup = errors.New("consumer group is required for Subscribe")

	// ErrTransportClosed is returned when the transport is already closed.
	ErrTransportClosed = errors.New("transport is closed")

	// ErrCommandHandlerAlreadySubscribed is returned when Subscribe is called more than once.
	ErrCommandHandlerAlreadySubscribed = errors.New("command handler already subscribed")

	// ErrReplyHandlerAlreadySubscribed is returned when SubscribeReplies is called more than once.
	ErrReplyHandlerAlreadySubscribed = errors.New("reply handler already subscribed")
)
