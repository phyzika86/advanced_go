package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"tasks-api/internal/models"
	"tasks-api/internal/storage"
)

const maxBodyBytes = 1 << 20 // 1 MiB

type Handler struct{ Store storage.Storage }

func New(s storage.Storage) *Handler { return &Handler{Store: s} }

// taskInput contains only user-editable fields. Server fields cannot be sent.
type taskInput struct {
	Title *string         `json:"title"`
	Done  json.RawMessage `json:"done"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// NotFound ensures unknown routes also return JSON rather than net/http's HTML/text.
func NotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "маршрут не найден")
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "метод не поддерживается")
}

// decodeTask checks that the Content-Type is JSON and the body is one JSON
// object with only the supported fields. Additional JSON values are rejected.
func decodeTask(w http.ResponseWriter, r *http.Request) (taskInput, error) {
	var in taskInput

	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return in, errors.New("Content-Type должен быть application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&in); err != nil {
		return in, errors.New("некорректный JSON или неверные типы полей")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return in, errors.New("в теле должен быть один JSON-объект")
	}
	return in, nil
}

func parseDone(in taskInput, required bool) (bool, error) {
	if len(in.Done) == 0 {
		if required {
			return false, errors.New("поле done обязательно для PUT")
		}
		return false, nil // Default status for a newly created task.
	}
	if string(in.Done) == "null" {
		return false, errors.New("поле done должно быть boolean")
	}
	var done bool
	if err := json.Unmarshal(in.Done, &done); err != nil {
		return false, errors.New("поле done должно быть boolean")
	}
	return done, nil
}

func validateTitle(in taskInput) (string, error) {
	if in.Title == nil || strings.TrimSpace(*in.Title) == "" {
		return "", errors.New("поле title обязательно и не должно быть пустым")
	}
	return strings.TrimSpace(*in.Title), nil
}

// TasksCollection handles GET /tasks and POST /tasks.
func (h *Handler) TasksCollection(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tasks" {
		NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.Store.List())

	case http.MethodPost:
		in, err := decodeTask(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title, err := validateTitle(in)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		done, err := parseDone(in, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		task, err := h.Store.Create(models.Task{Title: title, Done: done})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "не удалось создать задачу")
			return
		}
		w.Header().Set("Location", "/tasks/"+strconv.Itoa(task.ID))
		writeJSON(w, http.StatusCreated, task)

	default:
		methodNotAllowed(w, "GET, POST")
	}
}

// TaskItem handles GET /tasks/{id}, PUT /tasks/{id}, DELETE /tasks/{id}.
func (h *Handler) TaskItem(w http.ResponseWriter, r *http.Request) {
	part := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if part == "" || strings.Contains(part, "/") || part == r.URL.Path {
		NotFound(w, r)
		return
	}
	id, err := strconv.Atoi(part)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id должен быть положительным целым числом")
		return
	}

	switch r.Method {
	case http.MethodGet:
		task, ok := h.Store.Get(id)
		if !ok {
			writeError(w, http.StatusNotFound, "задача не найдена")
			return
		}
		writeJSON(w, http.StatusOK, task)

	case http.MethodPut:
		in, err := decodeTask(w, r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		title, err := validateTitle(in)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		done, err := parseDone(in, true) // PUT replaces the complete editable representation.
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		task, err := h.Store.Update(id, models.Task{Title: title, Done: done})
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "задача не найдена")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "не удалось обновить задачу")
			return
		}
		writeJSON(w, http.StatusOK, task)

	case http.MethodDelete:
		if err := h.Store.Delete(id); err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				writeError(w, http.StatusNotFound, "задача не найдена")
			} else {
				writeError(w, http.StatusInternalServerError, "не удалось удалить задачу")
			}
			return
		}
		// HTTP 204 must not have a response body, even when the API uses JSON.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w, "GET, PUT, DELETE")
	}
}

func Health(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/health" {
		NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, "GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
