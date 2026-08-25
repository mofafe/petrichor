package app

import (
	"context"
	"encoding/json"
	"image/color"
	"math"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/mofafe/petrichor/apps/ebiten-lab/internal/net"
	"github.com/mofafe/petrichor/apps/ebiten-lab/internal/scene"
	"github.com/mofafe/petrichor/apps/ebiten-lab/internal/voice"
	"github.com/mofafe/petrichor/apps/shared/protocol"
)

const (
	ScreenWidth   = 960
	ScreenHeight  = 540
	DefaultServer = "https://petrichor.example.com"

	moveSendInterval = 80 * time.Millisecond
)

type Config struct {
	Name   string
	RoomID string
	Server string
}

type mode int

const (
	modeEntry mode = iota
	modeRoom
)

type Game struct {
	cfg Config

	mode       mode
	active     int
	entryName  string
	entryRoom  string
	status     string
	voiceState string

	ctx    context.Context
	cancel context.CancelFunc

	worldSocket     *net.Socket
	signalingSocket *net.Socket
	worldIncoming   chan protocol.Message
	voice           *voice.Manager
	world           *scene.World

	localUserID string
	remotes     map[string]protocol.PlayerState

	x, y       float32
	pitch, yaw float32
	moved      bool
	lastMove   time.Time

	mouseCaptured bool
	lastMouseX    int
	lastMouseY    int
}

func New(cfg Config) *Game {
	ctx, cancel := context.WithCancel(context.Background())
	if strings.TrimSpace(cfg.Name) == "" {
		cfg.Name = "guest"
	}
	if strings.TrimSpace(cfg.RoomID) == "" {
		cfg.RoomID = "debug"
	}
	if strings.TrimSpace(cfg.Server) == "" {
		cfg.Server = DefaultServer
	}
	return &Game{
		cfg:           cfg,
		mode:          modeEntry,
		entryName:     cfg.Name,
		entryRoom:     cfg.RoomID,
		status:        "enter name and room",
		voiceState:    "mic off",
		ctx:           ctx,
		cancel:        cancel,
		worldIncoming: make(chan protocol.Message, 64),
		remotes:       map[string]protocol.PlayerState{},
		yaw:           math.Pi,
		moved:         true,
	}
}

func (g *Game) Update() error {
	g.updateFullscreenToggle()

	switch g.mode {
	case modeEntry:
		g.updateEntry()
	case modeRoom:
		g.updateRoom()
	}
	return nil
}

func (g *Game) updateFullscreenToggle() {
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) ||
		(inpututil.IsKeyJustPressed(ebiten.KeyEnter) && (ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight))) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
}

func (g *Game) Draw(screen *ebiten.Image) {
	switch g.mode {
	case modeEntry:
		g.drawEntry(screen)
	case modeRoom:
		g.drawRoom(screen)
	}
}

func (g *Game) Layout(width, height int) (int, int) {
	if g.world != nil {
		g.world.Resize(width, height)
	}
	return width, height
}

func (g *Game) updateEntry() {
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.active = 1 - g.active
	}
	if inputChars := ebiten.AppendInputChars(nil); len(inputChars) > 0 {
		target := &g.entryName
		limit := 24
		if g.active == 1 {
			target = &g.entryRoom
			limit = 64
		}
		*target = trimMax(*target+string(inputChars), limit)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		target := &g.entryName
		if g.active == 1 {
			target = &g.entryRoom
		}
		if len(*target) > 0 {
			*target = (*target)[:len(*target)-1]
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) && !altPressed() {
		g.cfg.Name = normalizeDefault(g.entryName, "guest")
		g.cfg.RoomID = normalizeRoomID(g.entryRoom)
		g.joinRoom()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.active = 1 - g.active
	}
}

func (g *Game) updateRoom() {
	g.consumeWorldMessages()
	g.consumeInput()
	g.voice.Tick()
	g.world.SetCamera(g.x, g.y, g.pitch, g.yaw)
	g.sendMoveIfNeeded()
}

