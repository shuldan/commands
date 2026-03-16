package commands

import (
	"context"
	"fmt"
)

type replySender struct {
	address   ReplyAddress
	codec     Codec
	transport Transport
}

// NewReplySender creates a ReplySender for deferred replies.
// Use this when sending a reply from a saved ReplyAddress.
func NewReplySender(transport Transport, codec Codec, addr ReplyAddress) ReplySender {
	return &replySender{
		address:   addr,
		codec:     codec,
		transport: transport,
	}
}

func (r *replySender) Send(ctx context.Context, result Result) error {
	payload, err := r.codec.Encode(result)
	if err != nil {
		return fmt.Errorf("encode result: %w", err)
	}

	return r.transport.Reply(ctx, ReplyEnvelope{
		CorrelationID: r.address.CorrelationID,
		ResultName:    result.ResultName(),
		Payload:       payload,
	})
}

func (r *replySender) SendError(ctx context.Context, err error) error {
	ep, ok := err.(*ErrorPayload)
	if !ok {
		ep = &ErrorPayload{
			Code:    "UNKNOWN",
			Message: err.Error(),
		}
	}

	return r.transport.Reply(ctx, ReplyEnvelope{
		CorrelationID: r.address.CorrelationID,
		Error:         ep,
	})
}

func (r *replySender) Address() ReplyAddress {
	return r.address
}
