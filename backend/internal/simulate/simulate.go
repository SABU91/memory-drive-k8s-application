// Package simulate provides configurable workload generation: an in-memory
// cache, on-demand memory allocation, and CPU load bursts. These features let
// you deliberately raise resource utilisation so it can be observed in Grafana
// without ever crashing the application.
package simulate

import (
	"crypto/rand"
	"sync"
	"time"

	"memorydrive/internal/metrics"
)

const blockSize = 1 << 20 // 1 MiB per allocation block

// Manager owns all deliberately-held memory: a growable cache and a pool of
// ad-hoc allocations requested via /simulate/memory. It is safe for concurrent
// use.
type Manager struct {
	mu sync.Mutex

	// cache holds long-lived 1 MiB blocks representing an in-memory cache.
	cache [][]byte

	// allocations holds ad-hoc blocks requested at runtime. Each entry can be
	// released automatically after an optional hold duration.
	allocations [][]byte
}

// NewManager creates a Manager and, if enabled, pre-fills the cache to the
// configured size.
func NewManager(enableCache bool, cacheSizeMB int) *Manager {
	m := &Manager{}
	if enableCache && cacheSizeMB > 0 {
		m.SetCacheSizeMB(cacheSizeMB)
	}
	m.publishMetrics()
	return m
}

// SetCacheSizeMB grows or shrinks the cache to the requested size in MiB.
func (m *Manager) SetCacheSizeMB(sizeMB int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sizeMB < 0 {
		sizeMB = 0
	}
	for len(m.cache) < sizeMB {
		m.cache = append(m.cache, newBlock())
	}
	if len(m.cache) > sizeMB {
		// Drop the tail; the trimmed blocks become eligible for GC.
		m.cache = m.cache[:sizeMB]
	}
	m.publishMetricsLocked()
}

// CacheSizeMB returns the current cache size in MiB.
func (m *Manager) CacheSizeMB() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.cache)
}

// TouchCache reads a byte from every cache block, simulating cache access so
// the pages stay resident. Safe to call from background workers.
func (m *Manager) TouchCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	var sink byte
	for _, b := range m.cache {
		if len(b) > 0 {
			sink += b[0]
		}
	}
	_ = sink
}

// Allocate reserves sizeMB of additional memory. If hold > 0 the memory is
// released automatically after that duration; otherwise it is held until the
// process restarts or ReleaseAll is called.
func (m *Manager) Allocate(sizeMB int, hold time.Duration) {
	if sizeMB <= 0 {
		return
	}
	blocks := make([][]byte, 0, sizeMB)
	for i := 0; i < sizeMB; i++ {
		blocks = append(blocks, newBlock())
	}

	m.mu.Lock()
	m.allocations = append(m.allocations, blocks...)
	m.publishMetricsLocked()
	m.mu.Unlock()

	if hold > 0 {
		time.AfterFunc(hold, func() { m.release(blocks) })
	}
}

// ReleaseAll frees all ad-hoc allocations (but keeps the cache).
func (m *Manager) ReleaseAll() {
	m.mu.Lock()
	m.allocations = nil
	m.publishMetricsLocked()
	m.mu.Unlock()
}

// AllocatedMB returns the size of ad-hoc allocations in MiB.
func (m *Manager) AllocatedMB() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.allocations)
}

// release removes a specific batch of blocks from the allocation slice.
func (m *Manager) release(blocks [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	remaining := m.allocations[:0]
	for _, existing := range m.allocations {
		keep := true
		for _, b := range blocks {
			if &existing[0] == &b[0] {
				keep = false
				break
			}
		}
		if keep {
			remaining = append(remaining, existing)
		}
	}
	m.allocations = remaining
	m.publishMetricsLocked()
}

func (m *Manager) publishMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishMetricsLocked()
}

// publishMetricsLocked updates gauges; the caller must hold m.mu.
func (m *Manager) publishMetricsLocked() {
	metrics.CacheSize.Set(float64(len(m.cache)) * blockSize)
	metrics.MemoryAllocated.Set(float64(len(m.allocations)) * blockSize)
}

// newBlock allocates a 1 MiB block filled with random bytes. Random content
// prevents the memory from being deduplicated or optimised away, so it counts
// toward real resident memory.
func newBlock() []byte {
	b := make([]byte, blockSize)
	_, _ = rand.Read(b)
	return b
}
