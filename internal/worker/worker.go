package worker

import (
	"context"
	"sync"
)

// Job is a function that performs a unit of work and may return an error.
type Job func(ctx context.Context) error

// worker processes jobs from a channel and reports errors.
type worker struct {
	jobs   <-chan Job      // Channel to receive jobs
	done   chan struct{}   // Channel to signal worker shutdown
	errors chan error      // Channel to report job errors
	wg     *sync.WaitGroup // WaitGroup to track worker completion
}

// newWorker creates a new worker.
func newWorker(jobs <-chan Job, wg *sync.WaitGroup) *worker {
	return &worker{
		jobs:   jobs,
		done:   make(chan struct{}),
		errors: make(chan error, 8),
		wg:     wg,
	}
}

// Start launches the worker goroutine to process jobs until stopped or jobs channel is closed.
func (w *worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer func() {
			close(w.errors)
			w.wg.Done()
		}()
		for {
			select {
			case <-w.done:
				return
			case job, ok := <-w.jobs:
				if !ok {
					return
				}
				if err := job(ctx); err != nil {
					select {
					case w.errors <- err:
						// error sent
					default:
						// drop error if channel is full
					}
				}
			}
		}
	}()
}

// Stop signals the worker to stop processing jobs.
func (w *worker) Stop() {
	close(w.done)
}

// Errors returns the error channel for job processing errors.
func (w *worker) Errors() <-chan error {
	return w.errors
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

// Errors returns a merged channel of errors from all workers and closes it when all are done.
func (wp *WorkerPool) Errors() <-chan error {
	errCh := make(chan error, 8)
	var wg sync.WaitGroup
	wg.Add(len(wp.workers))
	for _, w := range wp.workers {
		workerErrs := w.Errors()
		go func(c <-chan error) {
			defer wg.Done()
			for err := range c {
				errCh <- err
			}
		}(workerErrs)
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	return errCh
}
