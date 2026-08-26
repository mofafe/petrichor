package handler

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/auth/middleware"
	"github.com/mofafe/petrichor/apps/server/internal/auth/repository"
	"github.com/mofafe/petrichor/apps/server/internal/auth/service"
	_ "modernc.org/sqlite"
)

func TestAuthRegisterLoginAndMe(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := testRouter(t)

	registerReq := `{"username":"example","password":"password123"}`
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(registerReq)))

	if registerRec.Code != http.StatusCreated {
		t.Fatalf("expected register status %d, got %d: %s", http.StatusCreated, registerRec.Code, registerRec.Body.String())
	}
	if strings.Contains(registerRec.Body.String(), "password_hash") {
		t.Fatalf("register response must not include password_hash: %s", registerRec.Body.String())
	}

	loginReq := `{"username":"example","password":"password123"}`
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(loginReq)))

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected login status %d, got %d: %s", http.StatusOK, loginRec.Code, loginRec.Body.String())
	}

	var loginBody struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginBody); err != nil {
		t.Fatal(err)
	}
	if loginBody.Token == "" {
		t.Fatal("expected login token")
	}

	meRec := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginBody.Token)
	router.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected me status %d, got %d: %s", http.StatusOK, meRec.Code, meRec.Body.String())
	}
	if strings.Contains(meRec.Body.String(), "password_hash") {
		t.Fatalf("me response must not include password_hash: %s", meRec.Body.String())
	}
}

func TestAuthRejectsDuplicateUsername(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := testRouter(t)

	req := `{"username":"example","password":"password123"}`
	firstRec := httptest.NewRecorder()
	router.ServeHTTP(firstRec, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(req)))
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("expected first register status %d, got %d: %s", http.StatusCreated, firstRec.Code, firstRec.Body.String())
	}

	secondRec := httptest.NewRecorder()
	router.ServeHTTP(secondRec, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(req)))
	if secondRec.Code != http.StatusConflict {
		t.Fatalf("expected duplicate status %d, got %d: %s", http.StatusConflict, secondRec.Code, secondRec.Body.String())
	}
}

func TestAuthLoginFailureDoesNotRevealReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := testRouter(t)

	registerReq := `{"username":"example","password":"password123"}`
	registerRec := httptest.NewRecorder()
	router.ServeHTTP(registerRec, httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader(registerReq)))

	wrongPasswordRec := httptest.NewRecorder()
	router.ServeHTTP(wrongPasswordRec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"example","password":"wrong-password"}`)))

	missingUserRec := httptest.NewRecorder()
	router.ServeHTTP(missingUserRec, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"missing","password":"wrong-password"}`)))

	if wrongPasswordRec.Code != http.StatusUnauthorized || missingUserRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected both failures to be unauthorized, got %d and %d", wrongPasswordRec.Code, missingUserRec.Code)
	}
	if wrongPasswordRec.Body.String() != missingUserRec.Body.String() {
		t.Fatalf("expected identical failure bodies, got %q and %q", wrongPasswordRec.Body.String(), missingUserRec.Body.String())
	}
}

func TestAuthMeRequiresBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router, _ := testRouter(t)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}
}

func testRouter(t *testing.T) (*gin.Engine, *service.Service) {
	t.Helper()

	authService := testService(t)
	authHandler := New(authService)
	router := gin.New()
	auth := router.Group("/api/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.GET("/me", middleware.RequireAuth(authService), authHandler.Me)

	return router, authService
}

func testService(t *testing.T) *service.Service {
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
