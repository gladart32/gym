package response

import (
	"github.com/gin-gonic/gin"
)

const (
	statusSuccess = "success"
	statusError   = "error"
)

type envelope struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// Success writes a unified success envelope: {"status":"success","data":...}.
// The data field is omitted when data is nil.
func Success(c *gin.Context, code int, data any) {
	c.JSON(code, envelope{Status: statusSuccess, Data: data})
}

// Error writes a unified error envelope: {"status":"error","message":...}
// and aborts the handler chain, so it is safe to use in middleware.
func Error(c *gin.Context, code int, msg string) {
	c.AbortWithStatusJSON(code, envelope{Status: statusError, Message: msg})
}