func (g *Game) joinRoom() {
	g.ctx, g.cancel = context.WithCancel(context.Background())
	g.worldIncoming = make(chan protocol.Message, 64)
	g.remotes = map[string]protocol.PlayerState{}
	g.localUserID = ""

	worldURL, err := net.WorldURL(g.cfg.Server, g.cfg.RoomID)
	if err != nil {
		g.status = err.Error()
		return
	}
	socket, err := net.Dial(g.ctx, worldURL, g.worldIncoming)
	if err != nil {
		g.status = "world websocket error: " + err.Error()
		return
	}
	g.worldSocket = socket
	g.voice = voice.New(socket)
	if iceServers, err := net.LoadICEServers(g.ctx, g.cfg.Server); err == nil {
		g.voice.SetICEServers(iceServers)
		g.status = "world connected"
	} else {
		g.status = "world connected; ice fallback: " + err.Error()
	}
	go g.voice.Run(g.ctx)
	g.world = scene.New(ScreenWidth, ScreenHeight)
	g.mode = modeRoom
	g.voiceState = "mic off"
	_ = g.worldSocket.Send(protocol.EventJoin, protocol.JoinPayload{
		Name:     g.cfg.Name,
		X:        g.x,
		Y:        g.y,
		Rotation: g.rotation(),
	})
}

func (g *Game) leaveRoom() {
	if g.worldSocket != nil {
		_ = g.worldSocket.Send(protocol.EventLeave, struct{}{})
		g.worldSocket.Close()
	}
	if g.signalingSocket != nil {
		g.signalingSocket.Close()
	}
	g.cancel()
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	g.mouseCaptured = false
	g.ctx, g.cancel = context.WithCancel(context.Background())
	g.mode = modeEntry
	g.status = "left room"
	g.voiceState = "mic off"
}

func (g *Game) consumeWorldMessages() {
	for {
		select {
		case msg := <-g.worldIncoming:
			g.handleWorldMessage(msg)
		default:
			return
		}
	}
}

func (g *Game) handleWorldMessage(msg protocol.Message) {
	switch msg.T {
	case protocol.EventJoin:
		g.handleJoin(msg)
	case protocol.EventMove:
		g.handleMove(msg)
	case protocol.EventStateSync:
		var payload protocol.StateSyncPayload
		if json.Unmarshal(msg.D, &payload) == nil {
			g.syncRemotePlayers(payload.Players)
		}
	case protocol.EventSpeakStart, protocol.EventSpeakStop:
		g.handleSpeak(msg)
	case protocol.EventLeave:
		g.removeRemote(msg.U)
	case protocol.EventRoomFull:
		g.status = "room full"
		if g.worldSocket != nil {
			g.worldSocket.Close()
		}
	}
}

func (g *Game) handleJoin(msg protocol.Message) {
	if msg.U == "" {
		return
	}
	if g.localUserID == "" {
		g.localUserID = msg.U
		g.voice.SetLocalUserID(msg.U)
		g.openSignalingSocket(msg.U)
		g.world.RemoveRemotePlayer(msg.U)
		g.status = "joined room " + g.cfg.RoomID
		return
	}
	var payload protocol.JoinPayload
	if json.Unmarshal(msg.D, &payload) != nil {
		return
	}
	g.upsertRemote(protocol.PlayerState{
		U:        msg.U,
		Name:     payload.Name,
		X:        payload.X,
		Y:        payload.Y,
		Rotation: payload.Rotation,
		Speaking: false,
	})
}

func (g *Game) handleMove(msg protocol.Message) {
	if msg.U == "" {
		return
	}
	var payload protocol.MovePayload
	if json.Unmarshal(msg.D, &payload) != nil {
		return
	}
	current := g.remotes[msg.U]
	current.U = msg.U
	current.X = payload.X
	current.Y = payload.Y
	current.Rotation = payload.Rotation
	g.upsertRemote(current)
}

func (g *Game) handleSpeak(msg protocol.Message) {
	if msg.U == "" {
		return
	}
	current := g.remotes[msg.U]
	current.U = msg.U
	current.Speaking = msg.T == protocol.EventSpeakStart
	g.upsertRemote(current)
}

func (g *Game) upsertRemote(player protocol.PlayerState) {
	if player.U == "" || player.U == g.localUserID {
		return
	}
	g.remotes[player.U] = player
	g.world.UpsertRemotePlayer(player, g.localUserID)
	g.voice.UpsertRemotePlayer(player)
}

func (g *Game) syncRemotePlayers(players []protocol.PlayerState) {
	seen := map[string]bool{}
	for _, player := range players {
		if player.U == g.localUserID {
			continue
		}
		seen[player.U] = true
		g.remotes[player.U] = player
	}
	for id := range g.remotes {
		if !seen[id] {
			delete(g.remotes, id)
		}
	}
	g.world.SyncRemotePlayers(players, g.localUserID)
	g.voice.SyncRemotePlayers(players)
}

func (g *Game) removeRemote(id string) {
	delete(g.remotes, id)
	g.world.RemoveRemotePlayer(id)
	g.voice.RemoveRemotePlayer(id)
}

