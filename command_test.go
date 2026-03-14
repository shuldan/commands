package commands

import (
	"testing"
)

type testCommand struct {
	BaseCommand
	name string
}

func (c testCommand) CommandName() string {
	return c.name
}

func newTestCommand(name, key string) testCommand {
	return testCommand{
		BaseCommand: BaseCommand{IdemKey: key},
		name:        name,
	}
}

func TestBaseCommand_IdempotencyKey_WithExplicitKey(t *testing.T) {
	t.Parallel()

	cmd := BaseCommand{IdemKey: "my-key-123"}
	got := cmd.IdempotencyKey()

	if got != "my-key-123" {
		t.Errorf("expected 'my-key-123', got %q", got)
	}
}

func TestBaseCommand_IdempotencyKey_GeneratesUUID(t *testing.T) {
	t.Parallel()

	cmd := BaseCommand{}
	got := cmd.IdempotencyKey()

	if got == "" {
		t.Error("expected non-empty generated UUID, got empty string")
	}
}

func TestBaseCommand_IdempotencyKey_UniquePerCall(t *testing.T) {
	t.Parallel()

	cmd := BaseCommand{}
	first := cmd.IdempotencyKey()
	second := cmd.IdempotencyKey()

	if first == second {
		t.Errorf("expected unique UUIDs per call, got same: %q", first)
	}
}

func TestBaseCommand_IdempotencyKey_EmptyStringGenerates(t *testing.T) {
	t.Parallel()

	cmd := BaseCommand{IdemKey: ""}
	got := cmd.IdempotencyKey()

	if got == "" {
		t.Error("expected generated UUID for empty IdemKey, got empty")
	}

	if len(got) < 32 {
		t.Errorf("expected UUID-length string, got %q (len=%d)", got, len(got))
	}
}
