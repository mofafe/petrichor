package voice

import (
	"context"
	"encoding/json"
	"log"
	"math"
	"sync"
	"time"

	"github.com/mofafe/petrichor/apps/ebiten-lab/internal/net"
	"github.com/mofafe/petrichor/apps/shared/protocol"
	pionopus "github.com/pion/opus"
	"github.com/pion/webrtc/v4"
)

type Sender interface {
	Send(eventType string, payload any) error
}

type Manager struct {
	world    Sender
	signaler Sender
	localID  string
	enabled  bool
	track    *webrtc.TrackLocalStaticSample
	mic      *microphone
	playback *playbackMixer
	localX   float32
	localY   float32

	iceServers []webrtc.ICEServer

	mu                 sync.Mutex
	peers              map[string]*peerEntry
	remotePositions    map[string]protocol.PlayerState
	pendingCandidates  map[string][]webrtc.ICECandidateInit
	signalingMessages  chan protocol.Message
	negotiationPending map[string]bool
	negotiationRetries map[string]int
}

type peerEntry struct {
	pc          *webrtc.PeerConnection
	polite      bool
	makingOffer bool
	ignoreOffer bool
	sender      *webrtc.RTPSender
	retryTimer  *time.Timer
}

const (
	fullVolumeDistance     = 2
	silentDistance         = 14
	maxNegotiationRetries  = 3
	retryBaseDelay         = 800 * time.Millisecond
	disconnectedRetryDelay = 3 * time.Second
	defaultSTUNServer      = "stun:stun.l.google.com:19302"
	remoteDecodeBufferSize = 5760
)

func New(world Sender) *Manager {
	return &Manager{
		world:              world,
		peers:              map[string]*peerEntry{},
		remotePositions:    map[string]protocol.PlayerState{},
		pendingCandidates:  map[string][]webrtc.ICECandidateInit{},
		signalingMessages:  make(chan protocol.Message, 64),
		negotiationPending: map[string]bool{},
		negotiationRetries: map[string]int{},
	}
}

func (m *Manager) Incoming() chan<- protocol.Message {
	return m.signalingMessages
}

func (m *Manager) SetLocalUserID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localID = id
}

func (m *Manager) SetSignalingSocket(socket *net.Socket) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.signaler = socket
}

func (m *Manager) SetICEServers(servers []webrtc.ICEServer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iceServers = append([]webrtc.ICEServer(nil), servers...)
}

func (m *Manager) Enable() error {
	m.mu.Lock()
	if m.enabled {
		m.mu.Unlock()
		return nil
	}
	track := newAudioTrack()
	mic, err := startMicrophone(track)
	if err != nil {
		m.mu.Unlock()
		return err
	}
	m.enabled = true
	m.track = track
	m.mic = mic
	ids := make([]string, 0, len(m.remotePositions))
	for id := range m.remotePositions {
		ids = append(ids, id)
	}
	existingPeers := make(map[string]*peerEntry, len(m.peers))
	for id, entry := range m.peers {
		existingPeers[id] = entry
	}
	m.mu.Unlock()

	_ = m.world.Send("speak_start", struct{}{})
	for id, entry := range existingPeers {
		m.addLocalTrack(id, entry, track)
	}
	for _, id := range ids {
		go m.requestNegotiationWithForce(id, true)
	}
	return nil
}

func (m *Manager) Disable() {
	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return
	}
	m.enabled = false
	mic := m.mic
	m.mic = nil
	m.track = nil
	peers := m.peers
	m.peers = map[string]*peerEntry{}
	m.mu.Unlock()

	if mic != nil {
		mic.Close()
	}
	_ = m.world.Send("speak_stop", struct{}{})
	for _, entry := range peers {
		if entry.retryTimer != nil {
			entry.retryTimer.Stop()
		}
		_ = entry.pc.Close()
	}
	m.closePlayback()
}

func (m *Manager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			m.Disable()
			return
		case msg := <-m.signalingMessages:
			if err := m.handleMessage(msg); err != nil {
				log.Printf("voice signaling %s from %s failed: %v", msg.T, msg.U, err)
			}
		}
	}
}

func (m *Manager) UpsertRemotePlayer(player protocol.PlayerState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if player.U == m.localID {
		return
	}
	m.remotePositions[player.U] = player
	m.updateVolumeLocked(player.U)
	if m.enabled {
		m.negotiationPending[player.U] = true
	}
}

