package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gladart32/gym/internal/server"
	"github.com/gladart32/gym/internal/testutil"
	"github.com/gladart32/gym/internal/transport/http/handler"
)

// TestReadiness is an integration test: it requires the test infrastructure
// (make test-db-up) and verifies migrations, MySQL and Redis connectivity.
func TestReadiness(t *testing.T) {
	a := testutil.Start(t)

	healthH := handler.NewHealth(a.Log, a.DB, a.Redis)
	tasksH := handler.NewTasks(a.Log, a.Tasks)
	router := server.NewRouter(a.Log, a.Config, healthH, tasksH)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d, body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"mysql":"ok"`, `"redis":"ok"`, `"status":"success"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("body %q does not contain %q", rec.Body.String(), want)
		}
	}
}