func (g *Game) openSignalingSocket(userID string) {
	rawurl, err := net.SignalingURL(g.cfg.Server, g.cfg.RoomID, userID)
	if err != nil {
		g.status = err.Error()
		return
	}
	socket, err := net.Dial(g.ctx, rawurl, g.voice.Incoming())
	if err != nil {
		g.status = "signaling websocket error: " + err.Error()
		return
	}
	g.signalingSocket = socket
	g.voice.SetSignalingSocket(socket)
}

func (g *Game) consumeInput() {
	const speed float32 = 4.2 / 60
	const turnSpeed float32 = 0.04
	const lookSensitivity float32 = 0.0024
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.leaveRoom()
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
		g.mouseCaptured = true
		g.lastMouseX, g.lastMouseY = ebiten.CursorPosition()
	}
	if g.mouseCaptured && ebiten.CursorMode() == ebiten.CursorModeCaptured {
		x, y := ebiten.CursorPosition()
		dx := x - g.lastMouseX
		dy := y - g.lastMouseY
		g.lastMouseX = x
		g.lastMouseY = y
		if dx != 0 || dy != 0 {
			g.yaw -= float32(dx) * lookSensitivity
			g.pitch -= float32(dy) * lookSensitivity
			g.pitch = clamp(g.pitch, -1.15, 1.15)
			g.moved = true
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.copyInviteURL()
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.yaw += turnSpeed
		g.moved = true
	}
	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.yaw -= turnSpeed
		g.moved = true
	}
	fx, fy := scene.Forward(g.yaw)
	rx, ry := scene.Right(g.yaw)
	var mx, my float32
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		mx += fx
		my += fy
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		mx -= fx
		my -= fy
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		mx += rx
		my += ry
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		mx -= rx
		my -= ry
	}
	if mx != 0 || my != 0 {
		length := float32(math.Hypot(float64(mx), float64(my)))
		g.x += mx / length * speed
		g.y += my / length * speed
		g.moved = true
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.voiceState == "mic on" {
			g.voice.Disable()
			g.voiceState = "mic off"
		} else {
			if err := g.voice.Enable(); err != nil {
				g.status = "mic error: " + err.Error()
				g.voiceState = "mic off"
				return
			}
			g.voiceState = "mic on"
		}
	}
	if g.voice != nil {
		g.voice.UpdateLocalPosition(g.x, g.y)
	}
}

func (g *Game) sendMoveIfNeeded() {
	if !g.moved || time.Since(g.lastMove) < moveSendInterval || g.worldSocket == nil {
		return
	}
	g.moved = false
	g.lastMove = time.Now()
	_ = g.worldSocket.Send(protocol.EventMove, protocol.MovePayload{
		X:        g.x,
		Y:        g.y,
		Rotation: g.rotation(),
	})
}

func (g *Game) rotation() protocol.Rotation {
	return protocol.Rotation{X: g.pitch, Y: g.yaw, Z: 0}
}

func (g *Game) drawEntry(screen *ebiten.Image) {
	width, height := screenSize(screen)
	panelW := minInt(360, width-40)
	panelH := 260
	panelX := (width - panelW) / 2
	panelY := (height - panelH) / 2
	contentX := panelX + 30
	inputW := panelW - 60

	screen.Fill(color.RGBA{7, 8, 10, 255})
	drawPanel(screen, panelX, panelY, panelW, panelH)
	ebitenutil.DebugPrintAt(screen, "Iolite UI Lab", contentX, panelY+30)
	drawInput(screen, contentX, panelY+75, inputW, "Name", g.entryName, g.active == 0)
	drawInput(screen, contentX, panelY+140, inputW, "Room", g.entryRoom, g.active == 1)
	drawButton(screen, contentX, panelY+210, inputW, "Join")
	ebitenutil.DebugPrintAt(screen, "Tab/Arrows select - Enter join", contentX, panelY+272)
	ebitenutil.DebugPrintAt(screen, "F11 or Alt+Enter fullscreen", contentX, panelY+294)
	ebitenutil.DebugPrintAt(screen, g.status, contentX, panelY+316)
}

