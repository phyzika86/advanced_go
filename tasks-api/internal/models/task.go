package models

// Task is a task returned by the API.
// ID and CreatedAt are assigned by the server.
type Task struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at,omitempty"`
}
