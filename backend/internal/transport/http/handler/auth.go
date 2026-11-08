// Package-level helpers for the auth endpoints.
package handler

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/config"
	"github.com/gladart32/gym/internal/tasks"
	"github.com/gladart32/gym/internal/transport/http/middleware"
	"github.com/gladart32/gym/internal/transport/http/response"
)

const refreshCookieName = "gym_refresh"
const refreshCookiePath = "/api/v1/auth/refresh"

// Auth exposes the /api/v1/auth endpoints.
type Auth struct {
	log   *slog.Logger
	cfg   *config.Config
	svc   *auth.Service
	tasks *tasks.Client
}

func NewAuth(log *slog.Logger, cfg *config.Config, svc *auth.Service, tasksClient *tasks.Client) *Auth {
	return &Auth{log: log, cfg: cfg, svc: svc, tasks: tasksClient}
}

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
	Name     string `json:"name"`
}

// Register handles POST /api/v1/auth/register.
func (h *Auth) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.Register(c.Request.Context(), req.Email, req.Password, req.Name)
	switch {
	case errors.Is(err, auth.ErrInvalidPassword):
		response.Error(c, http.StatusBadRequest, auth.ErrInvalidPassword.Error())
		return
	case errors.Is(err, auth.ErrEmailTaken):
		response.Error(c, http.StatusConflict, auth.ErrEmailTaken.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "register user",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to register user")
		return
	}

	h.sendEmailToken(c, user.ID, user.Email, "Подтвердите email", auth.TokenEmailVerify, func(token string) string {
		return h.cfg.Auth.WebBaseURL + "/auth/confirm-email?token=" + token
	})

	response.Success(c, http.StatusCreated, gin.H{"user": user})
}

type loginRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required"`
	ClientType  string `json:"client_type"`
	DeviceName  string `json:"device_name"`
	OS          string `json:"os"`
	AppVersion  string `json:"app_version"`
}

// Login handles POST /api/v1/auth/login.
func (h *Auth) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	clientType := req.ClientType
	if clientType == "" {
		clientType = auth.ClientTypeWeb
	}
	if clientType != auth.ClientTypeWeb && clientType != auth.ClientTypeMobile {
		response.Error(c, http.StatusBadRequest, "client_type must be web or mobile")
		return
	}

	in := auth.LoginInput{
		Email:      req.Email,
		Password:   req.Password,
		ClientType: clientType,
		UserAgent:  c.GetHeader("User-Agent"),
		IP:         clientIPBytes(c),
	}
	if clientType == auth.ClientTypeMobile {
		in.DeviceName, in.OS, in.AppVersion = req.DeviceName, req.OS, req.AppVersion
	} else {
		in.DeviceName, in.OS = parseUserAgent(in.UserAgent)
	}

	tokens, err := h.svc.Login(c.Request.Context(), in)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, auth.ErrInvalidCredentials.Error())
		return
	case errors.Is(err, auth.ErrEmailNotVerified):
		response.Error(c, http.StatusForbidden, auth.ErrEmailNotVerified.Error())
		return
	case errors.Is(err, auth.ErrBlocked):
		response.Error(c, http.StatusForbidden, auth.ErrBlocked.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "login",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to login")
		return
	}

	h.respondTokens(c, http.StatusOK, tokens)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh handles POST /api/v1/auth/refresh: web sends the refresh token in
// the HttpOnly cookie, mobile in the JSON body.
func (h *Auth) Refresh(c *gin.Context) {
	token, err := c.Cookie(refreshCookieName)
	if err != nil || token == "" {
		var req refreshRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.RefreshToken == "" {
			response.Error(c, http.StatusUnauthorized, "missing refresh token")
			return
		}
		token = req.RefreshToken
	}

	tokens, err := h.svc.Refresh(c.Request.Context(), token)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, auth.ErrTokenInvalid.Error())
		return
	}

	h.respondTokens(c, http.StatusOK, tokens)
}

type verifyEmailRequest struct {
	Token string `json:"token" binding:"required"`
}

// VerifyEmail handles POST /api/v1/auth/verify-email.
func (h *Auth) VerifyEmail(c *gin.Context) {
	var req verifyEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.VerifyEmail(c.Request.Context(), req.Token); err != nil {
		if errors.Is(err, auth.ErrTokenInvalid) {
			response.Error(c, http.StatusBadRequest, "verification link is invalid or expired")
			return
		}
		h.log.ErrorContext(c.Request.Context(), "verify email",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to verify email")
		return
	}

	response.Success(c, http.StatusOK, nil)
}

// Logout handles POST /api/v1/auth/logout (requires Auth middleware).
func (h *Auth) Logout(c *gin.Context) {
	err := h.svc.RevokeSession(c.Request.Context(), middleware.SIDFrom(c), middleware.UserIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "logout",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to logout")
		return
	}

	h.clearRefreshCookie(c)
	response.Success(c, http.StatusOK, nil)
}

// ListSessions handles GET /api/v1/auth/sessions (requires Auth middleware).
func (h *Auth) ListSessions(c *gin.Context) {
	sessions, err := h.svc.ListSessions(c.Request.Context(), middleware.UserIDFrom(c), middleware.SIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "list sessions",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	response.Success(c, http.StatusOK, gin.H{"sessions": sessions})
}

// RevokeSessionByID handles DELETE /api/v1/auth/sessions/:id.
func (h *Auth) RevokeSessionByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "invalid session id")
		return
	}

	if err := h.svc.RevokeSession(c.Request.Context(), id, middleware.UserIDFrom(c)); err != nil {
		h.log.ErrorContext(c.Request.Context(), "revoke session",
			"error", err, "request_id", middleware.RequestIDFrom(c), "session_id", id)
		response.Error(c, http.StatusInternalServerError, "failed to revoke session")
		return
	}

	response.Success(c, http.StatusOK, nil)
}

