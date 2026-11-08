package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/transport/http/response"
)

const (
	ctxUserID     = "auth_user_id"
	ctxSID        = "auth_sid"
	ctxClientType = "auth_client_type"
	ctxRole       = "auth_role"
)

// Auth validates the Bearer access token in two steps (docs/AUTH.md, п.2):
// signature and exp without the database, then a single SELECT by sid that
// the session is still alive. On success it stores user_id, sid,
// client_type and role in the request context.
func Auth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			response.Error(c, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}

		claims, err := svc.JWT().ParseAccess(token)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired access token")
			return
		}

		userID, err := claims.UserID()
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired access token")
			return
		}

		active, err := svc.SessionActive(c.Request.Context(), claims.SID, userID)
		if err != nil {
			response.Error(c, http.StatusInternalServerError, "session check failed")
			return
		}
		if !active {
			response.Error(c, http.StatusUnauthorized, "session revoked or expired")
			return
		}

		c.Set(ctxUserID, userID)
		c.Set(ctxSID, claims.SID)
		c.Set(ctxClientType, claims.ClientType)
		c.Set(ctxRole, claims.Role)
		c.Next()
	}
}

// Admin requires the authenticated user to have the admin role.
func Admin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if RoleFrom(c) != auth.RoleAdmin {
			response.Error(c, http.StatusForbidden, "admin role required")
			return
		}
		c.Next()
	}
}

func bearerToken(c *gin.Context) (string, bool) {
	const prefix = "Bearer "
	header := c.GetHeader("Authorization")
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

// UserIDFrom returns the authenticated user id stored by the Auth middleware.
func UserIDFrom(c *gin.Context) uint32 {
	id, _ := c.Value(ctxUserID).(uint32)
	return id
}

// SIDFrom returns the login-session id stored by the Auth middleware.
func SIDFrom(c *gin.Context) uint64 {
	sid, _ := c.Value(ctxSID).(uint64)
	return sid
}

// ClientTypeFrom returns the client type stored by the Auth middleware.
func ClientTypeFrom(c *gin.Context) string {
	return c.GetString(ctxClientType)
}

// RoleFrom returns the user role stored by the Auth middleware.
func RoleFrom(c *gin.Context) string {
	return c.GetString(ctxRole)
}
