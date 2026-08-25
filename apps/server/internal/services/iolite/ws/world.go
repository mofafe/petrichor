package ws

import (
	"encoding/json"
	"log"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

func handleWorldMessage(hub *Hub, sender *Client, msg protocol.Message) {
	data, err := buildServerMessage(sender, msg)
	if err != nil {
		log.Println(err)
		return
	}
	switch msg.T {
	case protocol.EventJoin:
		var payloadData protocol.JoinPayload
		if err := json.Unmarshal(msg.D, &payloadData); err != nil {
			log.Println(err)
			return
		}

		state := protocol.PlayerState{
			U:        sender.ID,
			Name:     payloadData.Name,
			X:        payloadData.X,
			Y:        payloadData.Y,
			Rotation: payloadData.Rotation,
			Speaking: false,
		}

		hub.setPlayerState(sender, state)
		hub.sendStateSync(sender)

	case protocol.EventMove:
		var payloadData protocol.MovePayload
		if err := json.Unmarshal(msg.D, &payloadData); err != nil {
			log.Println(err)
			return
		}

		hub.updatePlayerMove(sender, payloadData)

	case protocol.EventSpeakStart:
		hub.updatePlayerSpeaking(sender, true)

	case protocol.EventSpeakStop:
		hub.updatePlayerSpeaking(sender, false)

	case protocol.EventLeave:
		hub.deletePlayerState(sender)

	case protocol.EventPing, protocol.EventPong:
		// 状態更新なし
	}

	hub.broadcast(sender, data)
}

func (h *Hub) broadcastWorldLeave(sender *Client) {
	payloadData, err := json.Marshal(protocol.LeavePayload{})
	if err != nil {
		log.Println(err)
		return
	}

	data, err := json.Marshal(protocol.Message{
		T: protocol.EventLeave,
		U: sender.ID,
		D: payloadData,
	})
	if err != nil {
		log.Println(err)
		return
	}

	h.broadcast(sender, data)
}
