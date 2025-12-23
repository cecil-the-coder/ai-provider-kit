// Package utils provides utility functions for the AI Provider Kit.
package utils

import (
	"runtime"
	"sync"
)

// WorkerPool manages a pool of worker goroutines for concurrent task execution.
// It automatically scales based on the number of available CPU cores and provides
// a simple interface for parallelizing workloads.
type WorkerPool struct {
	workerCount int
	taskQueue   chan func()
	wg          sync.WaitGroup
	once        sync.Once
}

// NewWorkerPool creates a new worker pool with the specified number of workers.
// If maxWorkers is 0 or negative, it defaults to the number of available CPUs.
func NewWorkerPool(maxWorkers int) *WorkerPool {
	if maxWorkers <= 0 {
		maxWorkers = runtime.NumCPU()
	}
	return &WorkerPool{
		workerCount: maxWorkers,
		taskQueue:   make(chan func(), maxWorkers*2),
	}
}

// Start initializes the worker pool's goroutines.
// This method is idempotent and can be called multiple times safely.
func (p *WorkerPool) Start() {
	p.once.Do(func() {
		for i := 0; i < p.workerCount; i++ {
			p.wg.Add(1)
			go p.worker()
		}
	})
}

// worker is the main worker loop that processes tasks from the queue.
func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for task := range p.taskQueue {
		task()
	}
}

// Submit adds a task to the worker pool queue.
// The task will be executed by one of the available workers.
// If the pool hasn't been started yet, it will be started automatically.
func (p *WorkerPool) Submit(task func()) {
	p.Start()
	p.taskQueue <- task
}

// Close gracefully shuts down the worker pool.
// It waits for all submitted tasks to complete before returning.
func (p *WorkerPool) Close() {
	close(p.taskQueue)
	p.wg.Wait()
}

// ProcessSlice concurrently processes a slice of items using the worker pool.
// It maps each item to a result using the provided mapper function.
// The results are returned in the same order as the input items.
func ProcessSlice[T, R any](items []T, mapper func(T) R) []R {
	if len(items) == 0 {
		return nil
	}

	// For small slices, sequential processing may be faster due to overhead
	if len(items) <= 4 {
		return processSequential(items, mapper)
	}

	results := make([]R, len(items))
	var wg sync.WaitGroup

	// Use a worker count based on CPU cores, but cap at the number of items
	workerCount := min(runtime.NumCPU(), len(items))

	// Process items in batches
	batchSize := (len(items) + workerCount - 1) / workerCount

	for i := 0; i < len(items); i += batchSize {
		end := min(i+batchSize, len(items))
		wg.Add(1)
		go func(start, end int) {
			defer wg.Done()
			for j := start; j < end; j++ {
				results[j] = mapper(items[j])
			}
		}(i, end)
	}

	wg.Wait()
	return results
}

// ProcessSliceWithPool processes a slice using an existing worker pool.
// This is useful when you want to reuse a pool for multiple operations.
func ProcessSliceWithPool[T, R any](pool *WorkerPool, items []T, mapper func(T) R) []R {
	if len(items) == 0 {
		return nil
	}

	results := make([]R, len(items))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for i, item := range items {
		wg.Add(1)
		idx := i // Capture loop variable
		pool.Submit(func() {
			defer wg.Done()
			result := mapper(item)
			mu.Lock()
			results[idx] = result
			mu.Unlock()
		})
	}

	wg.Wait()
	return results
}

// processSequential processes items sequentially without any goroutine overhead.
func processSequential[T, R any](items []T, mapper func(T) R) []R {
	results := make([]R, len(items))
	for i, item := range items {
		results[i] = mapper(item)
	}
	return results
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
