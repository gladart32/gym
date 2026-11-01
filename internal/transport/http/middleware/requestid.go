package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
)

const (
	HeaderRequestID = "X-Request-ID"
	ctxRequestID    = "request_id"
)

// RequestID takes X-Request-ID from the incoming request or generates a new
// one, stores it in the context and echoes it back in the response.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(HeaderRequestID)
		if id == "" {
			b := make([]byte, 8)
			if _, err := rand.Read(b); err == nil {
				id = hex.EncodeToString(b)
			}
		}

		c.Set(ctxRequestID, id)
		c.Writer.Header().Set(HeaderRequestID, id)
		c.Next()
	}
}

// RequestIDFrom returns the request id stored by the RequestID middleware.
func RequestIDFrom(c *gin.Context) string {
	return c.GetString(ctxRequestID)
}