func (m *Manager) SyncRemotePlayers(players []protocol.PlayerState) {
	seen := map[string]bool{}
	for _, p := range players {
		if p.U == m.localID {
			continue
		}
		seen[p.U] = true
		m.UpsertRemotePlayer(p)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for id := range m.remotePositions {
		if !seen[id] {
			delete(m.remotePositions, id)
			m.removePlaybackSourceLocked(id)
			delete(m.negotiationRetries, id)
			if entry := m.peers[id]; entry != nil {
				_ = entry.pc.Close()
			}
			delete(m.peers, id)
		}
	}
}

func (m *Manager) RemoveRemotePlayer(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.remotePositions, id)
	m.removePlaybackSourceLocked(id)
	delete(m.negotiationRetries, id)
	if entry := m.peers[id]; entry != nil {
		if entry.retryTimer != nil {
			entry.retryTimer.Stop()
		}
		_ = entry.pc.Close()
	}
	delete(m.peers, id)
}

func (m *Manager) UpdateLocalPosition(x, y float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localX = x
	m.localY = y
	for id := range m.peers {
		m.updateVolumeLocked(id)
	}
}

func (m *Manager) Tick() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.negotiationPending))
	for id := range m.negotiationPending {
		ids = append(ids, id)
		delete(m.negotiationPending, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		go m.requestNegotiation(id)
	}
}

func (m *Manager) handleMessage(message protocol.Message) error {
	switch message.T {
	case "offer":
		return m.handleOffer(message)
	case "answer":
		return m.handleAnswer(message)
	case "candidate":
		return m.handleCandidate(message)
	default:
		return nil
	}
}

func (m *Manager) requestNegotiation(remoteID string) {
	m.requestNegotiationWithForce(remoteID, false)
}

func (m *Manager) requestNegotiationWithForce(remoteID string, force bool) {
	m.mu.Lock()
	if !m.enabled || m.localID == "" || remoteID == m.localID {
		m.mu.Unlock()
		return
	}
	if !force && !(m.localID < remoteID) {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	entry, err := m.ensurePeer(remoteID)
	if err != nil {
		log.Printf("create peer %s failed: %v", remoteID, err)
		return
	}

	m.mu.Lock()
	if entry.makingOffer || entry.pc.SignalingState() != webrtc.SignalingStateStable {
		m.negotiationPending[remoteID] = true
		m.mu.Unlock()
		return
	}
	entry.makingOffer = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		entry.makingOffer = false
		m.mu.Unlock()
	}()

	offer, err := entry.pc.CreateOffer(nil)
	if err != nil {
		m.scheduleNegotiationRetry(remoteID, "offer failed")
		return
	}
	if err := entry.pc.SetLocalDescription(offer); err != nil {
		m.scheduleNegotiationRetry(remoteID, "offer failed")
		return
	}
	m.send("offer", protocol.OfferPayload{Target: remoteID, SDP: offer.SDP})
}

func (m *Manager) addLocalTrack(remoteID string, entry *peerEntry, track *webrtc.TrackLocalStaticSample) {
	m.mu.Lock()
	if current := m.peers[remoteID]; current != entry || entry.sender != nil {
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()

	sender, err := entry.pc.AddTrack(track)
	if err != nil {
		log.Printf("add local track %s failed: %v", remoteID, err)
		return
	}
	go readRTCP(sender)

	m.mu.Lock()
	if current := m.peers[remoteID]; current == entry {
		entry.sender = sender
	}
	m.mu.Unlock()
}

func (m *Manager) handleOffer(message protocol.Message) error {
	if message.U == "" {
		return nil
	}
	var payload protocol.OfferPayload
	if err := json.Unmarshal(message.D, &payload); err != nil {
		return err
	}

	entry, err := m.ensurePeer(message.U)
	if err != nil {
		return err
	}

	m.mu.Lock()
	collision := entry.makingOffer || entry.pc.SignalingState() != webrtc.SignalingStateStable
	entry.ignoreOffer = !entry.polite && collision
	if entry.ignoreOffer {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	if collision {
		if err := entry.pc.SetLocalDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeRollback}); err != nil {
			return err
		}
	}
	if err := entry.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  payload.SDP,
	}); err != nil {
		return err
	}
	answer, err := entry.pc.CreateAnswer(nil)
	if err != nil {
		return err
	}
	if err := entry.pc.SetLocalDescription(answer); err != nil {
		return err
	}
	m.send("answer", protocol.AnswerPayload{Target: message.U, SDP: answer.SDP})
	m.resetNegotiationRetry(message.U)
	return m.flushPendingCandidates(message.U)
}

