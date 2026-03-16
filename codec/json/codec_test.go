package json

import (
	"testing"
)

type sample struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestCodec_Encode_Success(t *testing.T) {
	t.Parallel()

	c := New()
	data, err := c.Encode(sample{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty data")
	}
}

func TestCodec_Encode_NilValue(t *testing.T) {
	t.Parallel()

	c := New()
	data, err := c.Encode(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "null" {
		t.Fatalf("expected 'null', got '%s'", string(data))
	}
}

func TestCodec_Encode_InvalidValue(t *testing.T) {
	t.Parallel()

	c := New()
	_, err := c.Encode(make(chan int))
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}

func TestCodec_Decode_Success(t *testing.T) {
	t.Parallel()

	c := New()
	var s sample
	err := c.Decode([]byte(`{"name":"test","value":42}`), &s)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "test" || s.Value != 42 {
		t.Fatalf("expected {test 42}, got {%s %d}", s.Name, s.Value)
	}
}

func TestCodec_Decode_InvalidJSON(t *testing.T) {
	t.Parallel()

	c := New()
	var s sample
	err := c.Decode([]byte(`not json`), &s)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestCodec_Decode_EmptyData(t *testing.T) {
	t.Parallel()

	c := New()
	var s sample
	err := c.Decode([]byte{}, &s)
	if err == nil {
		t.Fatal("expected error for empty data")
	}
}

func TestCodec_RoundTrip(t *testing.T) {
	t.Parallel()

	c := New()
	original := sample{Name: "round", Value: 99}

	data, err := c.Encode(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded sample
	err = c.Decode(data, &decoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if decoded.Name != original.Name || decoded.Value != original.Value {
		t.Fatalf("expected %+v, got %+v", original, decoded)
	}
}
