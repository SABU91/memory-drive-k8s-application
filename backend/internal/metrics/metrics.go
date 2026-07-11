// Package metrics defines and registers all Prometheus metrics for the app.
//
// The default Prometheus registry already ships the Go runtime collector
// (go_* : goroutines, heap, GC) and the process collector
// (process_* : process_cpu_seconds_total, process_resident_memory_bytes),
// which cover baseline CPU and memory. On top of those we expose
// application-specific metrics that make the workload-generation features
// observable in Grafana.
package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// HTTPRequests counts every HTTP request, labelled by method, route and status.
	HTTPRequests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "memorydrive_http_requests_total",
			Help: "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// HTTPLatency records request handling latency in seconds.
	HTTPLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "memorydrive_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Uploads counts uploaded items, labelled by kind (note/text/image).
	Uploads = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "memorydrive_uploads_total",
			Help: "Total number of uploaded items.",
		},
		[]string{"kind"},
	)

	// UploadSize records the size distribution of uploads in bytes.
	UploadSize = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memorydrive_upload_size_bytes",
			Help:    "Distribution of upload sizes in bytes.",
			Buckets: prometheus.ExponentialBuckets(1024, 4, 8), // 1KB .. ~16MB
		},
	)

	// MemoryAllocated reports the amount of memory (bytes) the application is
	// deliberately holding via /simulate/memory and the baseline allocation.
	MemoryAllocated = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_managed_memory_bytes",
			Help: "Memory deliberately allocated by the workload generator.",
		},
	)

	// HeapInUse mirrors runtime heap usage; refreshed periodically.
	HeapInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_heap_inuse_bytes",
			Help: "Bytes in in-use heap spans (runtime.MemStats.HeapInuse).",
		},
	)

	// CPUUsagePercent is an approximate process CPU utilisation percentage,
	// refreshed periodically from process CPU time.
	CPUUsagePercent = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_cpu_usage_percent",
			Help: "Approximate process CPU utilisation percentage.",
		},
	)

	// CacheSize reports the current in-memory cache size in bytes.
	CacheSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_cache_size_bytes",
			Help: "Current in-memory cache size in bytes.",
		},
	)

	// WorkerQueueSize reports the number of jobs currently queued.
	WorkerQueueSize = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_worker_queue_size",
			Help: "Number of jobs currently waiting in the worker queue.",
		},
	)

	// ActiveWorkers reports how many background workers are running.
	ActiveWorkers = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "memorydrive_active_workers",
			Help: "Number of active background workers.",
		},
	)

	// BackgroundJobDuration records how long background jobs take to run.
	BackgroundJobDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "memorydrive_background_job_duration_seconds",
			Help:    "Execution time of background jobs in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)

	// BackgroundJobs counts completed background jobs.
	BackgroundJobs = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "memorydrive_background_jobs_total",
			Help: "Total number of background jobs executed.",
		},
	)
)

// StartRuntimeSampler periodically refreshes the runtime-derived gauges
// (heap usage and approximate CPU percent). It runs until ctx is done.
func StartRuntimeSampler(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		var lastCPU time.Duration
		lastSample := time.Now()
		numCPU := float64(runtime.NumCPU())

		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				var ms runtime.MemStats
				runtime.ReadMemStats(&ms)
				HeapInUse.Set(float64(ms.HeapInuse))

				// Approximate CPU% from cumulative process CPU time.
				cpu := processCPUTime()
				elapsed := now.Sub(lastSample).Seconds()
				if elapsed > 0 && lastCPU > 0 {
					used := (cpu - lastCPU).Seconds()
					pct := (used / elapsed / numCPU) * 100
					if pct < 0 {
						pct = 0
					}
					CPUUsagePercent.Set(pct)
				}
				lastCPU = cpu
				lastSample = now
			}
		}
	}()
}
