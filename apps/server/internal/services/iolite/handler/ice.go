package handler

import (
	"strings"

	"github.com/mofafe/petrichor/apps/server/internal/services/iolite/turn"
)

type iceResponse struct {
	ICEServers []iceServer `json:"iceServers"`
}

type iceServer struct {
	URLs       any    `json:"urls"`
	Username   string `json:"username,omitempty"`
	Credential string `json:"credential,omitempty"`
}

func buildICEResponse(clientID string, turnSecret string, turnRealm string) iceResponse {
	turnUserID := strings.ReplaceAll(clientID, ":", "_")
	if turnUserID == "" {
		turnUserID = "anonymous"
	}

	username, credential := turn.GenerateTurnCredential(turnUserID, turnSecret)

	return iceResponse{
		ICEServers: []iceServer{
			{
				URLs: "stun:" + turnRealm + ":3478",
			},
			{
				URLs: []string{
					"turn:" + turnRealm + ":3478?transport=udp",
					"turn:" + turnRealm + ":3478?transport=tcp",
					"turns:" + turnRealm + ":5349?transport=tcp",
				},
				Username:   username,
				Credential: credential,
			},
		},
	}
}
