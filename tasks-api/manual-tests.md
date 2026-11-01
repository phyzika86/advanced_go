# Ручное тестирование API через curl

Запустить сервер в первом терминале:

```bash
go run ./cmd/server
```

Все команды ниже выполняются **во втором терминале**, из Linux/macOS/Git Bash. На Windows можно использовать терминал Git Bash. Выполнять **по порядку на свежем сервере**, чтобы ID = 1. `-i` показывает заголовки и HTTP-код. Во всех ответах есть заголовок `Content-Type: application/json`. Дата `created_at` генерируется во время запроса, поэтому её значение приведено условно.

## Сценарии для каждого обязательного эндпоинта

### GET /tasks

**Успешный:**

```bash
curl -i http://localhost:8080/tasks
```

Ожидается `HTTP/1.1 200 OK`, тело:

```json
[]
```

**Ошибочный (метод не поддерживается):**

```bash
curl -i -X PATCH http://localhost:8080/tasks
```

Ожидается `HTTP/1.1 405 Method Not Allowed` и `Allow: GET, POST`, тело:

```json
{"error":"метод не поддерживается"}
```

### POST /tasks

**Успешный:**

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"Купить молоко","done":false}'
```

Ожидается `HTTP/1.1 201 Created`, заголовок `Location: /tasks/1`, тело вида:

```json
{"id":1,"title":"Купить молоко","done":false,"created_at":"2026-11-01T08:30:00Z"}
```

**Ошибочный (пустой title):**

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: application/json' \
  -d '{"title":"   ","done":false}'
```

Ожидается `HTTP/1.1 400 Bad Request`, тело:

```json
{"error":"поле title обязательно и не должно быть пустым"}
```

### GET /tasks/{id}

**Успешный:**

```bash
curl -i http://localhost:8080/tasks/1
```

Ожидается `HTTP/1.1 200 OK` и задача с `id = 1`, `title = "Купить молоко"`, `done = false`.

**Ошибочный (задачи нет):**

```bash
curl -i http://localhost:8080/tasks/999
```

Ожидается `HTTP/1.1 404 Not Found`, тело:

```json
{"error":"задача не найдена"}
```

### PUT /tasks/{id}

**Успешный:**

```bash
curl -i -X PUT http://localhost:8080/tasks/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Купить хлеб","done":true}'
```

Ожидается `HTTP/1.1 200 OK`, тело вида (дата остаётся прежней):

```json
{"id":1,"title":"Купить хлеб","done":true,"created_at":"2026-11-01T08:30:00Z"}
```

**Ошибочный (в PUT отсутствует обязательное поле done):**

```bash
curl -i -X PUT http://localhost:8080/tasks/1 \
  -H 'Content-Type: application/json' \
  -d '{"title":"Новая задача"}'
```

Ожидается `HTTP/1.1 400 Bad Request`, тело:

```json
{"error":"поле done обязательно для PUT"}
```

### DELETE /tasks/{id}

**Успешный:**

```bash
curl -i -X DELETE http://localhost:8080/tasks/1
```

Ожидается `HTTP/1.1 204 No Content` с **пустым телом ответа**. Это соответствует протоколу HTTP.

**Ошибочный (повторно удаляем ту же задачу):**

```bash
curl -i -X DELETE http://localhost:8080/tasks/1
```

Ожидается `HTTP/1.1 404 Not Found`, тело:

```json
{"error":"задача не найдена"}
```

## Дополнительные проверки

**GET /health (PRO):**

```bash
curl -i http://localhost:8080/health
```

Ожидается `HTTP/1.1 200 OK`, тело `{"status":"ok"}`.

**Некорректный ID:**

```bash
curl -i http://localhost:8080/tasks/abc
```

Ожидается `400`, тело `{"error":"id должен быть положительным целым числом"}`.

**Неверный Content-Type:**

```bash
curl -i -X POST http://localhost:8080/tasks \
  -H 'Content-Type: text/plain' \
  -d '{"title":"Задача"}'
```

Ожидается `400`, тело `{"error":"Content-Type должен быть application/json"}`.

**Неизвестный маршрут:**

```bash
curl -i http://localhost:8080/no-such-route
```

Ожидается `404`, тело `{"error":"маршрут не найден"}`.

## Фактический результат проверки

Сервер дополнительно запущен и проверен через настоящий `curl` **1 ноября 2026 года**, с портом `18080` (порт `8080` в проверочной среде был занят). Получены HTTP-коды:

| Проверенный запрос | Фактический код |
|---|---|
| `GET /health` | `200` |
| `GET /tasks` (пустой список) | `200` |
| `POST /tasks` (корректный JSON) | `201` |
| `PUT /tasks/1` (корректный JSON) | `200` |
| `GET /tasks/1` | `200` |
| `DELETE /tasks/1` | `204` (пустое тело) |
| `GET /tasks/1` после удаления | `404` (`{"error":"задача не найдена"}`) |
| `POST /tasks` с пустым `title` | `400` (`{"error":"поле title обязательно и не должно быть пустым"}`) |

Остальные ошибочные сценарии проверяются автоматическими HTTP-тестами, в том числе `405` для неподдерживаемого метода.

Автоматические тесты из `internal/handlers/tasks_test.go` и `internal/storage/memory/memory_test.go` покрывают перечисленные сценарии, проверяют их JSON и HTTP-коды. Для запуска:

```bash
go test ./...
go test -race ./...
```

При ручном запуске `curl` значения `created_at` должны отличаться от примера — это корректно: сервер всегда генерирует текущее время.
