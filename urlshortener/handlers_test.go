package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPOSTShorten_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       string
		wantStatus int
	}{
		{"valid JSON", http.MethodPost, `{"url":"https://example.com/very/long/path"}`, 201},
		{"valid HTTP", http.MethodPost, `{"url":"http://example.com"}`, 201},
		{"empty URL", http.MethodPost, `{"url":""}`, 400},
		{"invalid URL", http.MethodPost, `{"url":"not-a-url"}`, 400},
		{"bad JSON", http.MethodPost, `{"url":`, 400},
		{"wrong JSON type", http.MethodPost, `{"url":123}`, 400},
		{"unknown field", http.MethodPost, `{"url":"https://example.com","extra":5}`, 400},
		{"extra JSON object", http.MethodPost, `{"url":"https://example.com"} {}`, 400},
		{"missing URL field", http.MethodPost, `{}`, 400},
		{"wrong HTTP method", http.MethodGet, ``, 405},
		{"overly large payload", http.MethodPost, `{"url":"` + strings.Repeat("a", 1<<20) + `"}`, 400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := newRouter(NewURLShortener())
			req := httptest.NewRequest(tt.method, "/shorten", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body)
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q", rec.Header().Get("Content-Type"))
			}
			if tt.wantStatus == 201 {
				var got struct {
					ShortURL    string `json:"short_url"`
					OriginalURL string `json:"original_url"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
					t.Fatalf("response is not JSON: %v", err)
				}
				if !validShortID(got.ShortURL) || got.OriginalURL == "" {
					t.Errorf("unexpected response: %+v", got)
				}
			} else {
				assertJSONError(t, rec.Body.Bytes())
			}
		})
	}
}

func TestRedirect_TableDriven(t *testing.T) {
	store := NewURLShortener()
	id, err := store.Shorten("https://example.com/target?q=1")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		method       string
		path         string
		wantStatus   int
		wantLocation string
	}{
		{"found", http.MethodGet, "/" + id, 302, "https://example.com/target?q=1"},
		{"missing", http.MethodGet, "/missing1", 404, ""},
		{"empty path", http.MethodGet, "/", 404, ""},
		{"nested path", http.MethodGet, "/abc123/child", 404, ""},
		{"invalid ID", http.MethodGet, "/abc*12", 404, ""},
		{"short ID", http.MethodGet, "/abc", 404, ""},
		{"long ID", http.MethodGet, "/abcdefghi", 404, ""},
		{"wrong method", http.MethodPost, "/" + id, 405, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newRouter(store).ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tt.wantStatus, rec.Body)
			}
			if location := rec.Header().Get("Location"); location != tt.wantLocation {
				t.Errorf("Location = %q, want %q", location, tt.wantLocation)
			}
			if tt.wantStatus != 302 {
				if rec.Header().Get("Content-Type") != "application/json" {
					t.Errorf("expected JSON error, got Content-Type %q", rec.Header().Get("Content-Type"))
				}
				assertJSONError(t, rec.Body.Bytes())
			} else if rec.Body.Len() != 0 {
				t.Errorf("redirect must have empty response body, got %q", rec.Body.String())
			}
		})
	}
}

func TestPOSTShorten_InternalError(t *testing.T) {
	store := NewURLShortener()
	store.newID = func() (string, error) { return "", errors.New("no entropy") }
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/shorten", strings.NewReader(`{"url":"https://example.com"}`))
	newRouter(store).ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	assertJSONError(t, rec.Body.Bytes())
}

func TestURLShortener_EndToEnd_HTTPTest(t *testing.T) {
	server := httptest.NewServer(newRouter(NewURLShortener()))
	defer server.Close()

	resp, err := http.Post(server.URL+"/shorten", "application/json", strings.NewReader(`{"url":"https://example.com/original"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("POST status = %d", resp.StatusCode)
	}
	var data struct {
		ShortURL string `json:"short_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	redirect, err := client.Get(server.URL + "/" + data.ShortURL)
	if err != nil {
		t.Fatal(err)
	}
	defer redirect.Body.Close()
	if redirect.StatusCode != 302 || redirect.Header.Get("Location") != "https://example.com/original" {
		t.Errorf("redirect status=%d location=%q", redirect.StatusCode, redirect.Header.Get("Location"))
	}
}

func assertJSONError(t *testing.T, data []byte) {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(data, &payload); err != nil || payload.Error == "" {
		t.Errorf("expected JSON error, got %q (decode err=%v)", string(data), err)
	}
}

// Sanity check: all success JSON responses contain exactly one object.
func TestJSONResponseIsSingleObject(t *testing.T) {
	rec := httptest.NewRecorder()
	newRouter(NewURLShortener()).ServeHTTP(rec, httptest.NewRequest("POST", "/shorten", strings.NewReader(`{"url":"https://example.com"}`)))
	decoder := json.NewDecoder(strings.NewReader(rec.Body.String()))
	var first map[string]interface{}
	if err := decoder.Decode(&first); err != nil {
		t.Fatal(err)
	}
	var second interface{}
	if err := decoder.Decode(&second); err != io.EOF {
		t.Errorf("want EOF after first object; got %v", err)
	}
}
