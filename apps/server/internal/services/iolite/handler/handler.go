package handler

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func WebSocket(hub *ws.Hub, channel ws.Channel) gin.HandlerFunc {
	return func(c *gin.Context) {
		roomID := c.Param("roomID")
		ws.HandleWS(hub, roomID, channel, c.Writer, c.Request)
	}
}

var limiter = newIceRateLimiter()

func IceOption(c *gin.Context) {
	if !setIceCORSHeaders(c, currentAllowedIceOrigins()) {
		c.JSON(403, gin.H{"error": "origin is not allowed"})
		return
	}

	c.Status(204)
}

func IceAPI(c *gin.Context) {
	turnSecret := currentTurnSecret()
	if turnSecret == "" {
		c.JSON(503, gin.H{"error": "TURN_SECRET is not configured"})
		return
	}

	if !setIceCORSHeaders(c, currentAllowedIceOrigins()) {
		c.JSON(403, gin.H{"error": "origin is not allowed"})
		return
	}

	clientID := requestClientID(c.Request.RemoteAddr, c.ClientIP())
	if !limiter.allow(clientID) {
		c.JSON(429, gin.H{"error": "too many requests"})
		return
	}

	c.JSON(200, buildICEResponse(clientID, turnSecret, currentTurnRealm()))
}

func currentTurnSecret() string {
	return strings.TrimSpace(os.Getenv("TURN_SECRET"))
}

func currentTurnRealm() string {
	realm := strings.TrimSpace(os.Getenv("TURN_REALM"))
	if realm == "" {
		return "turn.example.com"
	}

	return realm
}

func currentAllowedIceOrigins() map[string]struct{} {
	return parseAllowedOrigins(os.Getenv("ICE_ALLOWED_ORIGINS"))
}
