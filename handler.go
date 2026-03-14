package commands

import "log/slog"

// ErrorContext содержит информацию об ошибке обработки команды.
type ErrorContext struct {
	CommandName    string
	IdempotencyKey string
	Err            error
}

// PanicContext содержит информацию о панике при обработке команды.
type PanicContext struct {
	CommandName    string
	IdempotencyKey string
	PanicValue     any
	Stack          []byte
}

// ErrorHandler обрабатывает ошибки выполнения команд.
type ErrorHandler interface {
	Handle(ctx ErrorContext)
}

// PanicHandler обрабатывает паники при выполнении команд.
type PanicHandler interface {
	Handle(ctx PanicContext)
}

type defaultErrorHandler struct{}

func (d *defaultErrorHandler) Handle(ctx ErrorContext) {
	slog.Error("command error",
		"command", ctx.CommandName,
		"idempotency_key", ctx.IdempotencyKey,
		"error", ctx.Err,
	)
}

type defaultPanicHandler struct{}

func (d *defaultPanicHandler) Handle(ctx PanicContext) {
	slog.Error("command panic",
		"command", ctx.CommandName,
		"idempotency_key", ctx.IdempotencyKey,
		"panic", ctx.PanicValue,
		"stack", string(ctx.Stack),
	)
}
