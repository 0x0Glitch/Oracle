package main

import (
	"context"
	"log"
	"sync"
	"time"
)

type Job interface {
	Name() string
	Interval() time.Duration
	Run(ctx context.Context) error
}

// Closer is an optional interface for jobs that need cleanup
type Closer interface {
	Close() error
}

type Worker struct {
	jobs []Job
	wg   sync.WaitGroup
}

func NewWorker() *Worker {
	return &Worker{
		jobs: make([]Job, 0),
	}
}

func (w *Worker) Register(job Job) {
	w.jobs = append(w.jobs, job)
}

func (w *Worker) Start(ctx context.Context) {
	for _, job := range w.jobs {
		w.wg.Add(1)
		go w.runJob(ctx, job)
	}
	log.Printf("started %d workers", len(w.jobs))
}

func (w *Worker) Wait() {
	w.wg.Wait()
}

// Close closes all jobs that implement the Closer interface
func (w *Worker) Close() {
	for _, job := range w.jobs {
		if closer, ok := job.(Closer); ok {
			if err := closer.Close(); err != nil {
				log.Printf("[%s] error closing: %v", job.Name(), err)
			} else {
				log.Printf("[%s] closed", job.Name())
			}
		}
	}
}

func (w *Worker) runJob(ctx context.Context, job Job) {
	defer w.wg.Done()

	log.Printf("[%s] started", job.Name())

	w.executeJob(ctx, job)

	ticker := time.NewTicker(job.Interval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.executeJob(ctx, job)
		case <-ctx.Done():
			log.Printf("[%s] stopped", job.Name())
			return
		}
	}
}

func (w *Worker) executeJob(ctx context.Context, job Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[%s] PANIC RECOVERED: %v", job.Name(), r)
		}
	}()

	start := time.Now()
	err := job.Run(ctx)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[%s] error after %v: %v", job.Name(), duration, err)
	} else {
		log.Printf("[%s] completed in %v", job.Name(), duration)
	}
}
