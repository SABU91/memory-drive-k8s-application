// Package handlers contains the HTTP handlers and middleware that expose the
// Memory Drive REST API.
package handlers

import (
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"memorydrive/internal/config"
	"memorydrive/internal/db"
	"memorydrive/internal/metrics"
	"memorydrive/internal/models"
	"memorydrive/internal/simulate"
	"memorydrive/internal/storage"
	"memorydrive/internal/workers"
)

// Handlers bundles the dependencies every handler needs.
type Handlers struct {
	cfg       *config.Config
	store     *db.Store
	blobs     *storage.Store
	mem       *simulate.Manager
	pool      *workers.Pool
	startTime time.Time
}

// New builds a Handlers value.
func New(cfg *config.Config, store *db.Store, blobs *storage.Store, mem *simulate.Manager, pool *workers.Pool) *Handlers {
	return &Handlers{
		cfg:       cfg,
		store:     store,
		blobs:     blobs,
		mem:       mem,
		pool:      pool,
		startTime: time.Now(),
	}
}

// Health is a liveness/readiness probe target.
func (h *Handlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
}

// Upload accepts either a typed note (form field "content") or an uploaded
// file (form field "file"). The kind is inferred from the content type.
func (h *Handlers) Upload(c *gin.Context) {
	name := strings.TrimSpace(c.PostForm("name"))
	content := c.PostForm("content")

	// Case 1: a typed note (no file part, but text content provided).
	fileHeader, fileErr := c.FormFile("file")
	if fileErr != nil {
		if strings.TrimSpace(content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "provide either a file or note content"})
			return
		}
		if name == "" {
			name = "Untitled note"
		}
		f := &models.File{
			ID:          uuid.NewString(),
			Name:        name,
			Kind:        models.KindNote,
			ContentType: "text/plain; charset=utf-8",
			Size:        int64(len(content)),
			TextContent: content,
			CreatedAt:   time.Now().UTC(),
		}
		if err := h.store.Insert(f); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		metrics.Uploads.WithLabelValues(string(models.KindNote)).Inc()
		metrics.UploadSize.Observe(float64(f.Size))
		c.JSON(http.StatusCreated, f)
		return
	}

	// Case 2: an uploaded file.
	maxBytes := int64(h.cfg.MaxUploadMB) << 20
	if fileHeader.Size > maxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{
			"error": "file exceeds max size of " + strconv.Itoa(h.cfg.MaxUploadMB) + " MB",
		})
		return
	}

	contentType := fileHeader.Header.Get("Content-Type")
	kind := models.KindText
	if strings.HasPrefix(contentType, "image/") {
		kind = models.KindImage
	}
	if name == "" {
		name = fileHeader.Filename
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read upload: " + err.Error()})
		return
	}
	defer src.Close()

	id := uuid.NewString()

	// For text files we also capture the content in the DB so it is searchable.
	var textContent string
	var reader io.Reader = src
	if kind == models.KindText {
		data, err := io.ReadAll(io.LimitReader(src, maxBytes))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read upload: " + err.Error()})
			return
		}
		textContent = string(data)
		reader = strings.NewReader(textContent)
	}

	size, path, err := h.blobs.Save(id, reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	f := &models.File{
		ID:          id,
		Name:        name,
		Kind:        kind,
		ContentType: firstNonEmpty(contentType, "application/octet-stream"),
		Size:        size,
		StoragePath: path,
		TextContent: textContent,
		CreatedAt:   time.Now().UTC(),
	}
	if err := h.store.Insert(f); err != nil {
		_ = h.blobs.Delete(id) // roll back the blob so we do not leak files
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	metrics.Uploads.WithLabelValues(string(kind)).Inc()
	metrics.UploadSize.Observe(float64(size))
	c.JSON(http.StatusCreated, f)
}

// ListFiles returns file metadata, optionally filtered by ?search=.
func (h *Handlers) ListFiles(c *gin.Context) {
	search := strings.TrimSpace(c.Query("search"))
	files, err := h.store.List(search)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if files == nil {
		files = []*models.File{}
	}
	c.JSON(http.StatusOK, gin.H{"count": len(files), "files": files})
}

// GetFile returns the raw content of a single item: notes and text files are
// served as text/plain, images are served with their stored content type.
func (h *Handlers) GetFile(c *gin.Context) {
	id := c.Param("id")
	f, err := h.store.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if f == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if f.Kind == models.KindNote {
		c.Data(http.StatusOK, f.ContentType, []byte(f.TextContent))
		return
	}

	blob, err := h.blobs.Open(f.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "blob missing on disk"})
		return
	}
	defer blob.Close()

	c.Header("Content-Type", f.ContentType)
	c.Header("Content-Disposition", "inline; filename=\""+f.Name+"\"")
	http.ServeContent(c.Writer, c.Request, f.Name, f.CreatedAt, blob)
}

