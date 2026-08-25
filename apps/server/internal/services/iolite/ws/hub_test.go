package ws

import (
	"errors"
	"fmt"
	"testing"

	"github.com/gorilla/websocket"
)

func TestJoinRoomCapacity(t *testing.T) {
	hub := NewHub()
	roomID := "cap-test"

	// MaxRoomClients人まで参加できることを確認
	for i := range MaxRoomClients {
		c := newTestClient(fmt.Sprintf("user-%d", i), roomID)
		if !hub.Join(c) {
			t.Fatalf("Join returned false for user %d, want true", i)
		}
	}

	// MaxRoomClients+1人目は拒否される
	overflow := newTestClient("user-overflow", roomID)
	if hub.Join(overflow) {
		t.Fatal("Join returned true when room is full, want false")
	}

	// 満員でもRoomに追加されていないことを確認
	room := hub.rooms[roomID]
	if _, ok := room.Clients[overflow.ID]; ok {
		t.Fatal("overflow client was added to full room")
	}
}

func TestJoinAfterLeaveAllowsNewClient(t *testing.T) {
	hub := NewHub()
	roomID := "leave-test"

	clients := make([]*Client, MaxRoomClients)
	for i := range MaxRoomClients {
		c := newTestClient(fmt.Sprintf("user-%d", i), roomID)
		hub.Join(c)
		clients[i] = c
	}

	// 満員状態で1人退出
	hub.Leave(clients[0], disconnectInfo{Status: "normal"})

	// 再度参加できることを確認
	newcomer := newTestClient("user-newcomer", roomID)
	if !hub.Join(newcomer) {
		t.Fatal("Join returned false after a slot was freed, want true")
	}
}

func TestClassifyDisconnectNormalClosure(t *testing.T) {
	info := classifyDisconnect(&websocket.CloseError{
		Code: websocket.CloseNormalClosure,
		Text: "normal closure",
	})

	if info.Status != "normal" {
		t.Fatalf("Status = %q, want %q", info.Status, "normal")
	}
	if info.Code != websocket.CloseNormalClosure {
		t.Fatalf("Code = %d, want %d", info.Code, websocket.CloseNormalClosure)
	}
	if info.Reason != "normal closure" {
		t.Fatalf("Reason = %q, want %q", info.Reason, "normal closure")
	}
}

func TestClassifyDisconnectAbnormalClose(t *testing.T) {
	info := classifyDisconnect(&websocket.CloseError{
		Code: websocket.CloseProtocolError,
		Text: "protocol error",
	})

	if info.Status != "abnormal" {
		t.Fatalf("Status = %q, want %q", info.Status, "abnormal")
	}
	if info.Code != websocket.CloseProtocolError {
		t.Fatalf("Code = %d, want %d", info.Code, websocket.CloseProtocolError)
	}
	if info.Reason != "protocol error" {
		t.Fatalf("Reason = %q, want %q", info.Reason, "protocol error")
	}
}

func TestClassifyDisconnectNonCloseError(t *testing.T) {
	err := errors.New("read failed")
	info := classifyDisconnect(err)

	if info.Status != "abnormal" {
		t.Fatalf("Status = %q, want %q", info.Status, "abnormal")
	}
	if info.Code != websocket.CloseAbnormalClosure {
		t.Fatalf("Code = %d, want %d", info.Code, websocket.CloseAbnormalClosure)
	}
	if info.Reason != err.Error() {
		t.Fatalf("Reason = %q, want %q", info.Reason, err.Error())
	}
}
