package memory_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"tasks-api/internal/models"
	"tasks-api/internal/storage"
	"tasks-api/internal/storage/memory"
)

func TestStorageCRUD(t *testing.T) {
	s := memory.New()
	if got := s.List(); got == nil || len(got) != 0 {
		t.Fatalf("empty List() should return [], got %#v", got)
	}

	a, err := s.Create(models.Task{Title: "первая"})
	if err != nil || a.ID != 1 || a.CreatedAt == "" {
		t.Fatalf("Create() = %+v, %v", a, err)
	}
	if _, err := time.Parse(time.RFC3339, a.CreatedAt); err != nil {
		t.Fatalf("invalid created_at: %s", a.CreatedAt)
	}
	b, _ := s.Create(models.Task{Title: "вторая"})
	if b.ID != 2 {
		t.Fatalf("expected ID 2, got %d", b.ID)
	}
	if fetched, ok := s.Get(1); !ok || fetched != a {
		t.Fatalf("Get(1) = %+v, %v", fetched, ok)
	}

	updated, err := s.Update(1, models.Task{ID: 999, Title: "обновлённая", Done: true, CreatedAt: "fake"})
	if err != nil || updated.ID != 1 || updated.CreatedAt != a.CreatedAt || !updated.Done {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}
	if err := s.Delete(1); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(1); ok {
		t.Fatal("deleted task still exists")
	}
	if _, err := s.Update(1, models.Task{}); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for Update, got %v", err)
	}
	if err := s.Delete(1); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for Delete, got %v", err)
	}
	if next, _ := s.Create(models.Task{Title: "после удаления"}); next.ID != 3 {
		t.Fatalf("IDs must not be reused, got %d", next.ID)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := memory.New()
	const count = 200
	ids := make(chan int, count)
	var wg sync.WaitGroup

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			task, err := s.Create(models.Task{Title: "parallel"})
			if err != nil {
				t.Errorf("Create error: %v", err)
				return
			}
			ids <- task.ID
			_, _ = s.Get(task.ID)
			_ = s.List()
			_, err = s.Update(task.ID, models.Task{Title: "updated", Done: true})
			if err != nil {
				t.Errorf("Update error: %v", err)
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[int]bool)
	for id := range ids {
		if seen[id] {
			t.Fatalf("duplicate task ID: %d", id)
		}
		seen[id] = true
	}
	if len(seen) != count || len(s.List()) != count {
		t.Fatalf("expected %d unique tasks, got IDs=%d tasks=%d", count, len(seen), len(s.List()))
	}

	for id := 1; id <= count; id++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := s.Delete(id); err != nil {
				t.Errorf("Delete(%d): %v", id, err)
			}
		}(id)
	}
	wg.Wait()
	if got := len(s.List()); got != 0 {
		t.Fatalf("expected no tasks after concurrent Delete, got %d", got)
	}
}