func (g *Game) drawRoom(screen *ebiten.Image) {
	width, height := screenSize(screen)
	g.world.Draw(screen)
	drawPanel(screen, 18, 18, 260, 64)
	ebitenutil.DebugPrintAt(screen, "Iolite UI Lab", 52, 30)
	ebitenutil.DebugPrintAt(screen, "pixel-depth voice space", 52, 50)
	drawStatusDot(screen, 34, 44, g.status)
	controlsW := 300
	controlsX := maxInt(18, width-controlsW-18)
	drawPanel(screen, controlsX, 18, controlsW, 58)
	ebitenutil.DebugPrintAt(screen, "[M] "+g.voiceState+"   [I] Invite   [Esc] Leave", controlsX+10, 38)
	drawPanel(screen, 18, 90, 190, 32)
	ebitenutil.DebugPrintAt(screen, g.status, 28, 100)
	hintW := 300
	hintX := (width - hintW) / 2
	hintY := height - 54
	drawPanel(screen, hintX, hintY, hintW, 34)
	ebitenutil.DebugPrintAt(screen, "Click look - WASD move - F11 full", hintX+16, hintY+11)
}

func drawPanel(screen *ebiten.Image, x, y, w, h int) {
	ebitenutil.DrawRect(screen, float64(x+4), float64(y+4), float64(w), float64(h), color.RGBA{10, 104, 79, 255})
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.RGBA{5, 6, 8, 220})
	ebitenutil.DrawLine(screen, float64(x), float64(y), float64(x+w), float64(y), scene.GridLineColor())
	ebitenutil.DrawLine(screen, float64(x), float64(y), float64(x), float64(y+h), scene.GridLineColor())
	ebitenutil.DrawLine(screen, float64(x+w), float64(y), float64(x+w), float64(y+h), scene.GridLineColor())
	ebitenutil.DrawLine(screen, float64(x), float64(y+h), float64(x+w), float64(y+h), scene.GridLineColor())
}

func drawInput(screen *ebiten.Image, x, y, w int, label, value string, active bool) {
	ebitenutil.DebugPrintAt(screen, strings.ToUpper(label), x, y)
	col := color.RGBA{248, 247, 220, 255}
	if active {
		col = color.RGBA{47, 240, 173, 255}
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y+20), float64(w), 42, color.RGBA{23, 25, 29, 255})
	ebitenutil.DrawLine(screen, float64(x), float64(y+20), float64(x+w), float64(y+20), col)
	ebitenutil.DrawLine(screen, float64(x), float64(y+62), float64(x+w), float64(y+62), col)
	ebitenutil.DebugPrintAt(screen, value, x+10, y+34)
}

func drawButton(screen *ebiten.Image, x, y, w int, label string) {
	ebitenutil.DrawRect(screen, float64(x+3), float64(y+3), float64(w), 42, color.RGBA{10, 104, 79, 255})
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 42, color.RGBA{47, 240, 173, 255})
	ebitenutil.DebugPrintAt(screen, label, x+w/2-len(label)*3, y+14)
}

func drawStatusDot(screen *ebiten.Image, x, y int, status string) {
	col := color.RGBA{47, 240, 173, 255}
	if !strings.Contains(status, "joined") && !strings.Contains(status, "connected") {
		col = color.RGBA{255, 79, 122, 255}
	}
	ebitenutil.DrawRect(screen, float64(x), float64(y), 14, 14, col)
}

func trimMax(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func normalizeDefault(s, fallback string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	return s
}

func normalizeRoomID(roomID string) string {
	roomID = strings.TrimSpace(roomID)
	if roomID == "" {
		return "debug"
	}
	if _, err := url.PathUnescape(roomID); err != nil {
		return "debug"
	}
	if len(roomID) > 64 {
		return roomID[:64]
	}
	return roomID
}

func (g *Game) copyInviteURL() {
	inviteURL, err := net.InviteURL(g.cfg.Server, g.cfg.RoomID)
	if err != nil {
		g.status = "invite failed: " + err.Error()
		return
	}
	if err := writeClipboard(inviteURL); err != nil {
		g.status = "invite copy failed: " + err.Error()
		return
	}
	g.status = "invite copied"
}

func writeClipboard(text string) error {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "pbcopy"
	case "linux":
		if _, err := exec.LookPath("wl-copy"); err == nil {
			name = "wl-copy"
		} else {
			name = "xclip"
			args = []string{"-selection", "clipboard"}
		}
	case "windows":
		name = "powershell"
		args = []string{"-NoProfile", "-Command", "Set-Clipboard"}
	default:
		return nil
	}
	cmd := exec.Command(name, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if _, err := stdin.Write([]byte(text)); err != nil {
		_ = stdin.Close()
		return err
	}
	if err := stdin.Close(); err != nil {
		return err
	}
	return cmd.Wait()
}

func altPressed() bool {
	return ebiten.IsKeyPressed(ebiten.KeyAlt) || ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight)
}

func clamp(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func screenSize(screen *ebiten.Image) (int, int) {
	bounds := screen.Bounds()
	return bounds.Dx(), bounds.Dy()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
