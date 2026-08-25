package ws

import (
	"encoding/json"
	"testing"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

func TestWorldMessageUpdatesPlayerState(t *testing.T) {
	hub := NewHub()
	client := newTestClient("user-1", "room-1")
	hub.Join(client)

	mustRouteMessage(t, hub, client, protocol.Message{
		T: protocol.EventJoin,
		D: mustTestPayload(t, protocol.JoinPayload{
			Name:     "alice",
			X:        1,
			Y:        2,
			Rotation: protocol.Rotation{X: 0, Y: 1, Z: 2},
		}),
	})

	state := hub.rooms[client.RoomID].Players[client.ID]
	if state.Name != "alice" || state.X != 1 || state.Y != 2 || state.Rotation != (protocol.Rotation{X: 0, Y: 1, Z: 2}) || state.Speaking {
		t.Fatalf("state after join = %+v", state)
	}

	mustRouteMessage(t, hub, client, protocol.Message{
		T: protocol.EventMove,
		D: mustTestPayload(t, protocol.MovePayload{
			X:        3,
			Y:        4,
			Rotation: protocol.Rotation{X: 3, Y: 4, Z: 5},
		}),
	})

	state = hub.rooms[client.RoomID].Players[client.ID]
	if state.Name != "alice" || state.X != 3 || state.Y != 4 || state.Rotation != (protocol.Rotation{X: 3, Y: 4, Z: 5}) || state.Speaking {
		t.Fatalf("state after move = %+v", state)
	}

	mustRouteMessage(t, hub, client, protocol.Message{T: protocol.EventSpeakStart, D: mustTestPayload(t, protocol.SpeakPayload{})})
	state = hub.rooms[client.RoomID].Players[client.ID]
	if !state.Speaking {
		t.Fatalf("Speaking after speak_start = %v, want true", state.Speaking)
	}

	mustRouteMessage(t, hub, client, protocol.Message{T: protocol.EventSpeakStop, D: mustTestPayload(t, protocol.SpeakPayload{})})
	state = hub.rooms[client.RoomID].Players[client.ID]
	if state.Speaking {
		t.Fatalf("Speaking after speak_stop = %v, want false", state.Speaking)
	}

	mustRouteMessage(t, hub, client, protocol.Message{T: protocol.EventLeave, D: mustTestPayload(t, protocol.LeavePayload{})})
	if _, ok := hub.rooms[client.RoomID].Players[client.ID]; ok {
		t.Fatal("player state remained after leave event")
	}
}

func TestJoinSendsStateSyncToJoiningClient(t *testing.T) {
	hub := NewHub()
	existing := newTestClient("user-1", "room-1")
	joining := newTestClient("user-2", "room-1")
	hub.Join(existing)
	hub.Join(joining)

	hub.setPlayerState(existing, protocol.PlayerState{
		U:        existing.ID,
		Name:     "alice",
		X:        1,
		Y:        2,
		Rotation: protocol.Rotation{X: 0, Y: 1, Z: 2},
	})

	mustRouteMessage(t, hub, joining, protocol.Message{
		T: protocol.EventJoin,
		D: mustTestPayload(t, protocol.JoinPayload{
			Name:     "bob",
			X:        5,
			Y:        6,
			Rotation: protocol.Rotation{X: 3, Y: 4, Z: 5},
		}),
	})

	var syncMsg protocol.Message
	readTestMessage(t, joining, &syncMsg)
	if syncMsg.T != protocol.EventStateSync {
		t.Fatalf("first message type = %q, want %q", syncMsg.T, protocol.EventStateSync)
	}

	var payload protocol.StateSyncPayload
	if err := json.Unmarshal(syncMsg.D, &payload); err != nil {
		t.Fatalf("unmarshal state_sync payload: %v", err)
	}

	if len(payload.Players) != 2 {
		t.Fatalf("state_sync players length = %d, want 2: %+v", len(payload.Players), payload.Players)
	}

	assertPlayerState(t, payload.Players, protocol.PlayerState{
		U:        existing.ID,
		Name:     "alice",
		X:        1,
		Y:        2,
		Rotation: protocol.Rotation{X: 0, Y: 1, Z: 2},
	})
	assertPlayerState(t, payload.Players, protocol.PlayerState{
		U:        joining.ID,
		Name:     "bob",
		X:        5,
		Y:        6,
		Rotation: protocol.Rotation{X: 3, Y: 4, Z: 5},
	})
}

func TestLeaveDeletesPlayerStateOnDisconnect(t *testing.T) {
	hub := NewHub()
	client := newTestClient("user-1", "room-1")
	remaining := newTestClient("user-2", "room-1")
	hub.Join(client)
	hub.Join(remaining)
	hub.setPlayerState(client, protocol.PlayerState{U: client.ID, Name: "alice"})
	hub.setPlayerState(remaining, protocol.PlayerState{U: remaining.ID, Name: "bob"})

	hub.Leave(client, disconnectInfo{Status: "normal"})

	room, ok := hub.rooms[client.RoomID]
	if !ok {
		t.Fatal("room was deleted while another client remained")
	}
	if _, ok := room.Players[client.ID]; ok {
		t.Fatal("player state remained after disconnect")
	}
	if _, ok := room.Players[remaining.ID]; !ok {
		t.Fatal("remaining player state was deleted")
	}
}

func TestAbruptWorldDisconnectBroadcastsLeave(t *testing.T) {
	hub := NewHub()
	client := newTestClient("user-1", "room-1")
	remaining := newTestClient("user-2", "room-1")
	hub.Join(client)
	hub.Join(remaining)
	hub.setPlayerState(client, protocol.PlayerState{U: client.ID, Name: "alice"})
	hub.setPlayerState(remaining, protocol.PlayerState{U: remaining.ID, Name: "bob"})

	if !hub.Leave(client, disconnectInfo{Status: "abnormal"}) {
		t.Fatal("Leave returned false for active world player")
	}
	hub.broadcastWorldLeave(client)

	var got protocol.Message
	readTestMessage(t, remaining, &got)
	if got.T != protocol.EventLeave || got.U != client.ID {
		t.Fatalf("disconnect message = %+v, want leave from %q", got, client.ID)
	}
	if _, ok := hub.rooms[client.RoomID].Players[client.ID]; ok {
		t.Fatal("player state remained after abrupt disconnect")
	}
}

func TestExplicitLeaveDoesNotBroadcastDuplicateOnDisconnect(t *testing.T) {
	hub := NewHub()
	client := newTestClient("user-1", "room-1")
	remaining := newTestClient("user-2", "room-1")
	hub.Join(client)
	hub.Join(remaining)
	hub.setPlayerState(client, protocol.PlayerState{U: client.ID, Name: "alice"})
	hub.setPlayerState(remaining, protocol.PlayerState{U: remaining.ID, Name: "bob"})

	mustRouteMessage(t, hub, client, protocol.Message{
		T: protocol.EventLeave,
		D: mustTestPayload(t, protocol.LeavePayload{}),
	})
	readTestMessage(t, remaining, &protocol.Message{})

	if hub.Leave(client, disconnectInfo{Status: "normal"}) {
		t.Fatal("Leave returned true after explicit leave already deleted player state")
	}
	assertNoTestMessage(t, remaining)
}

func TestWorldRouteRejectsSignalingMessage(t *testing.T) {
	hub := NewHub()
	client := newTestClient("user-1", "room-1")
	hub.Join(client)

	err := routeTestMessage(t, hub, client, protocol.Message{
		T: protocol.EventOffer,
		D: mustTestPayload(t, protocol.OfferPayload{
			Target: "user-2",
			SDP:    "offer-sdp",
		}),
	})
	if err == nil {
		t.Fatal("routeMessage returned nil for signaling event on world channel")
	}
}

func TestSignalingRouteSendsOnlyToSignalingClient(t *testing.T) {
	hub := NewHub()
	worldSender := newTestClient("user-1", "room-1")
	worldTarget := newTestClient("user-2", "room-1")
	signalingSender := newTestClient("user-1", "room-1")
	signalingSender.Channel = ChannelSignaling
	signalingTarget := newTestClient("user-2", "room-1")
	signalingTarget.Channel = ChannelSignaling

	hub.Join(worldSender)
	hub.Join(worldTarget)
	hub.Join(signalingSender)
	hub.Join(signalingTarget)

	mustRouteMessage(t, hub, signalingSender, protocol.Message{
		T: protocol.EventAnswer,
		D: mustTestPayload(t, protocol.AnswerPayload{
			Target: "user-2",
			SDP:    "answer-sdp",
		}),
	})

	var got protocol.Message
	readTestMessage(t, signalingTarget, &got)
	if got.T != protocol.EventAnswer || got.U != signalingSender.ID {
		t.Fatalf("signaling message = %+v, want answer from %q", got, signalingSender.ID)
	}

	assertNoTestMessage(t, worldTarget)
}

func newTestClient(id string, roomID string) *Client {
	return &Client{
		ID:     id,
		RoomID: roomID,
		Send:   make(chan []byte, 10),
	}
}

func mustRouteMessage(t *testing.T, hub *Hub, client *Client, msg protocol.Message) {
	t.Helper()

	if err := routeTestMessage(t, hub, client, msg); err != nil {
		t.Fatalf("routeMessage: %v", err)
	}
}

func routeTestMessage(t *testing.T, hub *Hub, client *Client, msg protocol.Message) error {
	t.Helper()

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}
	return routeMessage(hub, client, data)
}

func mustTestPayload(t *testing.T, payload any) json.RawMessage {
	t.Helper()

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

func readTestMessage(t *testing.T, client *Client, out *protocol.Message) {
	t.Helper()

	select {
	case data := <-client.Send:
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatalf("unmarshal sent message: %v", err)
		}
	default:
		t.Fatal("expected message, got none")
	}
}

func assertNoTestMessage(t *testing.T, client *Client) {
	t.Helper()

	select {
	case data := <-client.Send:
		t.Fatalf("unexpected message: %s", string(data))
	default:
	}
}

func assertPlayerState(t *testing.T, players []protocol.PlayerState, want protocol.PlayerState) {
	t.Helper()

	for _, got := range players {
		if got.U != want.U {
			continue
		}
		if got.Name != want.Name || got.X != want.X || got.Y != want.Y || got.Rotation != want.Rotation || got.Speaking != want.Speaking {
			t.Fatalf("player %q = %+v, want %+v", want.U, got, want)
		}
		return
	}

	t.Fatalf("player %q not found in %+v", want.U, players)
}