func (m *Manager) handleAnswer(message protocol.Message) error {
	if message.U == "" {
		return nil
	}
	var payload protocol.AnswerPayload
	if err := json.Unmarshal(message.D, &payload); err != nil {
		return err
	}

	m.mu.Lock()
	entry := m.peers[message.U]
	m.mu.Unlock()
	if entry == nil || entry.pc.SignalingState() != webrtc.SignalingStateHaveLocalOffer {
		return nil
	}
	if err := entry.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  payload.SDP,
	}); err != nil {
		m.scheduleNegotiationRetry(message.U, "answer failed")
		return err
	}
	m.resetNegotiationRetry(message.U)
	return m.flushPendingCandidates(message.U)
}

func (m *Manager) handleCandidate(message protocol.Message) error {
	if message.U == "" {
		return nil
	}
	var payload protocol.CandidatePayload
	if err := json.Unmarshal(message.D, &payload); err != nil {
		return err
	}
	candidate := webrtc.ICECandidateInit{
		Candidate:     payload.Candidate,
		SDPMid:        &payload.SDPMid,
		SDPMLineIndex: &payload.SDPMLineIndex,
	}

	m.mu.Lock()
	entry := m.peers[message.U]
	if entry == nil || entry.pc.RemoteDescription() == nil {
		m.pendingCandidates[message.U] = append(m.pendingCandidates[message.U], candidate)
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	return entry.pc.AddICECandidate(candidate)
}

func (m *Manager) ensurePeer(remoteID string) (*peerEntry, error) {
	m.mu.Lock()
	if entry := m.peers[remoteID]; entry != nil {
		m.mu.Unlock()
		return entry, nil
	}
	polite := m.localID > remoteID
	m.mu.Unlock()

	iceServers := m.peerICEServers()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{ICEServers: iceServers})
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	localTrack := m.track
	m.mu.Unlock()
	var sender *webrtc.RTPSender
	if localTrack != nil {
		sender, err = pc.AddTrack(localTrack)
		if err != nil {
			_ = pc.Close()
			return nil, err
		}
		go readRTCP(sender)
	} else {
		if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
			_ = pc.Close()
			return nil, err
		}
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		init := c.ToJSON()
		m.send("candidate", protocol.CandidatePayload{
			Target:        remoteID,
			Candidate:     init.Candidate,
			SDPMid:        derefString(init.SDPMid),
			SDPMLineIndex: derefUint16(init.SDPMLineIndex),
		})
	})
	pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if track.Kind() == webrtc.RTPCodecTypeAudio {
			go m.readRemoteAudio(remoteID, track)
		}
	})
	pc.OnSignalingStateChange(func(state webrtc.SignalingState) {
		if state == webrtc.SignalingStateStable {
			m.mu.Lock()
			queued := m.negotiationPending[remoteID]
			if queued {
				delete(m.negotiationPending, remoteID)
			}
			m.mu.Unlock()
			if queued {
				go m.requestNegotiation(remoteID)
			}
		}
	})
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("voice peer %s state=%s", remoteID, state.String())
		switch state {
		case webrtc.PeerConnectionStateConnected:
			m.resetNegotiationRetry(remoteID)
		case webrtc.PeerConnectionStateFailed:
			m.scheduleNegotiationRetry(remoteID, "connection failed")
		case webrtc.PeerConnectionStateDisconnected:
			m.scheduleNegotiationRetry(remoteID, "connection disconnected")
		case webrtc.PeerConnectionStateClosed:
			m.RemoveRemotePlayer(remoteID)
		}
	})

	entry := &peerEntry{pc: pc, polite: polite, sender: sender}
	m.mu.Lock()
	m.peers[remoteID] = entry
	m.updateVolumeLocked(remoteID)
	m.mu.Unlock()
	return entry, nil
}

func (m *Manager) peerICEServers() []webrtc.ICEServer {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.iceServers) == 0 {
		return []webrtc.ICEServer{{URLs: []string{defaultSTUNServer}}}
	}
	return append([]webrtc.ICEServer(nil), m.iceServers...)
}

func (m *Manager) readRemoteAudio(remoteID string, track *webrtc.TrackRemote) {
	playback, err := m.ensurePlayback()
	if err != nil {
		log.Printf("playback init failed: %v", err)
		return
	}
	playback.AddSource(remoteID)
	defer playback.RemoveSource(remoteID)

	decoder, err := pionopus.NewDecoderWithOutput(audioSampleRate, audioChannels)
	if err != nil {
		log.Printf("opus decoder init failed: %v", err)
		return
	}
	pcm := make([]int16, remoteDecodeBufferSize)
	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			return
		}
		sampleCount, err := decoder.DecodeToInt16(packet.Payload, pcm)
		if err != nil {
			log.Printf("opus decode failed: %v", err)
			continue
		}
		playback.Push(remoteID, append([]int16(nil), pcm[:sampleCount*audioChannels]...))
	}
}

