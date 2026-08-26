package httpx

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/auth/repository"
	"github.com/mofafe/petrichor/apps/server/internal/auth/service"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
	_ "modernc.org/sqlite"
)

func TestIceEndpointAddsCORSHeadersForAllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("ICE_ALLOWED_ORIGINS", "https://dev.petrichor.example.com")

	r := Routes(ws.NewHub(), testAuthService(t))

	req := httptest.NewRequest(http.MethodGet, "/api/ice", nil)
	req.Header.Set("Origin", "https://dev.petrichor.example.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dev.petrichor.example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin header, got %q", got)
	}

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary: Origin, got %q", got)
	}
}

func TestIceEndpointAllowsSameOriginRequestWithoutCORSHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("ICE_ALLOWED_ORIGINS", "https://dev.iolite.example.com")

	r := Routes(ws.NewHub(), testAuthService(t))

	req := httptest.NewRequest(http.MethodGet, "/api/ice", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no Access-Control-Allow-Origin header, got %q", got)
	}
}

func TestIceEndpointHandlesAllowedPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("ICE_ALLOWED_ORIGINS", "https://dev.petrichor.example.com")

	r := Routes(ws.NewHub(), testAuthService(t))

	req := httptest.NewRequest(http.MethodOptions, "/api/ice", nil)
	req.Header.Set("Origin", "https://dev.petrichor.example.com")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://dev.petrichor.example.com" {
		t.Fatalf("expected Access-Control-Allow-Origin header, got %q", got)
	}
}

func testAuthService(t *testing.T) *service.Service {
	t.Helper()

	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })

	_, err = conn.Exec(`CREATE TABLE users (
		id TEXT PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`)
	if err != nil {
		t.Fatal(err)
	}

	authService, err := service.New(repository.NewUserRepository(conn), "test-secret")
	if err != nil {
		t.Fatal(err)
	}

	return authService
}
