package main

import (
	"log"
	"net/http"
	"os"

	"tasks-api/internal/handlers"
	httpmiddleware "tasks-api/internal/http"
	"tasks-api/internal/storage/memory"
)

func main() {
	store := memory.New()
	h := handlers.New(store)

	mux := http.NewServeMux()
	mux.HandleFunc("/tasks", h.TasksCollection)
	mux.HandleFunc("/tasks/", h.TaskItem)
	mux.HandleFunc("/health", handlers.Health)
	mux.HandleFunc("/", handlers.NotFound)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:    ":" + port,
		Handler: httpmiddleware.Recover(httpmiddleware.Logging(mux)),
	}
	log.Printf("server listening on http://localhost:%s", port)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
