package commands

// Logger is a minimal logging interface.
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

type noopLogger struct{}

func (n noopLogger) Info(_ string, _ ...any)  {}
func (n noopLogger) Warn(_ string, _ ...any)  {}
func (n noopLogger) Error(_ string, _ ...any) {}
