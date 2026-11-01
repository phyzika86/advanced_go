package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// newRouter can be used by both the real server and httptest tests.
func newRouter(shortener *URLShortener) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/shorten", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}

		// Limit the JSON payload so a client cannot exhaust server memory.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()

		var request struct {
			URL string `json:"url"`
		}
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		// Forbid extra JSON values after the first object.
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeError(w, http.StatusBadRequest, "request body must contain only one JSON object")
			return
		}

		shortID, err := shortener.Shorten(request.URL)
		if errors.Is(err, ErrInvalidURL) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err != nil {
			log.Printf("shortening failed: %v", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusCreated, struct {
			ShortURL    string `json:"short_url"`
			OriginalURL string `json:"original_url"`
		}{ShortURL: shortID, OriginalURL: request.URL})
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Only exactly one 6-8 character path segment is a short ID.
		shortID := strings.TrimPrefix(r.URL.Path, "/")
		if !validShortID(shortID) {
			writeError(w, http.StatusNotFound, "short URL not found")
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}

		originalURL, err := shortener.GetOriginal(shortID)
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, "short URL not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		// A 302 response tells the browser to visit this original URL.
		w.Header().Set("Location", originalURL)
		w.WriteHeader(http.StatusFound)
	})

	return loggingMiddleware(mux)
}

func validShortID(id string) bool {
	if len(id) < 6 || len(id) > 8 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := newRouter(NewURLShortener())
	log.Printf("URL shortener listening on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, server))
}
