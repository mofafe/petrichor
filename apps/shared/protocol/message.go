package protocol

import "encoding/json"

// 共通メッセージ

type Message struct {
	T string          `json:"t"`           // event type
	U string          `json:"u,omitempty"` // user id (server -> client)
	D json.RawMessage `json:"d"`           // payload
}
