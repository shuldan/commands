package commands

import (
	"testing"
	"time"
)

func TestWithSyncMode(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()
	WithSyncMode()(o)

	if o.asyncMode {
		t.Error("expected asyncMode to be false")
	}
}

func TestWithAsyncMode(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()
	WithSyncMode()(o)
	WithAsyncMode()(o)

	if !o.asyncMode {
		t.Error("expected asyncMode to be true")
	}
}

func TestWithWorkerCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive", 4, 4},
		{"zero_becomes_one", 0, 1},
		{"negative_becomes_one", -5, 1},
		{"one", 1, 1},
		{"large", 1000, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := defaultDispatcherOptions()
			WithWorkerCount(tt.input)(o)
			if o.workerCount != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, o.workerCount)
			}
		})
	}
}

func TestWithBufferSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"positive", 50, 50},
		{"zero", 0, 0},
		{"negative_becomes_zero", -10, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			o := defaultDispatcherOptions()
			WithBufferSize(tt.input)(o)
			if o.bufferSize != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, o.bufferSize)
			}
		})
	}
}

func TestWithPublishTimeout(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()
	WithPublishTimeout(10 * time.Second)(o)

	if o.publishTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %v", o.publishTimeout)
	}
}

func TestWithOrderedDelivery(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()
	WithOrderedDelivery()(o)

	if !o.ordered {
		t.Error("expected ordered to be true")
	}
}

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	mw1 := func(next HandleFunc) HandleFunc { return next }
	mw2 := func(next HandleFunc) HandleFunc { return next }

	o := defaultDispatcherOptions()
	WithMiddleware(mw1, mw2)(o)

	if len(o.middlewares) != 2 {
		t.Errorf("expected 2 middlewares, got %d", len(o.middlewares))
	}
}

func TestWithPanicHandler(t *testing.T) {
	t.Parallel()

	ph := &defaultPanicHandler{}
	o := defaultDispatcherOptions()
	WithPanicHandler(ph)(o)

	if o.panicHandler != ph {
		t.Error("expected custom panic handler")
	}
}

func TestWithErrorHandler(t *testing.T) {
	t.Parallel()

	eh := &defaultErrorHandler{}
	o := defaultDispatcherOptions()
	WithErrorHandler(eh)(o)

	if o.errorHandler != eh {
		t.Error("expected custom error handler")
	}
}

func TestWithMetrics(t *testing.T) {
	t.Parallel()

	m := &recordingMetrics{}
	o := defaultDispatcherOptions()
	WithMetrics(m)(o)

	if o.metrics != m {
		t.Error("expected custom metrics")
	}
}

func TestWithIdempotencyStore(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	o := defaultDispatcherOptions()
	WithIdempotencyStore(store)(o)

	if o.idempotency != store {
		t.Error("expected custom idempotency store")
	}
}

func TestWithIdempotencyTTL(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()
	WithIdempotencyTTL(2 * time.Hour)(o)

	if o.idempotencyTTL != 2*time.Hour {
		t.Errorf("expected 2h, got %v", o.idempotencyTTL)
	}
}

func TestDefaultDispatcherOptions(t *testing.T) {
	t.Parallel()

	o := defaultDispatcherOptions()

	if !o.asyncMode {
		t.Error("expected default asyncMode to be true")
	}

	if o.workerCount < 1 {
		t.Errorf("expected workerCount >= 1, got %d", o.workerCount)
	}

	if o.publishTimeout != 5*time.Second {
		t.Errorf("expected default timeout 5s, got %v", o.publishTimeout)
	}

	if o.ordered {
		t.Error("expected default ordered to be false")
	}

	if o.idempotencyTTL != 24*time.Hour {
		t.Errorf("expected default TTL 24h, got %v", o.idempotencyTTL)
	}

	if o.panicHandler == nil {
		t.Error("expected non-nil default panic handler")
	}

	if o.errorHandler == nil {
		t.Error("expected non-nil default error handler")
	}

	if o.metrics == nil {
		t.Error("expected non-nil default metrics")
	}

	if o.idempotency == nil {
		t.Error("expected non-nil default idempotency store")
	}
}
