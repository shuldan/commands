package commands

// Command represents a command that can be sent through the command bus.
type Command interface {
	CommandName() string
}

// Result represents a result returned from a command handler.
type Result interface {
	ResultName() string
}
