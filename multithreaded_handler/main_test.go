package main

import (
	"fmt"
	"testing"
	"time"
)

func TestRunPipeline_AllJobsProcessed(t *testing.T) {
	urls := make([]string, 17)
	for i := range urls {
		urls[i] = fmt.Sprintf("https://example.com/task/%d", i+1)
	}

	results := runPipeline(urls, 5)
	if len(results) != len(urls) {
		t.Fatalf("получено %d результатов, ожидалось %d", len(results), len(urls))
	}

	seen := make(map[int]bool)
	for i, result := range results {
		if result.Job.ID != i+1 {
			t.Errorf("результат %d: ID = %d, ожидалось %d", i, result.Job.ID, i+1)
		}
		if seen[result.Job.ID] {
			t.Errorf("дублирован ID %d", result.Job.ID)
		}
		seen[result.Job.ID] = true
		if result.Job.URL != urls[i] {
			t.Errorf("ID %d: URL = %q, ожидалось %q", i+1, result.Job.URL, urls[i])
		}
		if result.Status != statusProcessed {
			t.Errorf("ID %d: некорректный статус %q", i+1, result.Status)
		}
		if result.Duration <= 0 {
			t.Errorf("ID %d: время должно быть положительным", i+1)
		}
	}
}

func TestRunPipeline_SingleWorker(t *testing.T) {
	urls := []string{"https://example.com/a", "https://example.com/b"}
	results := runPipeline(urls, 1)
	if len(results) != len(urls) {
		t.Fatalf("получено %d результатов, ожидалось %d", len(results), len(urls))
	}
}

func TestRunPipeline_EmptyInput(t *testing.T) {
	results := runPipeline(nil, 5)
	if len(results) != 0 {
		t.Fatalf("для пустого списка ожидалось 0 результатов, получено %d", len(results))
	}
}

func TestCalculateStats(t *testing.T) {
	results := []Result{
		{Status: statusProcessed, Duration: 100 * time.Millisecond},
		{Status: statusProcessed, Duration: 200 * time.Millisecond},
		{Status: "ошибка", Duration: 300 * time.Millisecond},
	}
	stats := calculateStats(results)

	if stats.Total != 3 || stats.Success != 2 {
		t.Errorf("неверное число задач: всего %d, успешно %d", stats.Total, stats.Success)
	}
	if stats.Average != 200*time.Millisecond {
		t.Errorf("среднее время %s, ожидалось 200ms", stats.Average)
	}
	if stats.Minimum != 100*time.Millisecond || stats.Maximum != 300*time.Millisecond {
		t.Errorf("неверный минимум/максимум: %s / %s", stats.Minimum, stats.Maximum)
	}
}

func TestCalculateStats_Empty(t *testing.T) {
	stats := calculateStats(nil)
	if stats != (Stats{}) {
		t.Fatalf("для пустого списка ожидались нулевые значения, получено %+v", stats)
	}
}