// DeleteFile removes an item's metadata and its blob (if any).
func (h *Handlers) DeleteFile(c *gin.Context) {
	id := c.Param("id")
	f, err := h.store.Get(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if f == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if f.Kind != models.KindNote {
		if err := h.blobs.Delete(f.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	if _, err := h.store.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}

// simulateMemoryRequest is the body of POST /simulate/memory.
type simulateMemoryRequest struct {
	MegaBytes   int `json:"megabytes"`
	HoldSeconds int `json:"holdSeconds"` // 0 = hold until process restart
	CacheSizeMB *int `json:"cacheSizeMB"` // optional: resize the cache instead
}

// SimulateMemory allocates memory on demand (or resizes the cache) so resource
// usage can be observed. It never crashes the process.
func (h *Handlers) SimulateMemory(c *gin.Context) {
	var req simulateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	if req.CacheSizeMB != nil {
		h.mem.SetCacheSizeMB(*req.CacheSizeMB)
	}
	if req.MegaBytes > 0 {
		hold := time.Duration(req.HoldSeconds) * time.Second
		h.mem.Allocate(req.MegaBytes, hold)
	}

	c.JSON(http.StatusOK, gin.H{
		"allocatedMB": h.mem.AllocatedMB(),
		"cacheMB":     h.mem.CacheSizeMB(),
	})
}

// simulateLoadRequest is the body of POST /simulate/load.
type simulateLoadRequest struct {
	DurationSeconds int  `json:"durationSeconds"`
	Workers         int  `json:"workers"`
	Async           bool `json:"async"` // if true, return immediately
}

// SimulateLoad drives CPU utilisation for a bounded duration.
func (h *Handlers) SimulateLoad(c *gin.Context) {
	var req simulateLoadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if req.DurationSeconds <= 0 {
		req.DurationSeconds = 5
	}
	if req.Workers <= 0 {
		req.Workers = runtime.NumCPU()
	}
	dur := time.Duration(req.DurationSeconds) * time.Second

	if req.Async {
		go simulate.RunCPULoad(req.Workers, dur)
		c.JSON(http.StatusAccepted, gin.H{
			"status": "started", "workers": req.Workers, "durationSeconds": req.DurationSeconds,
		})
		return
	}

	simulate.RunCPULoad(req.Workers, dur)
	c.JSON(http.StatusOK, gin.H{
		"status": "completed", "workers": req.Workers, "durationSeconds": req.DurationSeconds,
	})
}

// Stats returns a human-friendly snapshot of application state, mirroring much
// of what is available on /metrics.
func (h *Handlers) Stats(c *gin.Context) {
	count, totalSize, err := h.store.Stats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	c.JSON(http.StatusOK, gin.H{
		"uptimeSeconds":   int64(time.Since(h.startTime).Seconds()),
		"files":           count,
		"totalFileBytes":  totalSize,
		"cacheMB":         h.mem.CacheSizeMB(),
		"allocatedMB":     h.mem.AllocatedMB(),
		"workerCount":     h.cfg.WorkerCount,
		"goroutines":      runtime.NumGoroutine(),
		"heapAllocBytes":  ms.HeapAlloc,
		"heapInuseBytes":  ms.HeapInuse,
		"sysBytes":        ms.Sys,
		"numGC":           ms.NumGC,
		"config": gin.H{
			"memoryCacheEnabled":  h.cfg.EnableMemoryCache,
			"backgroundWorkers":   h.cfg.EnableBackgroundWorkers,
			"maxUploadMB":         h.cfg.MaxUploadMB,
		},
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
