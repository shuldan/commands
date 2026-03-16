package json

import "encoding/json"

// Codec is a JSON implementation of commands.Codec.
type Codec struct{}

// New creates a new JSON codec.
func New() *Codec {
	return &Codec{}
}

// Encode serializes a value to JSON bytes.
func (c *Codec) Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Decode deserializes JSON bytes into a value.
func (c *Codec) Decode(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
