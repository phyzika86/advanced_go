package main

import (
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"
)

func TestIsValidURL_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"HTTP", "http://example.com", true},
		{"HTTPS with path and query", "https://example.com/very/long/path?q=go", true},
		{"localhost with port", "http://localhost:8080/a", true},
		{"IP address", "https://127.0.0.1/path", true},
		{"empty", "", false},
		{"missing scheme", "example.com", false},
		{"ftp scheme", "ftp://example.com", false},
		{"empty host", "https://", false},
		{"no host with scheme", "https:path", false},
		{"space in URL", "https://example.com/has space", false},
		{"space at start", " https://example.com", false},
		{"newline", "https://example.com\n", false},
		{"invalid escape", "https://example.com/%zz", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidURL(tt.url); got != tt.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestShorten_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid HTTP", "http://example.com", false},
		{"valid HTTPS", "https://google.com/search?q=test", false},
		{"invalid URL", "not-a-url", true},
		{"empty string", "", true},
		{"wrong protocol", "ftp://example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewURLShortener()
			id, err := store.Shorten(tt.url)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Shorten(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
			}
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidURL) {
					t.Errorf("expected ErrInvalidURL, got %v", err)
				}
				return
			}
			if !validShortID(id) {
				t.Errorf("Shorten returned invalid ID %q", id)
			}
			original, err := store.GetOriginal(id)
			if err != nil || original != tt.url {
				t.Errorf("GetOriginal(%q) = %q, %v", id, original, err)
			}
		})
	}
}

func TestGetOriginal_TableDriven(t *testing.T) {
	store := NewURLShortener()
	store.urls["abc123XY"] = "https://example.com/long/path"
	tests := []struct {
		name    string
		id      string
		want    string
		wantErr bool
	}{
		{"existing", "abc123XY", "https://example.com/long/path", false},
		{"missing", "nope1234", "", true},
		{"empty", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.GetOriginal(tt.id)
			if got != tt.want || (err != nil) != tt.wantErr {
				t.Errorf("GetOriginal(%q) = (%q,%v), want (%q, err=%v)", tt.id, got, err, tt.want, tt.wantErr)
			}
			if tt.wantErr && !errors.Is(err, ErrNotFound) {
				t.Errorf("expected ErrNotFound, got %v", err)
			}
		})
	}
}

func TestShorten_RetriesOnCollision(t *testing.T) {
	store := NewURLShortener()
	ids := []string{"abcdefgh", "abcdefgh", "xyz12345"}
	index := 0
	store.newID = func() (string, error) {
		id := ids[index]
		index++
		return id, nil
	}
	first, err := store.Shorten("https://one.example")
	if err != nil || first != "abcdefgh" {
		t.Fatalf("first: id=%q err=%v", first, err)
	}
	second, err := store.Shorten("https://two.example")
	if err != nil || second != "xyz12345" {
		t.Fatalf("collision retry: id=%q err=%v", second, err)
	}
	original, _ := store.GetOriginal(first)
	if original != "https://one.example" {
		t.Fatalf("collision overwrote first URL: %q", original)
	}
}

func TestShorten_ExhaustedCollisions(t *testing.T) {
	store := NewURLShortener()
	store.urls["abcdefgh"] = "https://original.example"
	store.newID = func() (string, error) { return "abcdefgh", nil }
	_, err := store.Shorten("https://new.example")
	if !errors.Is(err, ErrIDGeneration) {
		t.Errorf("expected ErrIDGeneration, got %v", err)
	}
}

func TestShorten_RandomGeneratorError(t *testing.T) {
	store := NewURLShortener()
	failed := errors.New("random source unavailable")
	store.newID = func() (string, error) { return "", failed }
	_, err := store.Shorten("https://example.com")
	if !errors.Is(err, failed) {
		t.Errorf("expected wrapped random source error, got %v", err)
	}
}

func TestGenerateShortID(t *testing.T) {
	pattern := regexp.MustCompile(`^[A-Za-z0-9_-]{8}$`)
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := generateShortID()
		if err != nil {
			t.Fatalf("generateShortID: %v", err)
		}
		if !pattern.MatchString(id) {
			t.Fatalf("unexpected generated ID: %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate random ID: %q", id)
		}
		seen[id] = true
	}
}

func TestConcurrentShorten(t *testing.T) {
	store := NewURLShortener()
	const count = 200
	ids := make(chan string, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			original := fmt.Sprintf("https://example.com/item/%d", i)
			id, err := store.Shorten(original)
			if err != nil {
				t.Errorf("Shorten: %v", err)
				return
			}
			got, err := store.GetOriginal(id)
			if err != nil || got != original {
				t.Errorf("GetOriginal(%q) = %q, err=%v", id, got, err)
			}
			ids <- id
		}(i)
	}
	wg.Wait()
	close(ids)
	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate ID across goroutines: %q", id)
		}
		seen[id] = true
	}
	if len(seen) != count {
		t.Errorf("saved %d URLs, want %d", len(seen), count)
	}
}
