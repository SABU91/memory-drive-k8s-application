package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"memorydrive/internal/metrics"
)

// PrometheusMiddleware records request count and latency for every request.
// It uses the matched route template (c.FullPath) as the path label to keep
// metric cardinality bounded.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		status := strconv.Itoa(c.Writer.Status())

		metrics.HTTPRequests.WithLabelValues(c.Request.Method, path, status).Inc()
		metrics.HTTPLatency.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}
