package commands

import "context"

// Handler обрабатывает команду типа C и возвращает результат.
type Handler[C Command] interface {
	Handle(ctx context.Context, cmd C) (Result, error)
}

// HandlerFunc — функциональный адаптер для Handler.
type HandlerFunc[C Command] func(ctx context.Context, cmd C) (Result, error)

// Handle вызывает функцию.
func (f HandlerFunc[C]) Handle(ctx context.Context, cmd C) (Result, error) {
	return f(ctx, cmd)
}

// ResultHandler обрабатывает результат выполнения команды.
type ResultHandler[R Result] interface {
	Handle(ctx context.Context, result R, err error) error
}

// ResultHandlerFunc — функциональный адаптер для ResultHandler.
type ResultHandlerFunc[R Result] func(ctx context.Context, result R, err error) error

// Handle вызывает функцию.
func (f ResultHandlerFunc[R]) Handle(ctx context.Context, result R, err error) error {
	return f(ctx, result, err)
}

// HandleFunc — обобщённый тип обработчика команды.
type HandleFunc func(ctx context.Context, cmd Command) (Result, error)

// ResultHandleFunc — обобщённый тип обработчика результата.
type ResultHandleFunc func(ctx context.Context, result Result, err error) error
