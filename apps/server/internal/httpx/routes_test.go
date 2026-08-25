package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func TestIceEndpointAddsCORSHeadersForAllowedOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TURN_SECRET", "test-secret")
	t.Setenv("ICE_ALLOWED_ORIGINS", "https://dev.petrichor.example.com")

	r := Routes(ws.NewHub())

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

	r := Routes(ws.NewHub())

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

	r := Routes(ws.NewHub())

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
