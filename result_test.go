package commands

import "testing"

func TestBaseResult_ResultName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    BaseResult
		expected string
	}{
		{"non_empty", BaseResult{Name: "order.created"}, "order.created"},
		{"empty", BaseResult{Name: ""}, ""},
		{"special_chars", BaseResult{Name: "a.b-c_d"}, "a.b-c_d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.input.ResultName()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestBaseResult_ImplementsResult(t *testing.T) {
	t.Parallel()

	var _ Result = BaseResult{Name: "test"}
}
