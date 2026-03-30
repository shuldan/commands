# `commands` — Асинхронная командная шина с типобезопасным request/reply для Go

[![Go CI](https://github.com/shuldan/commands/workflows/Go%20CI/badge.svg)](https://github.com/shuldan/commands/actions)
[![codecov](https://codecov.io/gh/shuldan/commands/branch/main/graph/badge.svg)](https://codecov.io/gh/shuldan/commands)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

Пакет `commands` предоставляет командную шину с асинхронным request/reply для Go-приложений. Команда отправляется
клиентом, обрабатывается сервером, а результат доставляется обратно через `Future`. Ответ может быть отправлен
как синхронно (в момент обработки), так и асинхронно — позже, из другой горутины или даже другого процесса.
Использует дженерики для типобезопасности на этапе компиляции, абстрагируется от конкретного транспорта
(Kafka, NATS, RabbitMQ, in-memory).

---

## Основные возможности

- **Типобезопасность через дженерики** — команды, результаты и хендлеры типизированы на этапе компиляции.
- **Асинхронный request/reply** — клиент получает `Future`, результат доставляется когда будет готов.
- **Отложенный ответ** — сервер может отправить ответ позже, из другой горутины или процесса, через `ReplySender`.
- **Разделение клиента и сервера** — отправитель и обработчик независимы, могут жить в разных процессах.
- **Абстракция транспорта** — ядро не зависит от брокера сообщений, маршрутизация и топология — ответственность транспорта.
- **Таймауты на каждый запрос** — дефолтный таймаут на клиенте, override на уровне отдельной команды.
- **Graceful shutdown** — сервер дожидается завершения активных хендлеров, клиент отменяет pending futures.
- **Тестируемость** — интерфейс `CommandSender` для мокирования в тестах.

---

## Установка

Требуется **Go 1.24+**.

```sh
go get github.com/shuldan/commands
```

### Транспорты

```sh
# In-memory (для тестов и single-process)
# Входит в основной модуль, дополнительная установка не требуется.

# Kafka
go get github.com/shuldan/commands/transport/kafka
```

---

## Быстрый старт: синхронный ответ

Простейший сценарий — хендлер обрабатывает команду и отвечает сразу.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shuldan/commands"
	jsoncodec "github.com/shuldan/commands/codec/json"
	"github.com/shuldan/commands/transport/memory"
)

// --- Команда и результат ---

type CreateOrder struct {
	OrderID string  `json:"order_id"`
	UserID  string  `json:"user_id"`
	Amount  float64 `json:"amount"`
}

func (c CreateOrder) CommandName() string { return "orders.create" }

type CreateOrderResult struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func (r CreateOrderResult) ResultName() string { return "orders.create.result" }

// --- Хендлер с синхронным ответом ---

type CreateOrderHandler struct{}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder, reply commands.ReplySender) error {
	fmt.Printf("Creating order %s for user %s, amount: %.2f\n", cmd.OrderID, cmd.UserID, cmd.Amount)

	// Обработка и ответ в одном вызове.
	return reply.Send(ctx, CreateOrderResult{
		OrderID: cmd.OrderID,
		Status:  "created",
	})
}

func main() {
	transport := memory.New()
	codec := jsoncodec.New()

	// Создаём и запускаем сервер.
	server, _ := commands.NewCommandServer(transport, codec)
	commands.Register[CreateOrder](server, &CreateOrderHandler{})
	server.Open(context.Background())
	defer server.Close(context.Background())

	// Создаём и запускаем клиент.
	client, _ := commands.NewCommandClient(transport, codec,
		commands.WithTimeout(5*time.Second),
	)
	client.Open(context.Background())
	defer client.Close(context.Background())

	// Отправляем команду и ждём результат.
	future, err := commands.Send[CreateOrderResult](context.Background(), client, CreateOrder{
		OrderID: "order-1",
		UserID:  "user-42",
		Amount:  199.90,
	})
	if err != nil {
		log.Fatal(err)
	}

	result, err := future.Await(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Order %s status: %s\n", result.OrderID, result.Status)
}
```

---

## Быстрый старт: Kafka

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shuldan/commands"
	jsoncodec "github.com/shuldan/commands/codec/json"
	kafkatransport "github.com/shuldan/commands/transport/kafka"
)

func main() {
	// --- Серверная сторона (обработчик команд) ---

	serverTransport, err := kafkatransport.New(kafkatransport.Config{
		Brokers:          []string{"localhost:9092"},
		CommandTopic:     "orders.commands",
		ReplyTopic:       "orders.replies",
		ConsumerGroup:    "orders-service",
		AutoCreateTopics: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	codec := jsoncodec.New()

	server, _ := commands.NewCommandServer(serverTransport, codec)
	commands.Register[CreateOrder](server, &CreateOrderHandler{})
	server.Open(context.Background())
	defer server.Close(context.Background())

	// --- Клиентская сторона (отправитель команд) ---

	clientTransport, err := kafkatransport.New(kafkatransport.Config{
		Brokers:          []string{"localhost:9092"},
		CommandTopic:     "orders.commands",
		ReplyTopic:       "orders.replies",
		AutoCreateTopics: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	client, _ := commands.NewCommandClient(clientTransport, codec,
		commands.WithTimeout(10*time.Second),
	)
	client.Open(context.Background())
	defer client.Close(context.Background())

	future, err := commands.Send[CreateOrderResult](context.Background(), client, CreateOrder{
		OrderID: "order-1",
		UserID:  "user-42",
		Amount:  199.90,
	})
	if err != nil {
		log.Fatal(err)
	}

	result, err := future.Await(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Order %s status: %s\n", result.OrderID, result.Status)
}
```

---

## Асинхронный ответ (отложенная обработка)

Главная особенность библиотеки — ответ на команду не обязан отправляться в момент обработки.
Хендлер принимает команду и `ReplySender`. Через `reply.Address()` можно получить сериализуемый
`ReplyAddress`, сохранить его в БД и отправить ответ позже — когда результат будет готов.

```go
// --- Хендлер: принимает команду, сохраняет адрес в БД ---

type CreateOrderHandler struct {
	repo OrderRepository
}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd CreateOrder, reply commands.ReplySender) error {
	// Сохраняем команду и адрес для ответа.
	// ReplyAddress сериализуемый — можно хранить в БД.
	return h.repo.Save(ctx, Order{
		ID:           cmd.OrderID,
		UserID:       cmd.UserID,
		Amount:       cmd.Amount,
		ReplyAddress: reply.Address(),
		Status:       "pending_approval",
	})
	// return nil означает "команда принята, ответ будет позже"
}

// --- Воркер: позже отправляет ответ ---

type OrderApprovalWorker struct {
	repo      OrderRepository
	transport commands.Transport
	codec     commands.Codec
}

func (w *OrderApprovalWorker) OnApproved(ctx context.Context, orderID string) error {
	order, err := w.repo.Get(ctx, orderID)
	if err != nil {
		return err
	}

	// Создаём ReplySender из сохранённого адреса.
	reply := commands.NewReplySender(w.transport, w.codec, order.ReplyAddress)

	// Отправляем ответ клиенту, который ждёт через Future.
	return reply.Send(ctx, CreateOrderResult{
		OrderID: order.ID,
		Status:  "approved",
	})
}

func (w *OrderApprovalWorker) OnRejected(ctx context.Context, orderID string, reason string) error {
	order, err := w.repo.Get(ctx, orderID)
	if err != nil {
		return err
	}

	reply := commands.NewReplySender(w.transport, w.codec, order.ReplyAddress)

	return reply.SendError(ctx, &commands.ErrorPayload{
		Code:    "ORDER_REJECTED",
		Message: reason,
	})
}
```

---

## Определение команд и результатов

Каждая команда реализует интерфейс `Command`, каждый результат — интерфейс `Result`:

```go
type Command interface {
    CommandName() string
}

type Result interface {
    ResultName() string
}
```

### Пример

```go
type ChargePayment struct {
    PaymentID string  `json:"payment_id"`
    OrderID   string  `json:"order_id"`
    Amount    float64 `json:"amount"`
    Currency  string  `json:"currency"`
}

func (c ChargePayment) CommandName() string { return "payments.charge" }

type ChargePaymentResult struct {
    PaymentID     string `json:"payment_id"`
    TransactionID string `json:"transaction_id"`
    Status        string `json:"status"`
}

func (r ChargePaymentResult) ResultName() string { return "payments.charge.result" }
```

---

## Обработчики команд

Любая структура с методом `Handle(ctx, cmd, ReplySender) error` реализует интерфейс `Handler[C]`:

```go
type Handler[C Command] interface {
    Handle(ctx context.Context, cmd C, reply ReplySender) error
}
```

Хендлер получает команду и `ReplySender`, который позволяет:

| Сценарий | Что делает хендлер |
|---|---|
| Синхронный ответ | Вызывает `reply.Send(ctx, result)` и возвращает `nil` |
| Асинхронный ответ | Сохраняет `reply.Address()` в БД, возвращает `nil`. Ответ отправляется позже. |
| Бизнес-ошибка | Вызывает `reply.SendError(ctx, err)` и возвращает `nil` |
| Инфраструктурный сбой | Возвращает `error`. Клиент получит `ErrInternal`. |

### ReplySender

```go
type ReplySender interface {
    Send(ctx context.Context, result Result) error
    SendError(ctx context.Context, err error) error
    Address() ReplyAddress
}
```

| Метод | Назначение |
|---|---|
| `reply.Send(ctx, result)` | Отправить успешный результат клиенту |
| `reply.SendError(ctx, err)` | Отправить бизнес-ошибку клиенту |
| `reply.Address()` | Получить сериализуемый адрес для отложенного ответа |

### ReplyAddress — сериализуемая структура

```go
type ReplyAddress struct {
    CorrelationID string `json:"correlation_id"`
    ReplyTo       string `json:"reply_to"`
}
```

`ReplyAddress` можно сохранить в БД, передать в очередь, использовать из другого процесса.
Для отправки отложенного ответа создайте `ReplySender` из сохранённого адреса:

```go
reply := commands.NewReplySender(transport, codec, savedAddress)
reply.Send(ctx, result)
```

### Семантика ошибок хендлера

| Канал | Семантика | Пример |
|---|---|---|
| `reply.Send(ctx, result)` | Успешный результат | Заказ создан |
| `reply.SendError(ctx, err)` | Бизнес-ошибка | Недостаточно средств |
| `return error` из `Handle()` | Инфраструктурный сбой | БД недоступна |

При инфраструктурном сбое (`return error`) сервер автоматически отправляет клиенту `ErrInternal` —
детали ошибки не утекают.

При `return nil` без отправки reply — ответ ожидается позже. Ответственность за отправку reply
лежит на разработчике. Клиент защищён таймаутом.

---

## Клиент — отправка команд

### Нетипизированная отправка

```go
future, err := client.Send(ctx, cmd)
if err != nil {
    // ошибка отправки
}

result, err := future.Await(ctx)
```

### Типобезопасная отправка

```go
future, err := commands.Send[CreateOrderResult](ctx, client, cmd)
if err != nil {
    // ошибка отправки
}

result, err := future.Await(ctx) // result — CreateOrderResult, не Result
```

### Future

`Future` представляет отложенный результат команды:

```go
type Future interface {
    Await(ctx context.Context) (Result, error)
    Done() <-chan struct{}
    Result() (Result, error, bool)
}

type TypedFuture[R Result] interface {
    Await(ctx context.Context) (R, error)
    Done() <-chan struct{}
    Result() (R, error, bool)
}
```

| Метод | Описание |
|---|---|
| `Await(ctx)` | Блокирует до получения результата или отмены контекста |
| `Done()` | Канал, закрывается при получении результата |
| `Result()` | Неблокирующая проверка — третье значение `true` если результат готов |

---

## Конфигурация

### Клиент

```go
client, err := commands.NewCommandClient(transport, codec,
    commands.WithTimeout(30*time.Second),
    commands.WithClientLogger(slog.Default()),
)
```

### Сервер

```go
server, err := commands.NewCommandServer(transport, codec,
    commands.WithServerLogger(slog.Default()),
)
```

### Опции отправки

```go
future, err := commands.Send[R](ctx, client, cmd,
    commands.WithSendTimeout(5*time.Second),
)
```

### Справочник опций

| Опция | Описание | По умолчанию |
|---|---|---|
| `WithTimeout(d)` | Дефолтный таймаут ожидания ответа на клиенте | `30s` |
| `WithClientLogger(l)` | Логгер клиента | No-op |
| `WithServerLogger(l)` | Логгер сервера | No-op |
| `WithSendTimeout(d)` | Таймаут для конкретного запроса | Значение `WithTimeout` |

---

## Транспорт

Библиотека абстрагируется от конкретного брокера через интерфейс `Transport`:

```go
type Transport interface {
    Send(ctx context.Context, env CommandEnvelope) error
    Subscribe(ctx context.Context, handler CommandHandler) error
    Reply(ctx context.Context, env ReplyEnvelope) error
    SubscribeReplies(ctx context.Context, handler ReplyHandler) error
    ReplyAddress() string
    Close(ctx context.Context) error
}
```

Маршрутизация команд и стратегия reply-очередей — ответственность транспорта. Ядро библиотеки передаёт
`CommandName` в envelope, транспорт решает, куда отправить.

### In-Memory транспорт

Для тестов и single-process использования:

```go
import "github.com/shuldan/commands/transport/memory"

transport := memory.New()
```

### Kafka транспорт

Для распределённых систем на базе Apache Kafka:

```go
import kafkatransport "github.com/shuldan/commands/transport/kafka"

transport, err := kafkatransport.New(kafkatransport.Config{
    Brokers:       []string{"localhost:9092", "localhost:9093"},
    CommandTopic:  "orders.commands",
    ReplyTopic:    "orders.replies",
    ConsumerGroup: "orders-service",  // обязателен для серверной стороны
})
```

#### Конфигурация Kafka транспорта

| Параметр | Описание | По умолчанию |
|---|---|---|
| `Brokers` | Адреса Kafka-брокеров | Обязателен |
| `CommandTopic` | Топик для команд | Обязателен |
| `ReplyTopic` | Топик для ответов | Обязателен |
| `ConsumerGroup` | Consumer group для серверной стороны | Обязателен для `Subscribe` |
| `AutoCreateTopics` | Автоматическое создание топиков | `false` |
| `NumPartitions` | Количество партиций (при автосоздании) | `1` |
| `ReplicationFactor` | Фактор репликации (при автосоздании) | `1` |
| `MaxBytes` | Максимальный размер batch от consumer | `1MB` |
| `CommitInterval` | Интервал коммита офсетов (`0` = синхронный) | `0` |
| `WriteTimeout` | Таймаут записи producer | `10s` |

#### Топология

- **Один command-топик** — все клиенты пишут команды в один топик, серверы читают в одной consumer group.
- **Один reply-топик** — каждый клиентский инстанс создаёт уникальную consumer group для получения всех ответов, фильтрация по `correlation_id` происходит автоматически.
- Масштабирование серверов — добавление инстансов в consumer group, Kafka распределяет партиции.
- Если нужна изоляция по типам команд — создайте несколько транспортов с разными топиками.

#### Автосоздание топиков

По умолчанию выключено. Если топик не существует — транспорт возвращает ошибку при создании.
Для dev-окружения можно включить:

```go
transport, err := kafkatransport.New(kafkatransport.Config{
    Brokers:           []string{"localhost:9092"},
    CommandTopic:      "orders.commands",
    ReplyTopic:        "orders.replies",
    AutoCreateTopics:  true,
    NumPartitions:     3,
    ReplicationFactor: 1,
})
```

В production рекомендуется создавать топики через IaC (Terraform, Helm) с нужными параметрами.

#### Сериализация в Kafka

Метаданные envelope (`message_id`, `correlation_id`, `command_name`, `reply_to`, `result_name`, `error`) передаются в Kafka Headers. Payload команды или результата — в Value сообщения. Пользовательские заголовки из `Headers` map передаются с префиксом `x-`.

#### Гарантии доставки

At-least-once. Если ответ потерян — клиент получит `ErrTimeout`. Идемпотентность — ответственность бизнес-логики.

---

## Сериализация

Библиотека использует интерфейс `Codec` для сериализации команд и результатов:

```go
type Codec interface {
    Encode(v any) ([]byte, error)
    Decode(data []byte, v any) error
}
```

### JSON кодек

```go
import jsoncodec "github.com/shuldan/commands/codec/json"

codec := jsoncodec.New()
```

---

## Обработка ошибок

### Бизнес-ошибки

`ErrorPayload` реализует интерфейс `error` и доставляется клиенту через `Future`:

```go
type ErrorPayload struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e *ErrorPayload) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
```

На стороне клиента:

```go
result, err := future.Await(ctx)
if err != nil {
    var busErr *commands.ErrorPayload
    if errors.As(err, &busErr) {
        fmt.Println("Business error:", busErr.Code, busErr.Message)
    } else {
        fmt.Println("Infrastructure error:", err)
    }
}
```

### Предопределённые ошибки

| Ошибка | Описание |
|---|---|
| `ErrInternal` | Инфраструктурный сбой сервера (детали скрыты) |
| `ErrCommandNotFound` | Нет зарегистрированного хендлера для команды |
| `ErrTimeout` | Таймаут ожидания ответа |
| `ErrClientClosed` | Клиент закрыт |
| `ErrServerClosed` | Сервер закрыт |
| `ErrAlreadyOpened` | Повторный вызов `Open` |
| `ErrNotOpened` | Вызов `Send`/`Close` до `Open` |
| `ErrAlreadyRegistered` | Дублирование хендлера для одной команды |
| `ErrServerStarted` | Регистрация хендлера после `Open` |

---

## Жизненный цикл

### Порядок вызовов

| | Client | Server |
|---|---|---|
| **New** | Создание объекта | Создание объекта |
| **Register** | — | Регистрация хендлеров |
| **Open** | Подписка на reply-очередь | Подписка на команды |
| **Send / Handle** | Отправка команд | Обработка команд |
| **Close** | Отмена pending futures, отписка | Drain хендлеров, отписка |

`Register` вызывается до `Open`. Вызов `Register` после `Open` возвращает `ErrServerStarted`.

### Graceful shutdown

**Сервер** — `Close(ctx)` прекращает приём новых команд и ожидает завершения активных хендлеров.
Отложенные ответы (сохранённые `ReplyAddress`) продолжают работать после перезапуска сервиса —
достаточно создать `NewReplySender` с тем же транспортом.

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := server.Close(ctx); err != nil {
    // context.DeadlineExceeded — не все хендлеры завершились вовремя
}
```

**Клиент** — `Close(ctx)` отменяет все pending futures с ошибкой `ErrClientClosed`:

```go
if err := client.Close(ctx); err != nil {
    // ошибка закрытия транспорта
}
```

---

## Логирование

Библиотека использует минимальный интерфейс `Logger`, совместимый с `slog.Logger`:

```go
type Logger interface {
    Info(msg string, keysAndValues ...any)
    Warn(msg string, keysAndValues ...any)
    Error(msg string, keysAndValues ...any)
}
```

```go
client, _ := commands.NewCommandClient(transport, codec,
    commands.WithClientLogger(slog.Default()),
)
```

Если логгер не передан — используется no-op реализация.

---

## Тестирование

Интерфейс `CommandSender` позволяет подменять клиент в тестах:

```go
type CommandSender interface {
    Send(ctx context.Context, cmd Command, opts ...SendOption) (Future, error)
    Close(ctx context.Context) error
}
```

### Пример мока

```go
type mockFuture struct {
    result commands.Result
    err    error
}

func (f *mockFuture) Await(_ context.Context) (commands.Result, error) {
    return f.result, f.err
}

func (f *mockFuture) Done() <-chan struct{} {
    ch := make(chan struct{})
    close(ch)
    return ch
}

func (f *mockFuture) Result() (commands.Result, error, bool) {
    return f.result, f.err, true
}

type mockSender struct {
    sentCommands []commands.Command
    future       commands.Future
}

func (m *mockSender) Send(_ context.Context, cmd commands.Command, _ ...commands.SendOption) (commands.Future, error) {
    m.sentCommands = append(m.sentCommands, cmd)
    return m.future, nil
}

func (m *mockSender) Close(_ context.Context) error { return nil }
```

### Интеграционный тест: синхронный ответ

```go
func TestIntegration_SyncReply(t *testing.T) {
    transport := memory.New()
    codec := jsoncodec.New()

    server, _ := commands.NewCommandServer(transport, codec)
    commands.Register[CreateOrder](server, &CreateOrderHandler{})
    server.Open(context.Background())
    defer server.Close(context.Background())

    client, _ := commands.NewCommandClient(transport, codec)
    client.Open(context.Background())
    defer client.Close(context.Background())

    future, _ := commands.Send[CreateOrderResult](context.Background(), client, CreateOrder{
        OrderID: "order-1",
        UserID:  "user-42",
        Amount:  199.90,
    })

    result, err := future.Await(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Status != "created" {
        t.Fatalf("expected 'created', got '%s'", result.Status)
    }
}
```

### Интеграционный тест: асинхронный ответ

```go
func TestIntegration_AsyncReply(t *testing.T) {
    transport := memory.New()
    codec := jsoncodec.New()

    // Хендлер сохраняет адрес, не отвечает сразу.
    addrCh := make(chan commands.ReplyAddress, 1)
    handler := &testHandler{
        handleFunc: func(_ context.Context, cmd CreateOrder, reply commands.ReplySender) error {
            addrCh <- reply.Address()
            return nil
        },
    }

    server, _ := commands.NewCommandServer(transport, codec)
    commands.Register[CreateOrder](server, handler)
    server.Open(context.Background())
    defer server.Close(context.Background())

    client, _ := commands.NewCommandClient(transport, codec,
        commands.WithTimeout(5*time.Second),
    )
    client.Open(context.Background())
    defer client.Close(context.Background())

    future, _ := commands.Send[CreateOrderResult](context.Background(), client, CreateOrder{
        OrderID: "order-1",
    })

    // Ответ ещё не готов.
    _, _, ready := future.Result()
    if ready {
        t.Fatal("expected result not ready yet")
    }

    // Получаем сохранённый адрес.
    savedAddr := <-addrCh

    // Позже — отправляем ответ из сохранённого адреса.
    reply := commands.NewReplySender(transport, codec, savedAddr)
    reply.Send(context.Background(), CreateOrderResult{
        OrderID: "order-1",
        Status:  "approved",
    })

    result, err := future.Await(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Status != "approved" {
        t.Fatalf("expected 'approved', got '%s'", result.Status)
    }
}
```

---

## Справочник API

### Создание и управление

| Функция / Метод | Описание |
|---|---|
| `commands.NewCommandClient(transport, codec, opts...)` | Создаёт клиент |
| `commands.NewCommandServer(transport, codec, opts...)` | Создаёт сервер |
| `commands.NewReplySender(transport, codec, addr)` | Создаёт `ReplySender` для отложенного ответа |
| `client.Open(ctx)` | Подписка на reply-очередь |
| `server.Open(ctx)` | Подписка на команды |
| `client.Close(ctx)` | Отмена pending futures, закрытие транспорта |
| `server.Close(ctx)` | Drain хендлеров, закрытие транспорта |

### Отправка команд

| Функция / Метод | Описание |
|---|---|
| `client.Send(ctx, cmd, opts...)` | Нетипизированная отправка |
| `commands.Send[R](ctx, client, cmd, opts...)` | Типобезопасная отправка |

### Отправка ответов

| Метод | Описание |
|---|---|
| `reply.Send(ctx, result)` | Отправить успешный результат |
| `reply.SendError(ctx, err)` | Отправить бизнес-ошибку |
| `reply.Address()` | Получить сериализуемый адрес для отложенного ответа |

### Регистрация хендлеров

| Функция | Описание |
|---|---|
| `commands.Register[C](server, handler)` | Регистрация типизированного хендлера |

---

## Архитектура

```
┌───────────────┐         CommandEnvelope           ┌─────────────────┐
│   Client      │ ───────────────────────────────▶ │     Server      │
│               │                                   │                 │
│ CommandClient │                                   │ CommandServer   │
│               │                                   │ Handler[C]      │
│   Future ◀───┤                                   │                 │
└──────┬────────┘                                   └────────┬────────┘
       │                                                     │
       │              ReplyEnvelope                  reply.Send()
       │ ◀──────────────────────────────────────────────────┘
       │                                          (синхронный ответ)
       │
       │                                           reply.Address()
       │                                          (сохраняется в БД)
       │                                                    │
       │              ReplyEnvelope                         ▼
       │ ◀──────────────────────────────── NewReplySender(addr)
       │                                     (отложенный ответ)
       │
       ▼
  ┌──────────────────────────────────────────────────────────────┐
  │                         Transport                            │
  │            (Kafka, NATS, RabbitMQ, In-Memory, ...)           │
  └──────────────────────────────────────────────────────────────┘
```

---

## Работа с проектом

### Запуск тестов

```sh
go test ./...
```

### Запуск тестов с race detector

```sh
go test -race ./...
```

### Запуск Kafka для интеграционных тестов

```sh
docker run -d --name kafka \
  -p 9092:9092 \
  -e KAFKA_CFG_NODE_ID=0 \
  -e KAFKA_CFG_PROCESS_ROLES=controller,broker \
  -e KAFKA_CFG_LISTENERS=PLAINTEXT://:9092,CONTROLLER://:9093 \
  -e KAFKA_CFG_LISTENER_SECURITY_PROTOCOL_MAP=CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT \
  -e KAFKA_CFG_CONTROLLER_QUORUM_VOTERS=0@localhost:9093 \
  -e KAFKA_CFG_CONTROLLER_LISTENER_NAMES=CONTROLLER \
  bitnami/kafka:latest
```

---

## Лицензия

Распространяется под лицензией [MIT](LICENSE).

---

## Вклад в проект

PR и issue приветствуются. Перед отправкой убедитесь, что тесты проходят без ошибок.

---

> **Репозиторий**: `github.com/shuldan/commands`
> **Go**: `1.24+`
