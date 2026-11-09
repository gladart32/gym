package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gladart32/gym/internal/app"
	"github.com/gladart32/gym/internal/auth"
	"github.com/gladart32/gym/internal/server"
	"github.com/gladart32/gym/internal/testutil"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

type apiEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type tokenData struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

var emailCounter int64

func uniqueEmail() string {
	return fmt.Sprintf("user%d@auth.test", time.Now().UnixNano()%1e9+time.Now().UnixNano()/1e9%1e3)
}

const testPassword = "test-pass-123"

func setup(t *testing.T) (*gin.Engine, *app.App) {
	t.Helper()
	a := testutil.Start(t)
	gin.SetMode(gin.TestMode)
	router := server.NewRouter(a.Log, a.Config,
		handler.NewHealth(a.Log, a.DB, a.Redis),
		handler.NewTasks(a.Log, a.Tasks),
		handler.NewAuth(a.Log, a.Config, a.Auth, a.Tasks),
		a.Auth,
		handler.NewProfile(a.Log, a.Profile),
		a.Profile,
	)
	return router, a
}

func doJSON(t *testing.T, router *gin.Engine, method, path, bearer, cookie string, body any) (*httptest.ResponseRecorder, apiEnvelope) {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var env apiEnvelope
	_ = json.Unmarshal(rec.Body.Bytes(), &env)
	return rec, env
}

// registerVerified creates a user over HTTP and verifies the email through
// the service (the email link normally goes via the worker log).
func registerVerified(t *testing.T, router *gin.Engine, a *app.App, email string) {
	t.Helper()

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/register", "", "", map[string]string{
		"email":    email,
		"password": testPassword,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	verifyEmailOf(t, a, email)
}

func verifyEmailOf(t *testing.T, a *app.App, email string) {
	t.Helper()

	var userID uint32
	if err := a.DB.Get(&userID, `SELECT id FROM users WHERE email = ?`, email); err != nil {
		t.Fatalf("select user: %v", err)
	}

	ctx := context.Background()
	token, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenEmailVerify)
	if err != nil {
		t.Fatalf("create email token: %v", err)
	}
	if err := a.Auth.VerifyEmail(ctx, token); err != nil {
		t.Fatalf("verify email: %v", err)
	}
}

func login(t *testing.T, router *gin.Engine, email string) (access string, refresh string) {
	t.Helper()

	rec, env := doJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email":       email,
		"password":    testPassword,
		"client_type": "mobile",
		"device_name": "iPhone 15",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var tokens tokenData
	if err := json.Unmarshal(env.Data, &tokens); err != nil {
		t.Fatalf("unmarshal tokens: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("login must return both tokens, got %+v", tokens)
	}
	return tokens.AccessToken, tokens.RefreshToken
}

func refresh(t *testing.T, router *gin.Engine, refreshToken string) (int, tokenData) {
	t.Helper()

	rec, env := doJSON(t, router, http.MethodPost, "/api/v1/auth/refresh", "", "", map[string]string{
		"refresh_token": refreshToken,
	})

	var tokens tokenData
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(env.Data, &tokens); err != nil {
			t.Fatalf("unmarshal tokens: %v", err)
		}
	}
	return rec.Code, tokens
}

func TestRegisterLoginAndUnverifiedBlock(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/register", "", "", map[string]string{
		"email": email, "password": testPassword,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	rec, _ = doJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email": email, "password": testPassword,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unverified login: expected 403, got %d", rec.Code)
	}

	verifyEmailOf(t, a, email)

	rec, _ = doJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email": email, "password": testPassword,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("verified login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRefreshRotationAndRace(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	_, refresh1 := login(t, router, email)

	code, tokens := refresh(t, router, refresh1)
	if code != http.StatusOK || tokens.RefreshToken == "" || tokens.RefreshToken == refresh1 {
		t.Fatalf("rotation: expected 200 with a new token, got %d", code)
	}

	var wg sync.WaitGroup
	codes := make([]int, 2)
	shared := tokens.RefreshToken
	for i := range 2 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = refresh(t, router, shared)
		}(i)
	}
	wg.Wait()

	if codes[0]+codes[1] != http.StatusOK+http.StatusUnauthorized {
		t.Fatalf("parallel refresh: expected one 200 and one 401, got %v", codes)
	}
}

func TestRefreshTheftRevokesSession(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	_, refresh1 := login(t, router, email)

	code, tokens := refresh(t, router, refresh1)
	if code != http.StatusOK {
		t.Fatalf("rotation failed: %d", code)
	}

	// test config has tolerance_sec = 0: any reuse of the previous token is theft
	code, _ = refresh(t, router, refresh1)
	if code != http.StatusUnauthorized {
		t.Fatalf("prev token: expected 401, got %d", code)
	}

	code, _ = refresh(t, router, tokens.RefreshToken)
	if code != http.StatusUnauthorized {
		t.Fatalf("session must be revoked after theft: expected 401, got %d", code)
	}
}

