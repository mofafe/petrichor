package protocol

// rotation
type Rotation struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
	Z float32 `json:"z"`
}

// room player state
type PlayerState struct {
	U        string   `json:"u"`
	Name     string   `json:"name"`
	X        float32  `json:"x"`
	Y        float32  `json:"y"`
	Rotation Rotation `json:"rotation"`
	Speaking bool     `json:"speaking"`
}

// --------------------
// payloads
// --------------------

// join
type JoinPayload struct {
	Name     string   `json:"name"`
	X        float32  `json:"x"`
	Y        float32  `json:"y"`
	Rotation Rotation `json:"rotation"`
}

// move
type MovePayload struct {
	X        float32  `json:"x"`
	Y        float32  `json:"y"`
	Rotation Rotation `json:"rotation"`
}

// leave
type LeavePayload struct{}

// speak start / stop
type SpeakPayload struct{}

// ping / pong
type PingPayload struct{}

// state sync
type StateSyncPayload struct {
	Players []PlayerState `json:"players"`
}

// chat
type ChatPayload struct {
	Message string `json:"message"`
}

// signaling
type OfferPayload struct {
	Target string `json:"target"`
	SDP    string `json:"sdp"`
}

type AnswerPayload struct {
	Target string `json:"target"`
	SDP    string `json:"sdp"`
}

type CandidatePayload struct {
	Target        string `json:"target"`
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex uint16 `json:"sdpMLineIndex"`
}
