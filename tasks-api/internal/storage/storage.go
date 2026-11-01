package storage

import (
	"errors"

	"tasks-api/internal/models"
)

// ErrNotFound allows callers to distinguish a missing task from a server error.
var ErrNotFound = errors.New("задача не найдена")

// Storage describes task persistence, independent of its implementation.
type Storage interface {
	List() []models.Task
	Create(models.Task) (models.Task, error)
	Get(id int) (models.Task, bool)
	Update(id int, task models.Task) (models.Task, error)
	Delete(id int) error
}
