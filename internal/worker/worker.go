package worker

import (
	"context"
	"sync"
)

// Job is a function that performs a unit of work.
type Job func(ctx context.Context)

// worker processes jobs from a channel and reports errors.
type worker struct {
	jobs   <-chan Job      // Channel to receive jobs
	done   chan struct{}   // Channel to signal worker shutdown
	wgPool *sync.WaitGroup // WaitGroup to track worker completion
}

// newWorker creates a new worker.
func newWorker(jobs <-chan Job, wgPool *sync.WaitGroup) *worker {
	return &worker{
		jobs:   jobs,
		done:   make(chan struct{}),
		wgPool: wgPool,
	}
}

// Start launches the worker goroutine to process jobs until stopped or jobs channel is closed.
func (w *worker) Start(ctx context.Context) {
	w.wgPool.Add(1)
	go func() {
		defer func() {
			w.wgPool.Done()
		}()
		for {
			select {
			case <-w.done:
				return
			case job, ok := <-w.jobs:
				if !ok {
					return
				}
				job(ctx)
			}
		}
	}()
}

// Stop signals the worker to stop processing jobs.
func (w *worker) Stop() {
	close(w.done)
}

// WorkerPool manages a pool of workers processing jobs concurrently.
type WorkerPool struct {
	workers []*worker      // Slice of workers
	jobs    chan Job       // Channel for submitting jobs
	wg      sync.WaitGroup // WaitGroup to track all workers
}

// NewWorkerPool creates a pool with n workers.
func NewWorkerPool(n int) *WorkerPool {
	jobs := make(chan Job)
	wp := &WorkerPool{
		workers: make([]*worker, n),
		jobs:    jobs,
	}
	for i := 0; i < n; i++ {
		wp.workers[i] = newWorker(jobs, &wp.wg)
	}
	return wp
}

// Submit adds a job to the pool's queue.
func (wp *WorkerPool) Submit(job Job) {
	wp.jobs <- job
}

// Start launches all workers in the pool.
func (wp *WorkerPool) Start(ctx context.Context) {
	for _, w := range wp.workers {
		w.Start(ctx)
	}
}

// Stop signals all workers to stop, closes the jobs channel, and waits for all workers to finish.
func (wp *WorkerPool) Stop() {
	for _, w := range wp.workers {
		w.Stop()
	}
	close(wp.jobs)
	wp.wg.Wait()
}
