package protocol

const (
	// world
	EventJoin       = "join"
	EventLeave      = "leave"
	EventMove       = "move"
	EventStateSync  = "state_sync"
	EventSpeakStart = "speak_start"
	EventSpeakStop  = "speak_stop"
	EventPing       = "ping"
	EventPong       = "pong"
	EventRoomFull   = "room_full"

	// chat
	EventChat = "chat"

	// signaling
	EventOffer     = "offer"
	EventAnswer    = "answer"
	EventCandidate = "candidate"
)
