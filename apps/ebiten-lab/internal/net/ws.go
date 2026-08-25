package net

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mofafe/petrichor/apps/shared/protocol"
	"github.com/pion/webrtc/v4"
)

type Socket struct {
	conn *websocket.Conn
	mu   sync.Mutex
	done chan struct{}
}

func WorldURL(server, roomID string) (string, error) {
	return wsURL(server, "/ws/rooms/world/"+url.PathEscape(roomID), nil)
}

func SignalingURL(server, roomID, userID string) (string, error) {
	q := url.Values{}
	q.Set("userID", userID)
	return wsURL(server, "/ws/rooms/signaling/"+url.PathEscape(roomID), q)
}

func InviteURL(server, roomID string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http", "https":
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return "", fmt.Errorf("unsupported server scheme %q", u.Scheme)
	}
	u.Path = "/iolite/r"
	u.RawQuery = url.Values{"room": []string{roomID}}.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func LoadICEServers(ctx context.Context, server string) ([]webrtc.ICEServer, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	default:
		return nil, fmt.Errorf("unsupported server scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/ice"
	u.RawQuery = ""

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ice server request failed: %s", resp.Status)
	}
	var payload struct {
		ICEServers []webrtc.ICEServer `json:"iceServers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.ICEServers, nil
}

func Dial(ctx context.Context, rawurl string, incoming chan<- protocol.Message) (*Socket, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, rawurl, nil)
	if err != nil {
		return nil, err
	}

	s := &Socket{conn: conn, done: make(chan struct{})}
	go s.readLoop(incoming)
	return s, nil
}

func (s *Socket) Send(eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	message, err := json.Marshal(protocol.Message{T: eventType, D: data})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn.WriteMessage(websocket.TextMessage, message)
}

func (s *Socket) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = s.conn.Close()
}

func (s *Socket) Done() <-chan struct{} {
	return s.done
}

func (s *Socket) readLoop(incoming chan<- protocol.Message) {
	defer close(s.done)
	for {
		_, data, err := s.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg protocol.Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		incoming <- msg
	}
}

func wsURL(server, path string, q url.Values) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported server scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	u.RawQuery = q.Encode()
	return u.String(), nil
}