// RevokeAllSessions handles POST /api/v1/auth/sessions/revoke-all.
func (h *Auth) RevokeAllSessions(c *gin.Context) {
	err := h.svc.RevokeAllExcept(c.Request.Context(), middleware.UserIDFrom(c), middleware.SIDFrom(c))
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "revoke all sessions",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to revoke sessions")
		return
	}

	response.Success(c, http.StatusOK, nil)
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// ChangePassword handles POST /api/v1/auth/password (requires Auth middleware).
func (h *Auth) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.ChangePassword(c.Request.Context(),
		middleware.UserIDFrom(c), middleware.SIDFrom(c),
		req.CurrentPassword, req.NewPassword)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		response.Error(c, http.StatusUnauthorized, "current password is incorrect")
		return
	case errors.Is(err, auth.ErrInvalidPassword):
		response.Error(c, http.StatusBadRequest, auth.ErrInvalidPassword.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "change password",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to change password")
		return
	}

	response.Success(c, http.StatusOK, nil)
}

type emailOnlyRequest struct {
	Email string `json:"email" binding:"required,email"`
}

// ForgotPassword handles POST /api/v1/auth/forgot-password. The response is
// uniform: it never reveals whether the email is registered.
func (h *Auth) ForgotPassword(c *gin.Context) {
	var req emailOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.UserBriefByEmail(c.Request.Context(), req.Email)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "forgot password lookup",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to process request")
		return
	}
	if user != nil {
		h.sendEmailToken(c, user.ID, user.Email, "Сброс пароля", auth.TokenPasswordReset, func(token string) string {
			return h.cfg.Auth.WebBaseURL + "/auth/reset-password?token=" + token
		})
	}

	response.Success(c, http.StatusOK, nil)
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ResetPassword handles POST /api/v1/auth/reset-password.
func (h *Auth) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	err := h.svc.ResetPassword(c.Request.Context(), req.Token, req.Password)
	switch {
	case errors.Is(err, auth.ErrTokenInvalid):
		response.Error(c, http.StatusBadRequest, "reset link is invalid or expired")
		return
	case errors.Is(err, auth.ErrInvalidPassword):
		response.Error(c, http.StatusBadRequest, auth.ErrInvalidPassword.Error())
		return
	case err != nil:
		h.log.ErrorContext(c.Request.Context(), "reset password",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to reset password")
		return
	}

	response.Success(c, http.StatusOK, nil)
}

// ResendVerification handles POST /api/v1/auth/resend-verification. The
// response is uniform regardless of whether the email exists or is verified.
func (h *Auth) ResendVerification(c *gin.Context) {
	var req emailOnlyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.UnverifiedUserByEmail(c.Request.Context(), req.Email)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "resend verification lookup",
			"error", err, "request_id", middleware.RequestIDFrom(c))
		response.Error(c, http.StatusInternalServerError, "failed to process request")
		return
	}
	if user != nil {
		h.sendEmailToken(c, user.ID, user.Email, "Подтвердите email", auth.TokenEmailVerify, func(token string) string {
			return h.cfg.Auth.WebBaseURL + "/auth/confirm-email?token=" + token
		})
	}

	response.Success(c, http.StatusOK, nil)
}

func (h *Auth) respondTokens(c *gin.Context, code int, t *auth.IssuedTokens) {
	if t.ClientType == auth.ClientTypeWeb {
		h.setRefreshCookie(c, t.RefreshToken)
	}

	data := gin.H{
		"access_token": t.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   t.ExpiresIn,
		"user":         t.User,
	}
	if t.ClientType == auth.ClientTypeMobile {
		data["refresh_token"] = t.RefreshToken
	}
	response.Success(c, code, data)
}

func (h *Auth) setRefreshCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, token,
		int(h.cfg.Auth.RefreshTTL.Seconds()),
		refreshCookiePath, "",
		h.cfg.Env == "prod", true,
	)
}

func (h *Auth) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1,
		refreshCookiePath, "",
		h.cfg.Env == "prod", true,
	)
}

// sendEmailToken issues a one-time token, builds the link and enqueues the
// email task. Enqueue failures are logged but do not fail the request:
// the user can request the email again.
func (h *Auth) sendEmailToken(c *gin.Context, userID uint32, to, subject, tokenType string, link func(token string) string) {
	token, err := h.svc.CreateEmailToken(c.Request.Context(), userID, tokenType)
	if err != nil {
		h.log.ErrorContext(c.Request.Context(), "create email token",
			"error", err, "request_id", middleware.RequestIDFrom(c), "user_id", userID)
		return
	}

	if _, err := h.tasks.EnqueueSendEmail(c.Request.Context(), tasks.SendEmailPayload{
		To:      to,
		Subject: subject,
		Body:    link(token),
	}); err != nil {
		h.log.WarnContext(c.Request.Context(), "enqueue send_email",
			"error", err, "request_id", middleware.RequestIDFrom(c), "user_id", userID)
	}
}

func clientIPBytes(c *gin.Context) []byte {
	ip := net.ParseIP(c.ClientIP())
	if ip == nil {
		return nil
	}
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	return ip.To16()
}
