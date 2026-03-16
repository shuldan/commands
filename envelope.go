package commands

// CommandEnvelope is a transport-level wrapper for a command.
type CommandEnvelope struct {
	MessageID     string            `json:"message_id"`
	CorrelationID string            `json:"correlation_id"`
	CommandName   string            `json:"command_name"`
	ReplyTo       string            `json:"reply_to"`
	Headers       map[string]string `json:"headers,omitempty"`
	Payload       []byte            `json:"payload"`
}

// ReplyEnvelope is a transport-level wrapper for a reply.
type ReplyEnvelope struct {
	CorrelationID string            `json:"correlation_id"`
	ResultName    string            `json:"result_name,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Payload       []byte            `json:"payload,omitempty"`
	Error         *ErrorPayload     `json:"error,omitempty"`
}

// ReplyAddress contains the information needed to send a reply.
type ReplyAddress struct {
	CorrelationID string `json:"correlation_id"`
	ReplyTo       string `json:"reply_to"`
}
