package main

import (
	"github.com/mofafe/petrichor/apps/server/internal/httpx"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func main() {
	hub := ws.NewHub()
	r := httpx.Routes(hub)
	r.Run(":8080")
}
