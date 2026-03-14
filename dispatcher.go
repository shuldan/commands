package commands

import (
	"context"
	"fmt"
	"hash/fnv"
	"runtime/debug"
	"sync"
	"time"
)

type commandTask struct {
	ctx     context.Context
	cmd     Command
	handler HandleFunc
}

// Dispatcher — локальный диспетчер команд.
type Dispatcher struct {
	mu             sync.RWMutex
	handlers       map[string]HandleFunc
	resultHandlers map[string]ResultHandleFunc
	closed         bool
	wg             sync.WaitGroup
	opts           *dispatcherOptions
	sharedChan     chan commandTask
	workerChans    []chan commandTask
}

// New создаёт новый Dispatcher.
func New(opts ...Option) *Dispatcher {
	o := defaultDispatcherOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.bufferSize == 0 {
		o.bufferSize = o.workerCount * 10 //nolint:mnd
	}

	d := &Dispatcher{
		handlers:       make(map[string]HandleFunc),
		resultHandlers: make(map[string]ResultHandleFunc),
		opts:           o,
	}

	if o.asyncMode {
		d.startWorkers()
	}

	return d
}

// Send отправляет команду на выполнение.
func (d *Dispatcher) Send(ctx context.Context, cmd Command) error {
	if cmd == nil {
		return ErrNilCommand
	}

	d.mu.RLock()
	if d.closed {
		d.mu.RUnlock()

		return ErrDispatcherClosed
	}

	name := cmd.CommandName()
	handler, ok := d.handlers[name]
	d.mu.RUnlock()

	if !ok {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, name)
	}

	d.opts.metrics.CommandDispatched(name)

	if !d.opts.asyncMode {
		d.processCommand(commandTask{
			ctx: ctx, cmd: cmd, handler: handler,
		})

		return nil
	}

	return d.enqueue(ctx, cmd, handler)
}

// Close завершает работу диспетчера.
func (d *Dispatcher) Close(ctx context.Context) error {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()

		return nil
	}

	d.closed = true
	d.mu.Unlock()

	if d.opts.asyncMode {
		if d.opts.ordered {
			for _, ch := range d.workerChans {
				close(ch)
			}
		} else {
			close(d.sharedChan)
		}
	}

	done := make(chan struct{})

	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Dispatcher) registerHandler(
	name string, handler HandleFunc,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.handlers[name]; exists {
		return fmt.Errorf("%w: %s", ErrHandlerExists, name)
	}

	d.handlers[name] = handler

	return nil
}

func (d *Dispatcher) registerResultHandler(
	commandName string, handler ResultHandleFunc,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.resultHandlers[commandName]; exists {
		return fmt.Errorf(
			"%w: %s", ErrResultHandlerExists, commandName,
		)
	}

	d.resultHandlers[commandName] = handler

	return nil
}

func (d *Dispatcher) startWorkers() {
	if d.opts.ordered {
		d.workerChans = make([]chan commandTask, d.opts.workerCount)

		for i := range d.opts.workerCount {
			d.workerChans[i] = make(
				chan commandTask, d.opts.bufferSize,
			)
			d.wg.Add(1)

			go d.worker(d.workerChans[i])
		}
	} else {
		d.sharedChan = make(chan commandTask, d.opts.bufferSize)

		for range d.opts.workerCount {
			d.wg.Add(1)

			go d.worker(d.sharedChan)
		}
	}
}

func (d *Dispatcher) worker(ch <-chan commandTask) {
	defer d.wg.Done()

	for task := range ch {
		d.processCommand(task)
	}
}

func (d *Dispatcher) enqueue(
	ctx context.Context,
	cmd Command,
	handler HandleFunc,
) error {
	task := commandTask{ctx: ctx, cmd: cmd, handler: handler}
	name := cmd.CommandName()

	var ch chan commandTask

	if d.opts.ordered {
		idx := d.partitionIndex(cmd.IdempotencyKey())
		ch = d.workerChans[idx]
	} else {
		ch = d.sharedChan
	}

	d.opts.metrics.QueueDepth(len(ch))

	select {
	case ch <- task:
		return nil
	case <-ctx.Done():
		d.opts.metrics.CommandDropped(name, "context_cancelled")

		return ctx.Err()
	case <-time.After(d.opts.publishTimeout):
		d.opts.metrics.CommandDropped(name, "timeout")

		return ErrDispatcherClosed
	}
}

func (d *Dispatcher) partitionIndex(key string) int {
	if key == "" {
		return 0
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(key))

	return int(h.Sum32()) % d.opts.workerCount
}

func (d *Dispatcher) processCommand(task commandTask) {
	cmd := task.cmd
	name := cmd.CommandName()
	key := cmd.IdempotencyKey()

	duplicate, err := d.opts.idempotency.Exists(task.ctx, key)
	if err != nil {
		d.opts.errorHandler.Handle(ErrorContext{
			CommandName:    name,
			IdempotencyKey: key,
			Err:            fmt.Errorf("idempotency check: %w", err),
		})

		return
	}

	if duplicate {
		d.opts.metrics.CommandDuplicate(name)

		return
	}

	start := time.Now()
	result, handleErr := d.safeExecute(task)
	duration := time.Since(start)

	d.opts.metrics.CommandHandled(name, duration, handleErr)

	if handleErr != nil {
		d.opts.errorHandler.Handle(ErrorContext{
			CommandName:    name,
			IdempotencyKey: key,
			Err:            handleErr,
		})

		d.deliverResult(task.ctx, name, nil, handleErr)

		return
	}

	if markErr := d.opts.idempotency.Mark(
		task.ctx, key, d.opts.idempotencyTTL,
	); markErr != nil {
		d.opts.errorHandler.Handle(ErrorContext{
			CommandName:    name,
			IdempotencyKey: key,
			Err:            fmt.Errorf("idempotency mark: %w", markErr),
		})
	}

	d.deliverResult(task.ctx, name, result, nil)
}

func (d *Dispatcher) safeExecute(
	task commandTask,
) (result Result, retErr error) {
	defer func() {
		if r := recover(); r != nil {
			d.opts.panicHandler.Handle(PanicContext{
				CommandName:    task.cmd.CommandName(),
				IdempotencyKey: task.cmd.IdempotencyKey(),
				PanicValue:     r,
				Stack:          debug.Stack(),
			})

			retErr = fmt.Errorf("panic recovered: %v", r)
		}
	}()

	return task.handler(task.ctx, task.cmd)
}

func (d *Dispatcher) deliverResult(
	ctx context.Context,
	commandName string,
	result Result,
	err error,
) {
	d.mu.RLock()
	handler, ok := d.resultHandlers[commandName]
	d.mu.RUnlock()

	if !ok {
		return
	}

	if resultErr := handler(ctx, result, err); resultErr != nil {
		d.opts.errorHandler.Handle(ErrorContext{
			CommandName: commandName,
			Err: fmt.Errorf(
				"result handler: %w", resultErr,
			),
		})
	}
}
