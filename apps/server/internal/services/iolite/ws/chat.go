package ws

import (
	"log"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

func handleChatMessage(hub *Hub, sender *Client, msg protocol.Message) {
	data, err := buildServerMessage(sender, msg)
	if err != nil {
		log.Println(err)
		return
	}
	hub.broadcast(sender, data)
}
