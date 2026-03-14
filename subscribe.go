package commands

import "context"

// Handle регистрирует типизированный обработчик команды.
func Handle[C Command](
	d *Dispatcher,
	handler Handler[C],
) error {
	return HandleFn(d, handler.Handle)
}

// HandleFn регистрирует функцию-обработчик команды.
func HandleFn[C Command](
	d *Dispatcher,
	fn func(context.Context, C) (Result, error),
) error {
	var zero C
	name := zero.CommandName()

	wrapped := func(ctx context.Context, cmd Command) (Result, error) {
		typed, ok := cmd.(C)
		if !ok {
			return nil, ErrTypeMismatch
		}

		return fn(ctx, typed)
	}

	final := buildChain(wrapped, d.opts.middlewares)

	return d.registerHandler(name, final)
}

// OnResult регистрирует типизированный обработчик результата.
func OnResult[R Result](
	d *Dispatcher,
	commandName string,
	handler ResultHandler[R],
) error {
	return OnResultFunc(d, commandName, handler.Handle)
}

// OnResultFunc регистрирует функцию-обработчик результата.
func OnResultFunc[R Result](
	d *Dispatcher,
	commandName string,
	fn func(context.Context, R, error) error,
) error {
	wrapped := func(ctx context.Context, result Result, err error) error {
		if err != nil {
			var zero R
			return fn(ctx, zero, err)
		}

		typed, ok := result.(R)
		if !ok {
			return ErrTypeMismatch
		}

		return fn(ctx, typed, nil)
	}

	return d.registerResultHandler(commandName, wrapped)
}
