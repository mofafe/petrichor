package httpx

import (
	"github.com/gin-gonic/gin"

	authhandler "github.com/mofafe/petrichor/apps/server/internal/auth/handler"
	authmiddleware "github.com/mofafe/petrichor/apps/server/internal/auth/middleware"
	authservice "github.com/mofafe/petrichor/apps/server/internal/auth/service"

	iolitehandler "github.com/mofafe/petrichor/apps/server/internal/services/iolite/handler"
	iolitews "github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func Routes(hub *iolitews.Hub, authService *authservice.Service) *gin.Engine {
	r := gin.Default()

	authHandler := authhandler.New(authService)
	auth := r.Group("/api/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.GET("/me", authmiddleware.RequireAuth(authService), authHandler.Me)

	ioliteWS := r.Group("/ws/iolite")
	ioliteWS.GET("/signaling/:roomID", iolitehandler.WebSocket(hub, iolitews.ChannelSignaling))
	ioliteWS.GET("/world/:roomID", iolitehandler.WebSocket(hub, iolitews.ChannelWorld))
	ioliteWS.GET("/chat/:roomID", iolitehandler.WebSocket(hub, iolitews.ChannelChat))

	ioliteHandler := r.Group("/api/iolite")
	ioliteHandler.OPTIONS("/ice", iolitehandler.IceOption)
	ioliteHandler.GET("/ice", iolitehandler.IceAPI)

	return r
}
