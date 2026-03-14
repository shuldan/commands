package commands

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	expiresAt time.Time
}

// MemoryIdempotencyStore — in-memory реализация IdempotencyStore.
type MemoryIdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

// NewMemoryIdempotencyStore создаёт in-memory store.
func NewMemoryIdempotencyStore() *MemoryIdempotencyStore {
	return &MemoryIdempotencyStore{
		entries: make(map[string]memoryEntry),
	}
}

// Exists проверяет наличие ключа.
func (s *MemoryIdempotencyStore) Exists(
	_ context.Context, key string,
) (bool, error) {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()

	if !ok {
		return false, nil
	}

	if time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()

		return false, nil
	}

	return true, nil
}

// Mark записывает ключ с TTL.
func (s *MemoryIdempotencyStore) Mark(
	_ context.Context, key string, ttl time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = memoryEntry{
		expiresAt: time.Now().Add(ttl),
	}

	return nil
}

// Cleanup удаляет просроченные записи.
func (s *MemoryIdempotencyStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}
