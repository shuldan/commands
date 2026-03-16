package commands

// Codec handles serialization and deserialization of commands and results.
type Codec interface {
	Encode(v any) ([]byte, error)
	Decode(data []byte, v any) error
}
