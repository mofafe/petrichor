package ws

import (
	"encoding/json"
	"log"

	"github.com/mofafe/petrichor/apps/shared/protocol"
)

// handleSignalingMessage
// offerは送信者以外の全員に送信
// answerとcandidateは送信先を指定して送信
func handleSignalingMessage(hub *Hub, sender *Client, msg protocol.Message) {
	data, err := buildServerMessage(sender, msg)
	if err != nil {
		log.Println(err)
		return
	}
	switch msg.T {
	case protocol.EventOffer:
		var payloadData protocol.OfferPayload
		if err := json.Unmarshal(msg.D, &payloadData); err != nil {
			log.Println(err)
			return
		}
		if payloadData.Target != "" {
			if err := hub.sendTo(sender, payloadData.Target, data); err != nil {
				log.Println(err)
			}
			return
		}
		hub.broadcastExcept(sender, data)
		return
	case protocol.EventAnswer:
		var payloadData protocol.AnswerPayload
		if err := json.Unmarshal(msg.D, &payloadData); err != nil {
			log.Println(err)
			return
		}
		if err := hub.sendTo(sender, payloadData.Target, data); err != nil {
			log.Println(err)
		}
		return
	case protocol.EventCandidate:
		var payloadData protocol.CandidatePayload
		if err := json.Unmarshal(msg.D, &payloadData); err != nil {
			log.Println(err)
			return
		}
		if err := hub.sendTo(sender, payloadData.Target, data); err != nil {
			log.Println(err)
		}
		return
	}
}
