package httpx

import (
	"github.com/gin-gonic/gin"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/handler"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func Routes(hub *ws.Hub) *gin.Engine {
	r := gin.Default()

	iolite := r.Group("/")

	iolite.GET("/ws/rooms/signaling/:roomID", handler.WebSocket(hub, ws.ChannelSignaling))
	iolite.GET("/ws/rooms/world/:roomID", handler.WebSocket(hub, ws.ChannelWorld))
	iolite.GET("/ws/rooms/chat/:roomID", handler.WebSocket(hub, ws.ChannelChat))

	iolite.OPTIONS("/api/ice", handler.IceOption)

	iolite.GET("/api/ice", handler.IceAPI)

	return r
}
