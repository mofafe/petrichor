package ws

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/mofafe/petrichor/apps/shared/protocol"
)

type Client struct {
	ID      string
	RoomID  string
	Channel Channel
	Conn    *websocket.Conn
	Send    chan []byte
}

type Channel string

const (
	ChannelWorld     Channel = "world"
	ChannelSignaling Channel = "signaling"
	ChannelChat      Channel = "chat"
)

type Hub struct {
	rooms map[string]*Room
	mu    sync.Mutex
}

type Room struct {
	ID               string
	Clients          map[string]*Client
	SignalingClients map[string]*Client
	ChatClients      map[string]*Client
	Players          map[string]protocol.PlayerState
}

// MaxRoomClients はRoomに同時参加できる最大人数。MVPでは6人。
const MaxRoomClients = 6

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 8192
)

type disconnectInfo struct {
	Status string
	Code   int
	Reason string
}

// NewHub 共通Hubの初期化をして接続の管理をできるように
func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]*Room),
	}
}

// HandleWS WebSocketアップグレードをしてroom,user登録
// goroutineでメッセージ読み取りとメッセージ送信を起動
func HandleWS(hub *Hub, roomID string, channel Channel, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf(
			"websocket upgrade failed channel=%s room=%s host=%s origin=%s upgrade=%q connection=%q error=%q",
			channel,
			roomID,
			r.Host,
			r.Header.Get("Origin"),
			r.Header.Get("Upgrade"),
			r.Header.Get("Connection"),
			err.Error(),
		)
		return
	}

	client := &Client{
		ID:      socketUserID(channel, r),
		RoomID:  roomID,
		Channel: channel,
		Conn:    conn,
		Send:    make(chan []byte, 256),
	}

	if !hub.Join(client) {
		// 満員のためroom_fullイベントを送信して接続を閉じる
		data, err := json.Marshal(protocol.Message{T: protocol.EventRoomFull})
		if err != nil {
			log.Printf("room_full marshal failed: %v", err)
			conn.Close()
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			log.Printf("room_full write failed: %v", err)
		}
		conn.Close()
		return
	}

	go client.writePump()
	go client.readPump(hub)
}

func socketUserID(channel Channel, r *http.Request) string {
	if channel == ChannelWorld {
		return uuid.New().String()
	}

	userID := strings.TrimSpace(r.URL.Query().Get("userID"))
	if userID != "" {
		return userID
	}

	return uuid.New().String()
}

// readPump メッセージを読み取ってrouteMessageに渡す
// 切断済みクライアントは内部で除去する。
func (c *Client) readPump(h *Hub) {
	disconnect := disconnectInfo{
		Status: "unknown",
		Reason: "read pump stopped",
	}

	defer func() {
		playerRemoved := h.Leave(c, disconnect)
		if playerRemoved {
			h.broadcastWorldLeave(c)
		}
		c.Conn.Close()
		close(c.Send)
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	if err := c.Conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		log.Printf("websocket read deadline setup failed user=%s room=%s error=%q", c.ID, c.RoomID, err.Error())
		return
	}
	c.Conn.SetPongHandler(func(string) error {
		return c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, msg, err := c.Conn.ReadMessage()
		if err != nil {
			disconnect = classifyDisconnect(err)
			return
		}
		err = routeMessage(h, c, msg)
		if err != nil {
			disconnect = disconnectInfo{
				Status: "abnormal",
				Reason: err.Error(),
			}
			return
		}
	}
}

// writePump c.Sendに値が来るたびに送信
// 接続が閉じるとclose(c.Send)されるため自動で消える
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				return
			}
			if err := c.writeMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("websocket write failed user=%s room=%s error=%q", c.ID, c.RoomID, err.Error())
				return
			}
		case <-ticker.C:
			if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("websocket ping failed user=%s room=%s error=%q", c.ID, c.RoomID, err.Error())
				return
			}
		}
	}
}

func (c *Client) writeMessage(messageType int, data []byte) error {
	if err := c.Conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return c.Conn.WriteMessage(messageType, data)
}

func classifyDisconnect(err error) disconnectInfo {
	info := disconnectInfo{
		Status: "abnormal",
		Code:   websocket.CloseAbnormalClosure,
		Reason: err.Error(),
	}

	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		info.Code = closeErr.Code
		info.Reason = closeErr.Text

		if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			info.Status = "normal"
		}
	}

	return info
}

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

// Join はクライアントをRoomに参加させ、Roomがなければ作成する。
// 満員の場合はfalseを返し、Roomには追加しない。
func (h *Hub) Join(c *Client) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[c.RoomID]
	if !ok {
		// okに有るか無いかが入るためfalseだと作成
		room = &Room{
			ID:               c.RoomID,
			Clients:          make(map[string]*Client),
			SignalingClients: make(map[string]*Client),
			ChatClients:      make(map[string]*Client),
			Players:          make(map[string]protocol.PlayerState),
		}
		h.rooms[c.RoomID] = room
	}

	if c.channel() == ChannelSignaling {
		room.SignalingClients[c.ID] = c
		log.Println("signaling connected:", c.ID, "room:", c.RoomID, "count:", len(room.SignalingClients))
		return true
	}

	if c.channel() == ChannelChat {
		room.ChatClients[c.ID] = c
		log.Println("chat connected:", c.ID, "room:", c.RoomID, "count:", len(room.ChatClients))
		return true
	}

	if len(room.Clients) >= MaxRoomClients {
		log.Printf("room full: room=%s count=%d max=%d", c.RoomID, len(room.Clients), MaxRoomClients)
		return false
	}

	room.Clients[c.ID] = c

	log.Println("connected:", c.ID, "room:", c.RoomID, "count:", len(room.Clients))
	return true
}

// Leave はクライアントをRoomから退出させ、空になったRoomを削除する。
// world player state を削除した時は true を返し、呼び出し側が残り client へ leave を通知する。
func (h *Hub) Leave(c *Client, info disconnectInfo) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	room, ok := h.rooms[c.RoomID]
	if !ok {
		return false
	}

	_, playerWasPresent := room.Players[c.ID]
	delete(room.Clients, c.ID)
	delete(room.SignalingClients, c.ID)
	delete(room.ChatClients, c.ID)
	if c.channel() == ChannelWorld {
		delete(room.Players, c.ID)
	}

	log.Printf(
		"websocket disconnected channel=%s user=%s room=%s world_count=%d signaling_count=%d chat_count=%d status=%s code=%d reason=%q",
		c.channel(),
		c.ID,
		c.RoomID,
		len(room.Clients),
		len(room.SignalingClients),
		len(room.ChatClients),
		info.Status,
		info.Code,
		info.Reason,
	)

	if len(room.Clients) == 0 && len(room.SignalingClients) == 0 && len(room.ChatClients) == 0 {
		delete(h.rooms, c.RoomID)
	}

	return c.channel() == ChannelWorld && playerWasPresent
}

func (c *Client) channel() Channel {
	if c.Channel == "" {
		return ChannelWorld
	}
	return c.Channel
}
