package ws

import (
	"encoding/json"
	"errors"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

func buildServerMessage(sender *Client, msg protocol.Message) ([]byte, error) {
	msg.U = sender.ID
	return json.Marshal(msg)
}

func routeMessage(hub *Hub, sender *Client, data []byte) error {
	var msg protocol.Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return err
	}

	switch sender.channel() {
	case ChannelWorld:
		return routeWorldMessage(hub, sender, msg)
	case ChannelSignaling:
		return routeSignalingMessage(hub, sender, msg)
	case ChannelChat:
		return routeChatMessage(hub, sender, msg)
	}

	return errors.New("unknown websocket channel")
}

func routeWorldMessage(hub *Hub, sender *Client, msg protocol.Message) error {
	switch msg.T {
	case protocol.EventJoin,
		protocol.EventLeave,
		protocol.EventMove,
		protocol.EventSpeakStart,
		protocol.EventSpeakStop,
		protocol.EventPing,
		protocol.EventPong:
		handleWorldMessage(hub, sender, msg)
		return nil
	case protocol.EventStateSync:
		return errors.New("invalid world payload type")
	}

	return errors.New("invalid world event type")
}

func routeSignalingMessage(hub *Hub, sender *Client, msg protocol.Message) error {
	switch msg.T {
	case protocol.EventOffer,
		protocol.EventAnswer,
		protocol.EventCandidate:
		handleSignalingMessage(hub, sender, msg)
		return nil
	}

	return errors.New("invalid signaling event type")
}

func routeChatMessage(hub *Hub, sender *Client, msg protocol.Message) error {
	switch msg.T {
	case protocol.EventChat:
		handleChatMessage(hub, sender, msg)
		return nil
	}

	return errors.New("invalid chat event type")
}
