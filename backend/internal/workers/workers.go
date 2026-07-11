// Package workers implements a small background worker pool with a job queue
// and a periodic scheduler. It exists to generate steady, observable
// background activity (queue depth, job duration, job counts) for Kubernetes
// observability practice.
package workers

import (
	"log"
	"sync"
	"time"

	"memorydrive/internal/metrics"
	"memorydrive/internal/simulate"
)

// Job is a unit of background work.
type Job struct {
	ID        int
	CreatedAt time.Time
}

// Pool runs a fixed number of workers that drain a buffered job queue, plus a
// ticker that periodically enqueues new jobs.
type Pool struct {
	queue    chan Job
	workers  int
	interval time.Duration
	mem      *simulate.Manager

	stop     chan struct{}
	wg       sync.WaitGroup
	nextID   int
	nextIDMu sync.Mutex
}

// NewPool creates a worker pool. The queue is buffered so we can observe a
// non-zero queue depth under load.
func NewPool(workers int, interval time.Duration, mem *simulate.Manager) *Pool {
	if workers <= 0 {
		workers = 1
	}
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Pool{
		queue:    make(chan Job, 256),
		workers:  workers,
		interval: interval,
		mem:      mem,
		stop:     make(chan struct{}),
	}
}

// Start launches the workers and the periodic scheduler.
func (p *Pool) Start() {
	metrics.ActiveWorkers.Set(float64(p.workers))
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	p.wg.Add(1)
	go p.scheduler()
	log.Printf("workers: started %d workers, periodic interval %s", p.workers, p.interval)
}

// Stop signals all goroutines to exit and waits for them.
func (p *Pool) Stop() {
	close(p.stop)
	p.wg.Wait()
	metrics.ActiveWorkers.Set(0)
	metrics.WorkerQueueSize.Set(0)
}

// Enqueue adds a job to the queue without blocking. If the queue is full the
// job is dropped (so we never block request handling or crash under pressure).
func (p *Pool) Enqueue() {
	p.nextIDMu.Lock()
	p.nextID++
	id := p.nextID
	p.nextIDMu.Unlock()

	select {
	case p.queue <- Job{ID: id, CreatedAt: time.Now()}:
		metrics.WorkerQueueSize.Set(float64(len(p.queue)))
	default:
		// Queue full: drop the job rather than block.
	}
}

func (p *Pool) scheduler() {
	defer p.wg.Done()
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-ticker.C:
			// Enqueue a small burst so queue depth is visible in metrics.
			for i := 0; i < p.workers; i++ {
				p.Enqueue()
			}
		}
	}
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()
	for {
		select {
		case <-p.stop:
			return
		case job := <-p.queue:
			metrics.WorkerQueueSize.Set(float64(len(p.queue)))
			p.process(job)
		}
	}
}

// process performs the actual background job: touch the cache to keep it
// resident and do a short bit of CPU work, recording duration metrics.
func (p *Pool) process(job Job) {
	start := time.Now()

	if p.mem != nil {
		p.mem.TouchCache()
	}
	// A brief, bounded piece of work so the job has measurable duration.
	simulate.RunCPULoad(1, 50*time.Millisecond)

	metrics.BackgroundJobDuration.Observe(time.Since(start).Seconds())
	metrics.BackgroundJobs.Inc()
}
