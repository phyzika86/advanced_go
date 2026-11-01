package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"
)

const statusProcessed = "обработан"

// Job описывает отдельную задачу обработки URL.
type Job struct {
	ID  int
	URL string
}

// Result содержит сведения о выполненной задаче.
type Result struct {
	Job      Job
	Status   string
	Duration time.Duration
}

// Stats объединяет показатели по всем полученным результатам.
type Stats struct {
	Total   int
	Success int
	Average time.Duration
	Minimum time.Duration
	Maximum time.Duration
}

// simulateRequest имитирует HTTP-запрос без обращения к сети.
// Функции верхнего уровня пакета math/rand безопасны для конкурентных вызовов.
func simulateRequest(_ string) {
	delay := time.Duration(100+rand.Intn(301)) * time.Millisecond
	time.Sleep(delay)
}

// worker читает задания до закрытия jobs и направляет результаты в results.
// Вызывать wg.Add нужно до запуска воркеров, а не внутри этой функции.
func worker(jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		started := time.Now()
		simulateRequest(job.URL)

		results <- Result{
			Job:      job,
			Status:   statusProcessed,
			Duration: time.Since(started),
		}
	}
}

// runPipeline реализует Fan-out (пул воркеров) и Fan-in (общий канал results).
func runPipeline(urls []string, workerCount int) []Result {
	// jobs и results сделаны без буфера. Отправка каждого элемента
	// синхронизируется с получателем, а число активных задач
	// ограничено количеством воркеров.
	jobs := make(chan Job)
	results := make(chan Result)

	var wg sync.WaitGroup
	wg.Add(workerCount)

	// Fan-out: фиксированное число горутин читает из одного канала.
	for i := 0; i < workerCount; i++ {
		go worker(jobs, results, &wg)
	}

	// Производитель работает отдельно: main может сразу читать results.
	// Это предотвращает взаимную блокировку на небуферизованных каналах.
	go func() {
		defer close(jobs) // Только отправитель закрывает jobs.
		for i, url := range urls {
			jobs <- Job{ID: i + 1, URL: url}
		}
	}()

	// После завершения всех воркеров закрывается results.
	// Самим воркерам закрывать этот канал нельзя.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Fan-in: результаты всех воркеров собираются одним читателем.
	collected := make([]Result, 0, len(urls))
	for result := range results {
		collected = append(collected, result)
	}

	// Результаты приходят в произвольном порядке. Для отчёта
	// сортируем их по ID (в порядке исходных URL).
	sort.Slice(collected, func(i, j int) bool {
		return collected[i].Job.ID < collected[j].Job.ID
	})
	return collected
}

func calculateStats(results []Result) Stats {
	stats := Stats{Total: len(results)}
	if len(results) == 0 {
		return stats
	}

	var totalDuration time.Duration
	for i, result := range results {
		if result.Status == statusProcessed {
			stats.Success++
		}
		totalDuration += result.Duration
		if i == 0 || result.Duration < stats.Minimum {
			stats.Minimum = result.Duration
		}
		if result.Duration > stats.Maximum {
			stats.Maximum = result.Duration
		}
	}
	stats.Average = totalDuration / time.Duration(stats.Total)
	return stats
}

func printReport(results []Result, elapsed time.Duration) {
	stats := calculateStats(results)

	fmt.Println("\n========== ОТЧЁТ ПО URL ==========")
	fmt.Printf("%-4s %-45s %-12s %12s\n", "ID", "URL", "СТАТУС", "ВРЕМЯ")
	fmt.Println("-------------------------------------------------------------------------------")
	for _, result := range results {
		fmt.Printf("%-4d %-45s %-12s %12s\n",
			result.Job.ID, result.Job.URL, result.Status, result.Duration.Round(time.Millisecond))
	}
	fmt.Println("-------------------------------------------------------------------------------")
	fmt.Printf("Всего URL:             %d\n", stats.Total)
	fmt.Printf("Успешно обработано:    %d\n", stats.Success)
	fmt.Printf("Среднее время запроса: %s\n", stats.Average.Round(time.Millisecond))
	fmt.Printf("Минимальное время:     %s\n", stats.Minimum.Round(time.Millisecond))
	fmt.Printf("Максимальное время:    %s\n", stats.Maximum.Round(time.Millisecond))
	fmt.Printf("Общее время программы: %s\n", elapsed.Round(time.Millisecond))
}

func main() {
	workerCount := flag.Int("workers", 5, "максимальное количество параллельных воркеров")
	flag.Parse()

	if *workerCount <= 0 {
		fmt.Fprintln(os.Stderr, "ошибка: количество воркеров должно быть положительным")
		os.Exit(2)
	}

	// В учебной задаче это именно заглушка: реальных HTTP-запросов нет.
	urls := []string{
		"https://example.com",
		"https://example.org",
		"https://example.net",
		"https://example.com/api/users",
		"https://example.org/products",
		"https://example.net/health",
		"https://example.com/about",
		"https://example.org/contact",
		"https://example.net/docs",
		"https://example.com/status",
		"https://example.org/blog",
		"https://example.net/search",
	}

	rand.Seed(time.Now().UnixNano()) // Один раз, до запуска горутин.
	fmt.Printf("Заданий: %d, воркеров: %d\n", len(urls), *workerCount)

	started := time.Now()
	results := runPipeline(urls, *workerCount)
	printReport(results, time.Since(started))
}
