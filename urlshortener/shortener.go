package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
)

const maxIDAttempts = 10

var (
	ErrInvalidURL   = errors.New("URL must be a valid http:// or https:// address")
	ErrNotFound     = errors.New("short URL not found")
	ErrIDGeneration = errors.New("failed to generate a unique short ID")
)

// URLShortener stores links only in RAM. RWMutex protects the map from
// simultaneous HTTP requests (a Go HTTP server handles them concurrently).
type URLShortener struct {
	urls  map[string]string
	mu    sync.RWMutex
	newID func() (string, error)
}

func NewURLShortener() *URLShortener {
	return &URLShortener{
		urls:  make(map[string]string),
		newID: generateShortID,
	}
}

// Shorten validates and saves a link. If a generated ID already exists,
// it tries again rather than overwriting the existing link.
func (us *URLShortener) Shorten(originalURL string) (string, error) {
	if !isValidURL(originalURL) {
		return "", ErrInvalidURL
	}

	us.mu.Lock()
	defer us.mu.Unlock()

	for attempt := 0; attempt < maxIDAttempts; attempt++ {
		shortID, err := us.newID()
		if err != nil {
			return "", fmt.Errorf("generate short ID: %w", err)
		}
		if _, exists := us.urls[shortID]; exists {
			continue
		}
		us.urls[shortID] = originalURL
		return shortID, nil
	}
	return "", ErrIDGeneration
}

// GetOriginal resolves a short ID back to its saved original URL.
func (us *URLShortener) GetOriginal(shortID string) (string, error) {
	us.mu.RLock()
	originalURL, exists := us.urls[shortID]
	us.mu.RUnlock()

	if !exists {
		return "", ErrNotFound
	}
	return originalURL, nil
}

// Six random bytes become exactly eight URL-safe base64 characters.
func generateShortID() (string, error) {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func isValidURL(raw string) bool {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, " \t\n\r") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" {
		return false
	}
	return true
}
