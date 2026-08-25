package ws

import (
	"errors"
	"log"
)

// broadcast 送信者含め全員に送信
func (h *Hub) broadcast(sender *Client, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	for id, client := range room.clientsFor(sender.channel()) {
		select {
		case client.Send <- data:
		default:
			log.Println("send buffer full:", id)
		}
	}
}

// broadcastExcept 送信者(sender.ID)以外の全員に送信
func (h *Hub) broadcastExcept(sender *Client, data []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	for id, client := range room.clientsFor(sender.channel()) {
		if id == sender.ID {
			continue
		}

		select {
		case client.Send <- data:
		default:
			log.Println("send buffer full:", id)
		}
	}
}

// sendTo targetIDで送信先を指定して送信
func (h *Hub) sendTo(sender *Client, targetID string, data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return errors.New("room not found")
	}

	client, ok := room.clientsFor(sender.channel())[targetID]

	if !ok {
		return errors.New("client not found")
	}

	select {
	case client.Send <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

func (r *Room) clientsFor(channel Channel) map[string]*Client {
	switch channel {
	case ChannelSignaling:
		return r.SignalingClients
	case ChannelChat:
		return r.ChatClients
	default:
		return r.Clients
	}
}

func (h *Hub) sendToClient(client *Client, data []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[client.RoomID]
	if !ok {
		return errors.New("room not found")
	}
	if room.Clients[client.ID] != client {
		return errors.New("client not found")
	}

	select {
	case client.Send <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}