func (m *Manager) ensurePlayback() (*playbackMixer, error) {
	m.mu.Lock()
	current := m.playback
	m.mu.Unlock()
	if current != nil {
		return current, nil
	}

	playback, err := newPlaybackMixer()
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.playback != nil {
		playback.Close()
		return m.playback, nil
	}
	m.playback = playback
	for id := range m.peers {
		m.updateVolumeLocked(id)
	}
	return playback, nil
}

func (m *Manager) closePlayback() {
	m.mu.Lock()
	playback := m.playback
	m.playback = nil
	m.mu.Unlock()
	if playback != nil {
		playback.Close()
	}
}

func (m *Manager) removePlaybackSourceLocked(id string) {
	if m.playback != nil {
		m.playback.RemoveSource(id)
	}
}

func (m *Manager) updateVolumeLocked(remoteID string) {
	remote, ok := m.remotePositions[remoteID]
	if !ok || m.playback == nil {
		return
	}
	distance := math.Hypot(float64(remote.X-m.localX), float64(remote.Y-m.localY))
	t := (distance - fullVolumeDistance) / (silentDistance - fullVolumeDistance)
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	volume := float32((1 - t) * (1 - t))
	m.playback.SetVolume(remoteID, volume)
}

func (m *Manager) scheduleNegotiationRetry(remoteID, reason string) {
	m.mu.Lock()
	entry := m.peers[remoteID]
	_, hasRemote := m.remotePositions[remoteID]
	if entry == nil || !m.enabled || !hasRemote || entry.retryTimer != nil {
		m.mu.Unlock()
		return
	}
	count := m.negotiationRetries[remoteID]
	if count >= maxNegotiationRetries {
		delete(m.negotiationRetries, remoteID)
		delete(m.peers, remoteID)
		m.removePlaybackSourceLocked(remoteID)
		m.mu.Unlock()
		_ = entry.pc.Close()
		log.Printf("voice negotiation retry exhausted %s: %s", remoteID, reason)
		return
	}
	count++
	m.negotiationRetries[remoteID] = count
	baseDelay := retryBaseDelay
	if reason == "connection disconnected" {
		baseDelay = disconnectedRetryDelay
	}
	delay := time.Duration(count) * baseDelay
	entry.retryTimer = time.AfterFunc(delay, func() {
		m.mu.Lock()
		current := m.peers[remoteID]
		if current == entry {
			delete(m.peers, remoteID)
			m.removePlaybackSourceLocked(remoteID)
		}
		localID := m.localID
		m.mu.Unlock()
		_ = entry.pc.Close()
		if localID != "" && localID < remoteID {
			m.requestNegotiation(remoteID)
		}
	})
	m.mu.Unlock()
}

func (m *Manager) resetNegotiationRetry(remoteID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.negotiationRetries, remoteID)
	if entry := m.peers[remoteID]; entry != nil && entry.retryTimer != nil {
		entry.retryTimer.Stop()
		entry.retryTimer = nil
	}
}

func newAudioTrack() *webrtc.TrackLocalStaticSample {
	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: audioSampleRate,
			Channels:  audioChannels,
		},
		"audio",
		"iolite-ebiten-lab",
	)
	if err != nil {
		panic(err)
	}
	return track
}

func readRTCP(sender *webrtc.RTPSender) {
	buf := make([]byte, 1500)
	for {
		if _, _, err := sender.Read(buf); err != nil {
			return
		}
	}
}

func (m *Manager) flushPendingCandidates(remoteID string) error {
	m.mu.Lock()
	entry := m.peers[remoteID]
	candidates := append([]webrtc.ICECandidateInit(nil), m.pendingCandidates[remoteID]...)
	delete(m.pendingCandidates, remoteID)
	m.mu.Unlock()

	if entry == nil {
		return nil
	}
	for _, candidate := range candidates {
		if err := entry.pc.AddICECandidate(candidate); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) send(eventType string, payload any) {
	m.mu.Lock()
	signaler := m.signaler
	m.mu.Unlock()
	if signaler == nil {
		return
	}
	if err := signaler.Send(eventType, payload); err != nil {
		log.Printf("send signaling %s failed: %v", eventType, err)
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefUint16(v *uint16) uint16 {
	if v == nil {
		return 0
	}
	return *v
}
