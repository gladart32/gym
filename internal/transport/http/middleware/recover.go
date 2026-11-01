package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/transport/http/response"
)

// Recover converts panics into 500 responses and logs the stack trace.
func Recover(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorContext(c.Request.Context(), "panic recovered",
					"error", r,
					"request_id", RequestIDFrom(c),
					"stack", string(debug.Stack()),
				)
				response.Error(c, http.StatusInternalServerError, "internal server error")
			}
		}()
		c.Next()
	}
}
