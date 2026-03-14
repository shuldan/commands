package commands

import (
	"testing"
	"time"
)

func TestNoopMetrics_AllMethods(t *testing.T) {
	t.Parallel()

	m := &noopMetrics{}
	m.CommandDispatched("test")
	m.CommandHandled("test", time.Second, nil)
	m.CommandDropped("test", "reason")
	m.CommandDuplicate("test")
	m.QueueDepth(10)
}

type recordingMetrics struct {
	dispatched int
	handled    int
	dropped    int
	duplicate  int
	depth      int
}

func (r *recordingMetrics) CommandDispatched(string)                    { r.dispatched++ }
func (r *recordingMetrics) CommandHandled(string, time.Duration, error) { r.handled++ }
func (r *recordingMetrics) CommandDropped(string, string)               { r.dropped++ }
func (r *recordingMetrics) CommandDuplicate(string)                     { r.duplicate++ }
func (r *recordingMetrics) QueueDepth(d int)                            { r.depth = d }
