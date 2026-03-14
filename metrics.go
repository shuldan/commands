package commands

import "time"

// MetricsCollector собирает метрики командной шины.
type MetricsCollector interface {
	CommandDispatched(commandName string)
	CommandHandled(commandName string, duration time.Duration, err error)
	CommandDropped(commandName string, reason string)
	CommandDuplicate(commandName string)
	QueueDepth(depth int)
}

type noopMetrics struct{}

func (n *noopMetrics) CommandDispatched(string)                    {}
func (n *noopMetrics) CommandHandled(string, time.Duration, error) {}
func (n *noopMetrics) CommandDropped(string, string)               {}
func (n *noopMetrics) CommandDuplicate(string)                     {}
func (n *noopMetrics) QueueDepth(int)                              {}
