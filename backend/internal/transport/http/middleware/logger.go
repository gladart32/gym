package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger logs every request via slog.
func Logger(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		attrs := []any{
			"request_id", RequestIDFrom(c),
			"method", c.Request.Method,
			"path", path,
			"query", query,
			"status", c.Writer.Status(),
			"latency_ms", float64(time.Since(start).Microseconds()) / 1000.0,
			"client_ip", c.ClientIP(),
			"user_agent", c.Request.UserAgent(),
			"errors_count", len(c.Errors),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, "errors", c.Errors.String())
		}

		log.InfoContext(c.Request.Context(), "http request", attrs...)
	}
}
