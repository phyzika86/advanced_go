package httpmiddleware

import (
	"encoding/json"
	"log"
	"net/http"
)

// Logging logs the method and URL path of each incoming request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// Recover converts an unexpected panic into a JSON server error.
// Like any recovery middleware, it cannot replace an already-sent response.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Printf("panic processing %s %s: %v", r.Method, r.URL.Path, v)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "внутренняя ошибка сервера"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
