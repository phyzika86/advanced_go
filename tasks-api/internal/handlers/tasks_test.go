package handlers_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tasks-api/internal/handlers"
	httpmiddleware "tasks-api/internal/http"
	"tasks-api/internal/models"
	"tasks-api/internal/storage/memory"
)

func newServer() http.Handler {
	h := handlers.New(memory.New())
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", h.TasksCollection)
	mux.HandleFunc("/tasks/", h.TaskItem)
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/", handlers.NotFound)
	return httpmiddleware.Recover(mux)
}

func request(t *testing.T, server http.Handler, method, path, body, contentType string, status int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)
	if w.Code != status {
		t.Fatalf("%s %s: wanted HTTP %d, got %d: %s", method, path, status, w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	if status == http.StatusNoContent {
		if w.Body.Len() > 0 {
			t.Fatalf("204 must be empty, got %s", w.Body.String())
		}
	} else if !json.Valid(w.Body.Bytes()) {
		t.Fatalf("response must be JSON, got: %q", w.Body.String())
	}
	return w
}

func TestRESTFlow(t *testing.T) {
	s := newServer()
	w := request(t, s, "GET", "/tasks", "", "", 200)
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty list should be [], got %s", w.Body.String())
	}

	w = request(t, s, "POST", "/tasks", `{"title":"  Купить молоко  "}`, "application/json; charset=utf-8", 201)
	var created models.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Title != "Купить молоко" || created.Done || created.CreatedAt == "" {
		t.Fatalf("unexpected task: %+v", created)
	}
	if _, err := time.Parse(time.RFC3339, created.CreatedAt); err != nil {
		t.Fatalf("created_at not RFC3339: %s", created.CreatedAt)
	}
	if got := w.Header().Get("Location"); got != "/tasks/1" {
		t.Fatalf("Location header = %s", got)
	}

	w = request(t, s, "GET", "/tasks/1", "", "", 200)
	var fetched models.Task
	_ = json.Unmarshal(w.Body.Bytes(), &fetched)
	if fetched != created {
		t.Fatalf("GET mismatch: %+v != %+v", fetched, created)
	}

	w = request(t, s, "PUT", "/tasks/1", `{"title":"Купить хлеб","done":true}`, "application/json", 200)
	var updated models.Task
	_ = json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.ID != created.ID || updated.CreatedAt != created.CreatedAt || updated.Title != "Купить хлеб" || !updated.Done {
		t.Fatalf("updated task: %+v", updated)
	}

	w = request(t, s, "GET", "/tasks", "", "", 200)
	var tasks []models.Task
	_ = json.Unmarshal(w.Body.Bytes(), &tasks)
	if len(tasks) != 1 || tasks[0] != updated {
		t.Fatalf("List() = %+v", tasks)
	}

	request(t, s, "DELETE", "/tasks/1", "", "", 204)
	request(t, s, "GET", "/tasks/1", "", "", 404)
	request(t, s, "DELETE", "/tasks/1", "", "", 404)
	request(t, s, "GET", "/health", "", "", 200)
}

func TestInvalidRequests(t *testing.T) {
	tests := []struct {
		name, method, path, body, contentType string
		status                                int
	}{
		{"unsupported collection method", "PATCH", "/tasks", "", "", 405},
		{"missing title", "POST", "/tasks", `{"done":true}`, "application/json", 400},
		{"empty title", "POST", "/tasks", `{"title":"  "}`, "application/json", 400},
		{"null title", "POST", "/tasks", `{"title":null}`, "application/json", 400},
		{"wrong title type", "POST", "/tasks", `{"title":12}`, "application/json", 400},
		{"wrong done type", "POST", "/tasks", `{"title":"x","done":"yes"}`, "application/json", 400},
		{"null done", "POST", "/tasks", `{"title":"x","done":null}`, "application/json", 400},
		{"missing Content-Type", "POST", "/tasks", `{"title":"x"}`, "", 400},
		{"wrong Content-Type", "POST", "/tasks", `{"title":"x"}`, "text/plain", 400},
		{"malformed JSON", "POST", "/tasks", `{"title":`, "application/json", 400},
		{"unknown JSON field", "POST", "/tasks", `{"title":"x","id":40}`, "application/json", 400},
		{"multiple JSON objects", "POST", "/tasks", `{"title":"x"}{"title":"y"}`, "application/json", 400},
		{"invalid id", "GET", "/tasks/abc", "", "", 400},
		{"zero id", "GET", "/tasks/0", "", "", 400},
		{"negative id", "GET", "/tasks/-1", "", "", 400},
		{"missing id", "GET", "/tasks/999", "", "", 404},
		{"unknown put", "PUT", "/tasks/999", `{"title":"x","done":false}`, "application/json", 404},
		{"incomplete put", "PUT", "/tasks/1", `{"title":"x"}`, "application/json", 400},
		{"bad put title", "PUT", "/tasks/1", `{"title":"","done":true}`, "application/json", 400},
		{"bad put done", "PUT", "/tasks/1", `{"title":"x","done":null}`, "application/json", 400},
		{"unknown delete", "DELETE", "/tasks/999", "", "", 404},
		{"unsupported item method", "POST", "/tasks/1", "", "", 405},
		{"health wrong method", "POST", "/health", "", "", 405},
		{"unknown route", "GET", "/unknown", "", "", 404},
		{"nested task route", "GET", "/tasks/1/subitem", "", "", 404},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newServer()
			request(t, s, "POST", "/tasks", `{"title":"existing"}`, "application/json", 201)
			w := request(t, s, tt.method, tt.path, tt.body, tt.contentType, tt.status)
			if tt.status == 400 || tt.status == 404 || tt.status == 405 {
				var e map[string]any
				if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil || e["error"] == "" {
					t.Fatalf("JSON error response required, got %s, %v", w.Body.String(), err)
				}
			}
		})
	}
}

// failingStorage confirms unexpected storage errors are returned as HTTP 500.
type failingStorage struct{ *memory.Store }

func (f failingStorage) Create(models.Task) (models.Task, error) {
	return models.Task{}, errors.New("disk??")
}
func (f failingStorage) Update(int, models.Task) (models.Task, error) {
	return models.Task{}, errors.New("failed")
}
func (f failingStorage) Delete(int) error { return errors.New("failed") }

func TestInternalErrors(t *testing.T) {
	s := failingStorage{memory.New()}
	h := handlers.New(s)
	request(t, http.HandlerFunc(h.TasksCollection), "POST", "/tasks", `{"title":"hello"}`, "application/json", 500)
	_, _ = s.Store.Create(models.Task{Title: "one"})
	request(t, http.HandlerFunc(h.TaskItem), "PUT", "/tasks/1", `{"title":"x","done":true}`, "application/json", 500)
	request(t, http.HandlerFunc(h.TaskItem), "DELETE", "/tasks/1", "", "", 500)
}

func TestPanicRecovery(t *testing.T) {
	panicHandler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("unexpected") })
	request(t, httpmiddleware.Recover(panicHandler), "GET", "/anywhere", "", "", 500)
}
