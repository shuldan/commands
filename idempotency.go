package commands

import (
	"context"
	"time"
)

// IdempotencyStore хранит ключи идемпотентности для дедупликации команд.
type IdempotencyStore interface {
	// Exists проверяет, была ли команда с таким ключом уже обработана.
	Exists(ctx context.Context, key string) (bool, error)
	// Mark помечает ключ как обработанный с заданным TTL.
	Mark(ctx context.Context, key string, ttl time.Duration) error
}
