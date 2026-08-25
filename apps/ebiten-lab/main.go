package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/mofafe/petrichor/apps/ebiten-lab/internal/app"
)

func main() {
	cfg := app.Config{}
	flag.StringVar(&cfg.Name, "name", "guest", "player display name")
	flag.StringVar(&cfg.RoomID, "room", "debug", "room id")
	flag.StringVar(&cfg.Server, "server", app.DefaultServer, "iolite server origin")
	flag.Parse()

	game := app.New(cfg)
	ebiten.SetWindowTitle("Iolite Ebiten Lab")
	ebiten.SetWindowSize(app.ScreenWidth, app.ScreenHeight)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
