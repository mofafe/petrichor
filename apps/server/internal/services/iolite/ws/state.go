package ws

import (
	"encoding/json"
	"log"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

func (h *Hub) setPlayerState(sender *Client, state protocol.PlayerState) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	if room.Players == nil {
		room.Players = make(map[string]protocol.PlayerState)
	}

	room.Players[sender.ID] = state
}

func (h *Hub) updatePlayerMove(sender *Client, payload protocol.MovePayload) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	state, ok := room.Players[sender.ID]
	if !ok {
		return
	}

	state.X = payload.X
	state.Y = payload.Y
	state.Rotation = payload.Rotation
	room.Players[sender.ID] = state
}

func (h *Hub) updatePlayerSpeaking(sender *Client, speaking bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	state, ok := room.Players[sender.ID]
	if !ok {
		return
	}

	state.Speaking = speaking
	room.Players[sender.ID] = state
}

func (h *Hub) deletePlayerState(sender *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return
	}

	delete(room.Players, sender.ID)
}

func (h *Hub) playerStates(sender *Client) []protocol.PlayerState {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[sender.RoomID]
	if !ok {
		return nil
	}

	players := make([]protocol.PlayerState, 0, len(room.Players))
	for _, state := range room.Players {
		players = append(players, state)
	}

	return players
}

func (h *Hub) sendStateSync(sender *Client) {
	payload := protocol.StateSyncPayload{
		Players: h.playerStates(sender),
	}

	payloadData, err := json.Marshal(payload)
	if err != nil {
		log.Println(err)
		return
	}

	msg, err := json.Marshal(protocol.Message{
		T: protocol.EventStateSync,
		D: payloadData,
	})
	if err != nil {
		log.Println(err)
		return
	}

	if err := h.sendToClient(sender, msg); err != nil {
		log.Println(err)
	}
}
