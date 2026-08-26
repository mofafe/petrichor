package main

import (
	"log"
	"os"

	"github.com/mofafe/petrichor/apps/server/internal/auth/repository"
	"github.com/mofafe/petrichor/apps/server/internal/auth/service"
	"github.com/mofafe/petrichor/apps/server/internal/db"
	"github.com/mofafe/petrichor/apps/server/internal/httpx"
	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/ws"
)

func main() {
	conn, err := db.Open(os.Getenv("PETRICHOR_DATABASE_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	authService, err := service.New(repository.NewUserRepository(conn), os.Getenv("JWT_SECRET"))
	if err != nil {
		log.Fatal(err)
	}

	hub := ws.NewHub()
	r := httpx.Routes(hub, authService)
	r.Run(":8080")
}
