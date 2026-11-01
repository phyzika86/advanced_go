// Package memory implements a concurrent in-memory task store.
package memory

import (
	"sort"
	"sync"
	"time"

	"tasks-api/internal/models"
	"tasks-api/internal/storage"
)

// Store is safe for concurrent calls from multiple HTTP handlers.
type Store struct {
	mu     sync.RWMutex
	tasks  map[int]models.Task
	nextID int
}

// New creates an initially empty in-memory store.
func New() *Store {
	return &Store{
		tasks:  make(map[int]models.Task),
		nextID: 1,
	}
}

// Compile-time assertion that *Store implements the required interface.
var _ storage.Storage = (*Store)(nil)

// List returns a snapshot of tasks ordered by ascending ID.
func (s *Store) List() []models.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, task)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Create assigns a unique, increasing ID and a creation timestamp.
func (s *Store) Create(task models.Task) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task.ID = s.nextID
	s.nextID++
	task.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	s.tasks[task.ID] = task
	return task, nil
}

// Get returns a copy of a task (not a reference to internal state).
func (s *Store) Get(id int) (models.Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	task, ok := s.tasks[id]
	return task, ok
}

// Update replaces the editable fields, retaining server-managed metadata.
func (s *Store) Update(id int, task models.Task) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	old, ok := s.tasks[id]
	if !ok {
		return models.Task{}, storage.ErrNotFound
	}
	task.ID = id
	task.CreatedAt = old.CreatedAt
	s.tasks[id] = task
	return task, nil
}

// Delete removes an existing task.
func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tasks[id]; !ok {
		return storage.ErrNotFound
	}
	delete(s.tasks, id)
	return nil
}