func TestRevokedSessionKillsAccess(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	access, _ := login(t, router, email)

	rec, _ := doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", access, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("sessions with live access: expected 200, got %d", rec.Code)
	}

	if _, err := a.DB.Exec(`UPDATE login_sessions SET revoked_at = UTC_TIMESTAMP() WHERE user_id = (SELECT id FROM users WHERE email = ?)`, email); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	rec, _ = doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", access, "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sessions with revoked session: expected 401, got %d", rec.Code)
	}
}

func TestEmailTokenExpiryAndOneShot(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)

	var userID uint32
	if err := a.DB.Get(&userID, `SELECT id FROM users WHERE email = ?`, email); err != nil {
		t.Fatalf("select user: %v", err)
	}

	ctx := context.Background()
	token, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenPasswordReset)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	if _, err := a.DB.Exec(`UPDATE email_action_tokens SET expires_at = UTC_TIMESTAMP() - INTERVAL 1 SECOND WHERE token_hash = ?`, auth.HashToken(token)); err != nil {
		t.Fatalf("expire token: %v", err)
	}

	if _, err := a.Auth.ConsumeEmailToken(ctx, token, auth.TokenPasswordReset); err != auth.ErrTokenInvalid {
		t.Fatalf("expired token: expected ErrTokenInvalid, got %v", err)
	}

	fresh, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenPasswordReset)
	if err != nil {
		t.Fatalf("create fresh token: %v", err)
	}
	if _, err := a.Auth.ConsumeEmailToken(ctx, fresh, auth.TokenPasswordReset); err != nil {
		t.Fatalf("consume fresh token: %v", err)
	}
	if _, err := a.Auth.ConsumeEmailToken(ctx, fresh, auth.TokenPasswordReset); err != auth.ErrTokenInvalid {
		t.Fatalf("second use must fail, got %v", err)
	}
}

func TestChangePasswordRevokesOtherSessions(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	accessKeep, _ := login(t, router, email)
	accessKill, _ := login(t, router, email)

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/password", accessKeep, "", map[string]string{
		"current_password": testPassword,
		"new_password":     "new-pass-456",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("change password: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	rec, _ = doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", accessKeep, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("current session must survive: expected 200, got %d", rec.Code)
	}

	rec, _ = doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", accessKill, "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("other sessions must die: expected 401, got %d", rec.Code)
	}
}

func TestResetPasswordRevokesAllSessions(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	access1, _ := login(t, router, email)
	access2, _ := login(t, router, email)

	var userID uint32
	if err := a.DB.Get(&userID, `SELECT id FROM users WHERE email = ?`, email); err != nil {
		t.Fatalf("select user: %v", err)
	}

	ctx := context.Background()
	token, err := a.Auth.CreateEmailToken(ctx, userID, auth.TokenPasswordReset)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/reset-password", "", "", map[string]string{
		"token":    token,
		"password": "reset-pass-789",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("reset password: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	for i, access := range []string{access1, access2} {
		rec, _ = doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", access, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("session %d must be revoked after reset: expected 401, got %d", i+1, rec.Code)
		}
	}

	rec, _ = doJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email": email, "password": "reset-pass-789",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login with new password: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogout(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)
	access, refreshTok := login(t, router, email)

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/logout", access, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d", rec.Code)
	}

	rec, _ = doJSON(t, router, http.MethodGet, "/api/v1/auth/sessions", access, "", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("access after logout: expected 401, got %d", rec.Code)
	}

	code, _ := refresh(t, router, refreshTok)
	if code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: expected 401, got %d", code)
	}
}

func TestWebLoginSetsRefreshCookie(t *testing.T) {
	router, a := setup(t)
	email := uniqueEmail()
	registerVerified(t, router, a, email)

	rec, _ := doJSON(t, router, http.MethodPost, "/api/v1/auth/login", "", "", map[string]string{
		"email":       email,
		"password":    testPassword,
		"client_type": "web",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("web login: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	cookie := rec.Header().Get("Set-Cookie")
	if cookie == "" {
		t.Fatal("web login must set the gym_refresh cookie")
	}
	if !bytes.Contains([]byte(cookie), []byte("gym_refresh=")) ||
		!bytes.Contains([]byte(cookie), []byte("HttpOnly")) ||
		!bytes.Contains([]byte(cookie), []byte("Path=/api/v1/auth/refresh")) {
		t.Fatalf("unexpected cookie: %s", cookie)
	}

	cookiePair := cookie
	if i := bytes.Index([]byte(cookie), []byte(";")); i >= 0 {
		cookiePair = cookie[:i]
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Cookie", cookiePair)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("refresh by cookie: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
}
