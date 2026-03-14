package commands

import (
	"runtime"
	"time"
)

// Option настраивает Dispatcher.
type Option func(*dispatcherOptions)

type dispatcherOptions struct {
	asyncMode      bool
	workerCount    int
	bufferSize     int
	publishTimeout time.Duration
	ordered        bool
	idempotencyTTL time.Duration

	middlewares  []Middleware
	panicHandler PanicHandler
	errorHandler ErrorHandler
	metrics      MetricsCollector
	idempotency  IdempotencyStore
}

func defaultDispatcherOptions() *dispatcherOptions {
	return &dispatcherOptions{
		asyncMode:      true,
		workerCount:    runtime.NumCPU(),
		bufferSize:     0,
		publishTimeout: 5 * time.Second, //nolint:mnd
		ordered:        false,
		idempotencyTTL: 24 * time.Hour, //nolint:mnd
		panicHandler:   &defaultPanicHandler{},
		errorHandler:   &defaultErrorHandler{},
		metrics:        &noopMetrics{},
		idempotency:    NewMemoryIdempotencyStore(),
	}
}

// WithAsyncMode включает асинхронную обработку команд.
func WithAsyncMode() Option {
	return func(o *dispatcherOptions) { o.asyncMode = true }
}

// WithSyncMode включает синхронную обработку команд.
func WithSyncMode() Option {
	return func(o *dispatcherOptions) { o.asyncMode = false }
}

// WithWorkerCount задаёт количество воркеров.
func WithWorkerCount(n int) Option {
	return func(o *dispatcherOptions) {
		if n < 1 {
			n = 1
		}
		o.workerCount = n
	}
}

// WithBufferSize задаёт размер буфера канала.
func WithBufferSize(size int) Option {
	return func(o *dispatcherOptions) {
		if size < 0 {
			size = 0
		}
		o.bufferSize = size
	}
}

// WithPublishTimeout задаёт таймаут при отправке в канал.
func WithPublishTimeout(d time.Duration) Option {
	return func(o *dispatcherOptions) { o.publishTimeout = d }
}

// WithOrderedDelivery гарантирует последовательную обработку
// команд с одинаковым idempotency key.
func WithOrderedDelivery() Option {
	return func(o *dispatcherOptions) { o.ordered = true }
}

// WithMiddleware добавляет middleware к диспетчеру.
func WithMiddleware(mw ...Middleware) Option {
	return func(o *dispatcherOptions) {
		o.middlewares = append(o.middlewares, mw...)
	}
}

// WithPanicHandler задаёт обработчик паник.
func WithPanicHandler(h PanicHandler) Option {
	return func(o *dispatcherOptions) { o.panicHandler = h }
}

// WithErrorHandler задаёт обработчик ошибок.
func WithErrorHandler(h ErrorHandler) Option {
	return func(o *dispatcherOptions) { o.errorHandler = h }
}

// WithMetrics задаёт коллектор метрик.
func WithMetrics(m MetricsCollector) Option {
	return func(o *dispatcherOptions) { o.metrics = m }
}

// WithIdempotencyStore задаёт хранилище идемпотентности.
func WithIdempotencyStore(store IdempotencyStore) Option {
	return func(o *dispatcherOptions) { o.idempotency = store }
}

// WithIdempotencyTTL задаёт время хранения ключей идемпотентности.
func WithIdempotencyTTL(ttl time.Duration) Option {
	return func(o *dispatcherOptions) { o.idempotencyTTL = ttl }
}
