// Command memorydrive is the Memory Drive backend: a small Gin HTTP server that
// stores notes, text files and images, exposes Prometheus metrics, and offers
// configurable workload generation for Kubernetes observability practice.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"memorydrive/internal/config"
	"memorydrive/internal/db"
	"memorydrive/internal/handlers"
	"memorydrive/internal/metrics"
	"memorydrive/internal/simulate"
	"memorydrive/internal/storage"
	"memorydrive/internal/workers"
)

func main() {
	cfg := config.Load()
	gin.SetMode(cfg.GinMode)

	// Ensure the data directory (parent of the DB file) exists on the volume.
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	store, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	blobs, err := storage.New(cfg.UploadDir)
	if err != nil {
		log.Fatalf("init storage: %v", err)
	}

	// Workload generation: in-memory cache + ad-hoc allocations.
	mem := simulate.NewManager(cfg.EnableMemoryCache, cfg.CacheSizeMB)
	if cfg.BaselineMemoryMB > 0 {
		mem.Allocate(cfg.BaselineMemoryMB, 0) // held for the process lifetime
		log.Printf("allocated %d MB baseline memory", cfg.BaselineMemoryMB)
	}

	// Background worker pool (optional).
	var pool *workers.Pool
	if cfg.EnableBackgroundWorkers {
		pool = workers.NewPool(cfg.WorkerCount, cfg.WorkerInterval, mem)
		pool.Start()
	}

	// Runtime metric sampler (heap + CPU%).
	stopSampler := make(chan struct{})
	metrics.StartRuntimeSampler(stopSampler)

	h := handlers.New(cfg, store, blobs, mem, pool)
	router := buildRouter(h)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Run the server and wait for a shutdown signal.
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	close(stopSampler)
	if pool != nil {
		pool.Stop()
	}
	log.Println("bye")
}

// buildRouter wires up middleware and all routes.
func buildRouter(h *handlers.Handlers) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(handlers.PrometheusMiddleware())

	// Permissive CORS so the Vite dev server (a different origin) can call the
	// API during local development. In-cluster the Ingress makes them same-origin.
	router.Use(cors.New(cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Accept"},
		MaxAge:          12 * time.Hour,
	}))

	// Operational endpoints.
	router.GET("/health", h.Health)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Application API.
	router.POST("/upload", h.Upload)
	router.GET("/files", h.ListFiles)
	router.GET("/files/:id", h.GetFile)
	router.DELETE("/files/:id", h.DeleteFile)
	router.GET("/stats", h.Stats)

	// Workload generation.
	router.POST("/simulate/memory", h.SimulateMemory)
	router.POST("/simulate/load", h.SimulateLoad)

	return router
}
