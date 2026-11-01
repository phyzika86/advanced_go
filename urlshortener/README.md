# URL Shortener — HTTP-сервис на Go

Сервис сокращает HTTP/HTTPS-ссылки и перенаправляет по короткому идентификатору на исходный адрес. Данные хранятся в памяти (`map`), доступ защищён `sync.RWMutex`. База данных и внешние зависимости не требуются. Нужен **Go 1.18+**.

## Запуск

В папке проекта:

```bash
go run .
```

Сервер доступен по адресу `http://localhost:8080`. При необходимости порт можно изменить переменной окружения `PORT` (например, в PowerShell: `$env:PORT="8081"; go run .`).

## API

| Метод | Путь | Описание | Ответ |
|---|---|---|---|
| `POST` | `/shorten` | Сокращает URL | `201 Created`, JSON с `short_url` и `original_url` |
| `GET` | `/{short_url}` | Перенаправляет на исходный URL | `302 Found`, заголовок `Location` |

**POST /shorten** — запрос:

```json
{"url":"https://example.com/very/long/path"}
```

Пример ответа (`201`):

```json
{"short_url":"abc123_X","original_url":"https://example.com/very/long/path"}
```

ID генерируется случайным образом и состоит из 8 символов. Для примера выше `GET /abc123_X` возвращает `302 Found` и `Location: https://example.com/very/long/path` (без тела JSON).

Ошибки: `400` — невалидный URL или JSON; `404` — ID не найден; `405` — неподдерживаемый HTTP-метод; `500` — внутренняя ошибка. Для ошибок используется JSON вида `{"error":"сообщение"}`.

## Проверка запросов

В другом терминале (Windows PowerShell):

```powershell
$created = Invoke-RestMethod -Uri 'http://localhost:8080/shorten' -Method Post -ContentType 'application/json' -Body '{"url":"https://example.com/very/long/path"}'
$created
curl.exe -i "http://localhost:8080/$($created.short_url)"
curl.exe -i 'http://localhost:8080/missing1'
```

Вместо PowerShell можно использовать `curl` или Postman. Чтобы увидеть ответ `302`, не включайте автоматическое следование перенаправлениям (`-L`).

## Автоматические тесты

```bash
go test -v ./...
go test -race ./...
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

- `shortener_test.go` — **table-driven** unit-тесты с `t.Run`: проверка URL, создание/поиск ID, коллизии, ошибки генерации и конкурентный доступ.
- `handlers_test.go` — тесты через **httptest**: создание ссылок, редирект `302`, ошибки `400/404/405/500`, некорректный JSON и полный HTTP-сценарий.

На Go 1.23.2 тесты проходят, общее покрытие кода — **90,7%**; функции `Shorten`, `GetOriginal`, `isValidURL` покрыты на **100%**. Минимальное требование задания к покрытию бизнес-логики (80%) выполнено.

## Файлы

```text
urlshortener/
├── main.go             # сервер, маршруты, обработчики HTTP
├── shortener.go        # бизнес-логика и потокобезопасное хранилище
├── shortener_test.go   # unit-тесты
├── handlers_test.go    # HTTP-тесты с httptest
├── go.mod              # Go-модуль
└── README.md           # запуск и инструкция по проверке
```

**Примечание:** содержимое `map` исчезает после перезапуска сервера — это соответствует заданию на in-memory хранение.
