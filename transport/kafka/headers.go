package kafka

import (
	"encoding/json"

	"github.com/segmentio/kafka-go"

	"github.com/shuldan/commands"
)

const (
	headerMessageID     = "message_id"
	headerCorrelationID = "correlation_id"
	headerCommandName   = "command_name"
	headerReplyTo       = "reply_to"
	headerResultName    = "result_name"
	headerError         = "error"
	headerPrefix        = "x-"
)

// commandEnvelopeToMessage converts a CommandEnvelope to a kafka.Message.
func commandEnvelopeToMessage(topic string, env commands.CommandEnvelope) kafka.Message {
	headers := []kafka.Header{
		{Key: headerMessageID, Value: []byte(env.MessageID)},
		{Key: headerCorrelationID, Value: []byte(env.CorrelationID)},
		{Key: headerCommandName, Value: []byte(env.CommandName)},
		{Key: headerReplyTo, Value: []byte(env.ReplyTo)},
	}

	for k, v := range env.Headers {
		headers = append(headers, kafka.Header{
			Key:   headerPrefix + k,
			Value: []byte(v),
		})
	}

	return kafka.Message{
		Topic:   topic,
		Headers: headers,
		Value:   env.Payload,
	}
}

// messageToCommandEnvelope converts a kafka.Message to a CommandEnvelope.
func messageToCommandEnvelope(msg kafka.Message) commands.CommandEnvelope {
	env := commands.CommandEnvelope{
		Payload: msg.Value,
	}

	var custom map[string]string

	for _, h := range msg.Headers {
		switch h.Key {
		case headerMessageID:
			env.MessageID = string(h.Value)
		case headerCorrelationID:
			env.CorrelationID = string(h.Value)
		case headerCommandName:
			env.CommandName = string(h.Value)
		case headerReplyTo:
			env.ReplyTo = string(h.Value)
		default:
			if len(h.Key) > len(headerPrefix) && h.Key[:len(headerPrefix)] == headerPrefix {
				if custom == nil {
					custom = make(map[string]string)
				}
				custom[h.Key[len(headerPrefix):]] = string(h.Value)
			}
		}
	}

	if custom != nil {
		env.Headers = custom
	}

	return env
}

// replyEnvelopeToMessage converts a ReplyEnvelope to a kafka.Message.
func replyEnvelopeToMessage(topic string, env commands.ReplyEnvelope) (kafka.Message, error) {
	headers := []kafka.Header{
		{Key: headerCorrelationID, Value: []byte(env.CorrelationID)},
	}

	if env.ResultName != "" {
		headers = append(headers, kafka.Header{
			Key:   headerResultName,
			Value: []byte(env.ResultName),
		})
	}

	if env.Error != nil {
		errBytes, err := json.Marshal(env.Error)
		if err != nil {
			return kafka.Message{}, err
		}
		headers = append(headers, kafka.Header{
			Key:   headerError,
			Value: errBytes,
		})
	}

	for k, v := range env.Headers {
		headers = append(headers, kafka.Header{
			Key:   headerPrefix + k,
			Value: []byte(v),
		})
	}

	return kafka.Message{
		Topic:   topic,
		Headers: headers,
		Value:   env.Payload,
	}, nil
}

// messageToReplyEnvelope converts a kafka.Message to a ReplyEnvelope.
func messageToReplyEnvelope(msg kafka.Message) (commands.ReplyEnvelope, error) {
	env := commands.ReplyEnvelope{
		Payload: msg.Value,
	}

	var custom map[string]string

	for _, h := range msg.Headers {
		switch h.Key {
		case headerCorrelationID:
			env.CorrelationID = string(h.Value)
		case headerResultName:
			env.ResultName = string(h.Value)
		case headerError:
			var ep commands.ErrorPayload
			if err := json.Unmarshal(h.Value, &ep); err != nil {
				return env, err
			}
			env.Error = &ep
		default:
			if len(h.Key) > len(headerPrefix) && h.Key[:len(headerPrefix)] == headerPrefix {
				if custom == nil {
					custom = make(map[string]string)
				}
				custom[h.Key[len(headerPrefix):]] = string(h.Value)
			}
		}
	}

	if custom != nil {
		env.Headers = custom
	}

	return env, nil
}
