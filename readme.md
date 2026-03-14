# `commands` — Типобезопасная командная шина для Go

[![Go CI](https://github.com/shuldan/commands/workflows/Go%20CI/badge.svg)](https://github.com/shuldan/commands/actions)
[![codecov](https://codecov.io/gh/shuldan/commands/branch/main/graph/badge.svg)](https://codecov.io/gh/shuldan/commands)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Пакет `commands` предоставляет высокопроизводительную командную шину для Go-приложений, построенных по принципам CQRS и DDD. Использует дженерики для типобезопасности на этапе компиляции, поддерживает синхронную и асинхронную обработку, middleware, идемпотентность, упорядоченную доставку и возврат результатов.

---

## Основные возможности

- **Типобезопасность через дженерики** — ошибки несоответствия типов обнаруживаются на этапе компиляции.
- **Структурные и функциональные обработчики** — слушателем может быть структура с методом `Handle` или обычная функция.
- **Middleware** — цепочки сквозной логики: логирование, трассировка, валидация, транзакции.
- **Идемпотентность** — встроенная дедупликация команд по ключу с настраиваемым TTL и хранилищем.
- **Упорядоченная доставка** — команды с одинаковым ключом идемпотентности обрабатываются последовательно.
- **Результаты команд** — обработчики возвращают типизированные результаты с возможностью подписки.
- **Graceful shutdown** — корректное завершение с поддержкой таймаута через контекст.
- **Observability** — интерфейс `MetricsCollector` для сбора метрик обработки.
- **Тестируемость** — интерфейсы для подмены в тестах.

---

## Установка

Требуется **Go 1.24+**.

```sh
go get github.com/shuldan/commands
```

---

## Быстрый старт

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/shuldan/commands"
)

type CreateOrder struct {
    commands.BaseCommand
    OrderID string
    UserID  string
    Amount  float64
}

func (c CreateOrder) CommandName() string { return "order.create" }

type CreateOrderResult struct {
    commands.BaseResult
    OrderID string
}

func main() {
    bus := commands.New(commands.WithSyncMode())
    defer bus.Close(context.Background())

    commands.HandleFn(bus, func(ctx context.Context, cmd CreateOrder) (commands.Result, error) {
        fmt.Printf("Creating order %s for user %s, amount: %.2f\n",
            cmd.OrderID, cmd.UserID, cmd.Amount)
        return CreateOrderResult{
            BaseResult: commands.BaseResult{Name: "order.created"},
            OrderID:    cmd.OrderID,
        }, nil
    })

    err := bus.Send(context.Background(), CreateOrder{
        OrderID: "order-1",
        UserID:  "user-42",
        Amount:  199.90,
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

---

## Определение команд

Каждая команда реализует интерфейс `Command`:

```go
type Command interface {
    CommandName() string
    IdempotencyKey() string
}
```

Для удобства предусмотрена базовая структура `BaseCommand`:

```go
type CreateOrder struct {
    commands.BaseCommand
    OrderID string
    UserID  string
    Amount  float64
}

func (c CreateOrder) CommandName() string { return "order.create" }
```

`BaseCommand` автоматически генерирует UUID в качестве ключа идемпотентности, если он не задан явно.

---

## Обработчики команд

### Структурный обработчик

```go
type CreateOrderHandler struct {
    repo OrderRepository
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder) (commands.Result, error) {
    order, err := h.repo.Create(ctx, cmd.OrderID, cmd.UserID, cmd.Amount)
    if err != nil {
        return nil, err
    }
    return CreateOrderResult{OrderID: order.ID}, nil
}

commands.Handle(bus, &CreateOrderHandler{repo: repo})
```

### Функциональный обработчик

```go
commands.HandleFn(bus, func(ctx context.Context, cmd CreateOrder) (commands.Result, error) {
    return CreateOrderResult{OrderID: cmd.OrderID}, nil
})
```

---

## Результаты команд

Обработчики команд возвращают типизированные результаты:

```go
type Result interface {
    ResultName() string
}
```

Подписка на результат:

```go
commands.OnResultFn[CreateOrderResult](bus, "order.create",
    func(ctx context.Context, result CreateOrderResult, err error) error {
        if err != nil {
            log.Printf("order creation failed: %v", err)
            return nil
        }
        log.Printf("order created: %s", result.OrderID)
        return nil
    },
)
```

---

## Идемпотентность

Встроенная дедупликация по ключу идемпотентности:

```go
bus := commands.New(
    commands.WithIdempotencyTTL(1 * time.Hour),
    commands.WithIdempotencyStore(commands.NewMemoryIdempotencyStore()),
)
```

Для production рекомендуется реализовать `IdempotencyStore` на базе Redis или БД:

```go
type IdempotencyStore interface {
    Exists(ctx context.Context, key string) (bool, error)
    Mark(ctx context.Context, key string, ttl time.Duration) error
}
```

---

## Конфигурация

```go
bus := commands.New(
    commands.WithAsyncMode(),
    commands.WithWorkerCount(8),
    commands.WithBufferSize(100),
    commands.WithPublishTimeout(3*time.Second),
    commands.WithOrderedDelivery(),
    commands.WithMiddleware(mw1, mw2),
    commands.WithPanicHandler(ph),
    commands.WithErrorHandler(eh),
    commands.WithMetrics(collector),
    commands.WithIdempotencyStore(store),
    commands.WithIdempotencyTTL(1*time.Hour),
)
```

### Режимы

**Синхронный** — команды обрабатываются в горутине вызывающего:

```go
bus := commands.New(commands.WithSyncMode())
```

**Асинхронный** — команды обрабатываются пулом воркеров:

```go
bus := commands.New(commands.WithAsyncMode(), commands.WithWorkerCount(4))
```

---

## Middleware

```go
type Middleware func(next HandleFunc) HandleFunc
```

### Пример: логирование

```go
func LoggingMiddleware() commands.Middleware {
    return func(next commands.HandleFunc) commands.HandleFunc {
        return func(ctx context.Context, cmd commands.Command) (commands.Result, error) {
            slog.InfoContext(ctx, "command started", "command", cmd.CommandName())
            start := time.Now()
            result, err := next(ctx, cmd)
            slog.InfoContext(ctx, "command finished",
                "command", cmd.CommandName(),
                "duration", time.Since(start),
                "error", err,
            )
            return result, err
        }
    }
}
```

---

## Обработка ошибок и паник

```go
type ErrorHandler interface {
    Handle(ctx ErrorContext)
}

type PanicHandler interface {
    Handle(ctx PanicContext)
}
```

---

## Observability

```go
type MetricsCollector interface {
    CommandDispatched(commandName string)
    CommandHandled(commandName string, duration time.Duration, err error)
    CommandDropped(commandName string, reason string)
    CommandDuplicate(commandName string)
    QueueDepth(depth int)
}
```

---

## Graceful shutdown

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := bus.Close(ctx); err != nil {
    slog.Error("shutdown timed out", "error", err)
}
```

---

## Справочник API

### Создание и управление

| Функция / Метод | Описание |
|---|---|
| `commands.New(opts ...Option) *Dispatcher` | Создаёт диспетчер |
| `bus.Send(ctx, cmd) error` | Отправляет команду |
| `bus.Close(ctx) error` | Graceful shutdown |

### Регистрация обработчиков

| Функция | Описание |
|---|---|
| `commands.Handle[C](bus, handler) error` | Структурный обработчик |
| `commands.HandleFn[C](bus, fn) error` | Функциональный обработчик |
| `commands.OnResult[R](bus, name, handler) error` | Обработчик результата |
| `commands.OnResultFn[R](bus, name, fn) error` | Функция-обработчик результата |

### Опции

| Опция | Описание | По умолчанию |
|---|---|---|
| `WithAsyncMode()` | Асинхронная обработка | Включено |
| `WithSyncMode()` | Синхронная обработка | — |
| `WithWorkerCount(n)` | Количество воркеров | `runtime.NumCPU()` |
| `WithBufferSize(n)` | Размер буфера | `workerCount * 10` |
| `WithPublishTimeout(d)` | Таймаут отправки | `5s` |
| `WithOrderedDelivery()` | Упорядоченная доставка | Выключено |
| `WithMiddleware(mw...)` | Middleware | — |
| `WithPanicHandler(h)` | Обработчик паник | slog |
| `WithErrorHandler(h)` | Обработчик ошибок | slog |
| `WithMetrics(m)` | Метрики | No-op |
| `WithIdempotencyStore(s)` | Хранилище идемпотентности | In-memory |
| `WithIdempotencyTTL(d)` | TTL ключей | `24h` |

---

## Работа с проектом

```sh
make install-tools
make all
make ci
```

---

## Лицензия

[MIT](LICENSE)

---

> **Автор**: MSeytumerov
> **Репозиторий**: `github.com/shuldan/commands`
> **Go**: `1.24+`
