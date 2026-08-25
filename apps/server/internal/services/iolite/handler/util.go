package handler

import (
	"net"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultIceRateLimitPerMinute = 30
	iceRateLimitWindow           = time.Minute
)

var defaultIceAllowedOrigins = map[string]struct{}{
	"http://localhost:5173": {},
	"http://127.0.0.1:5173": {},
}

type iceRateLimitState struct {
	windowStart time.Time
	count       int
}

type iceRateLimiter struct {
	mu     sync.Mutex
	window map[string]iceRateLimitState
}

func newIceRateLimiter() *iceRateLimiter {
	return &iceRateLimiter{
		window: make(map[string]iceRateLimitState),
	}
}

func (l *iceRateLimiter) allow(clientID string) bool {
	if clientID == "" {
		return false
	}

	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	for id, state := range l.window {
		if now.Sub(state.windowStart) >= iceRateLimitWindow {
			delete(l.window, id)
		}
	}

	state, ok := l.window[clientID]
	if !ok || now.Sub(state.windowStart) >= iceRateLimitWindow {
		l.window[clientID] = iceRateLimitState{windowStart: now, count: 1}
		return true
	}

	if state.count >= defaultIceRateLimitPerMinute {
		return false
	}

	state.count++
	l.window[clientID] = state
	return true
}

func setIceCORSHeaders(c *gin.Context, allowed map[string]struct{}) bool {
	origin := normalizeOrigin(c.GetHeader("Origin"))
	if origin == "" {
		return true
	}

	if !isAllowedOrigin(origin, allowed) {
		return false
	}

	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type")
	c.Header("Vary", "Origin")
	return true
}

func parseAllowedOrigins(value string) map[string]struct{} {
	origins := make(map[string]struct{})
	for _, origin := range strings.Split(value, ",") {
		trimmed := normalizeOrigin(origin)
		if trimmed == "" {
			continue
		}

		origins[trimmed] = struct{}{}
	}

	if len(origins) > 0 {
		return origins
	}

	defaults := make(map[string]struct{}, len(defaultIceAllowedOrigins))
	for origin := range defaultIceAllowedOrigins {
		defaults[origin] = struct{}{}
	}

	return defaults
}

func isAllowedOrigin(origin string, allowed map[string]struct{}) bool {
	_, ok := allowed[normalizeOrigin(origin)]
	return ok
}

func normalizeOrigin(origin string) string {
	return strings.TrimRight(strings.TrimSpace(origin), "/")
}

func requestClientID(remoteAddr string, fallback string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(remoteAddr))
	if err == nil && host != "" {
		return host
	}

	remote := strings.TrimSpace(remoteAddr)
	if remote != "" && !strings.Contains(remote, ":") {
		return remote
	}

	return strings.TrimSpace(fallback)
}
