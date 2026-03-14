package commands

import (
	"context"
	"testing"
	"time"
)

func TestMemoryIdempotencyStore_ExistsEmpty(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	exists, err := store.Exists(context.Background(), "key1")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists {
		t.Error("expected key not to exist in empty store")
	}
}

func TestMemoryIdempotencyStore_MarkAndExists(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	ctx := context.Background()

	err := store.Mark(ctx, "key1", time.Hour)
	if err != nil {
		t.Fatalf("unexpected error on Mark: %v", err)
	}

	exists, err := store.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("unexpected error on Exists: %v", err)
	}

	if !exists {
		t.Error("expected key to exist after Mark")
	}
}

func TestMemoryIdempotencyStore_Expiration(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	ctx := context.Background()

	err := store.Mark(ctx, "key1", time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error on Mark: %v", err)
	}

	time.Sleep(5 * time.Millisecond)

	exists, err := store.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("unexpected error on Exists: %v", err)
	}

	if exists {
		t.Error("expected expired key to not exist")
	}
}

func TestMemoryIdempotencyStore_Cleanup(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	ctx := context.Background()

	_ = store.Mark(ctx, "expired", time.Millisecond)
	_ = store.Mark(ctx, "valid", time.Hour)

	time.Sleep(5 * time.Millisecond)
	store.Cleanup()

	exists, _ := store.Exists(ctx, "expired")
	if exists {
		t.Error("expected 'expired' key to be cleaned up")
	}

	exists, _ = store.Exists(ctx, "valid")
	if !exists {
		t.Error("expected 'valid' key to still exist")
	}
}

func TestMemoryIdempotencyStore_OverwriteKey(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	ctx := context.Background()

	_ = store.Mark(ctx, "key1", time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_ = store.Mark(ctx, "key1", time.Hour)

	exists, _ := store.Exists(ctx, "key1")
	if !exists {
		t.Error("expected overwritten key to exist")
	}
}

func TestMemoryIdempotencyStore_MultipleKeys(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		key := "key-" + string(rune('a'+i%26))
		_ = store.Mark(ctx, key, time.Hour)
	}

	exists, _ := store.Exists(ctx, "key-a")
	if !exists {
		t.Error("expected 'key-a' to exist")
	}
}

func TestMemoryIdempotencyStore_CleanupEmpty(t *testing.T) {
	t.Parallel()

	store := NewMemoryIdempotencyStore()
	store.Cleanup()
}
