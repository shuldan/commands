package commands

import "github.com/google/uuid"

// Command — контракт команды.
type Command interface {
	CommandName() string
	IdempotencyKey() string
}

// BaseCommand — базовая структура для встраивания в конкретные команды.
type BaseCommand struct {
	IdemKey string `json:"idempotency_key,omitempty"`
}

// IdempotencyKey возвращает ключ идемпотентности.
// Если не задан — генерирует UUID.
func (b BaseCommand) IdempotencyKey() string {
	if b.IdemKey != "" {
		return b.IdemKey
	}

	return uuid.New().String()
}
