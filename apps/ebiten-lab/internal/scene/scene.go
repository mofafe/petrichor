package scene

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/mofafe/petrichor/apps/shared/protocol"
	"github.com/solarlune/tetra3d"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

const FloorSize = 42
const floorScale = float32(FloorSize) / 2

type World struct {
	Scene  *tetra3d.Scene
	Camera *tetra3d.Camera

	floor   *tetra3d.Model
	grid    *tetra3d.Node
	remotes map[string]*RemotePlayer
}

type RemotePlayer struct {
	Root     *tetra3d.Node
	Body     *tetra3d.Model
	Facing   *tetra3d.Model
	Label    *ebiten.Image
	Name     string
	Speaking bool
}

func New(width, height int) *World {
	w := &World{
		Scene:   tetra3d.NewScene("iolite-room"),
		Camera:  tetra3d.NewCamera("camera", width, height),
		remotes: map[string]*RemotePlayer{},
	}
	w.Camera.SetFieldOfView(68)
	w.Camera.SetNear(0.1)
	w.Camera.SetFar(60)
	w.Camera.SetLocalPosition(0, 1.45, 0)

	floor := tetra3d.NewModel("floor", tetra3d.NewPlaneMesh(FloorSize+1, FloorSize+1))
	floor.SetLocalScale(floorScale, 1, floorScale)
	floor.SetLocalPosition(-floorScale/2, 0, -floorScale/2)
	floor.Color = tetra3d.NewColor4(0, 1, 0, 1)
	floor.Shadeless = true
	w.floor = floor
	w.Scene.Root.AddChildren(floor)

	w.grid = w.createGrid()
	w.Scene.Root.AddChildren(w.grid)

	return w
}

func (w *World) Resize(width, height int) {
	w.Camera.Resize(width, height)
}

func (w *World) Draw(screen *ebiten.Image) {
	w.Camera.ClearWithColor(tetra3d.NewColor4(0.56, 0.84, 0.93, 1))
	w.Camera.RenderScene(w.Scene)
	for _, remote := range w.remotes {
		if remote.Label == nil {
			continue
		}
		pos := remote.Root.WorldPosition()
		w.Camera.RenderSprite3D(w.Camera.ColorTexture(), tetra3d.RenderSprite3DSettings{
			Image:         remote.Label,
			WorldPosition: tetra3d.Vector3{X: pos.X, Y: 1.65, Z: pos.Z},
			DepthOffset:   -0.002,
		})
	}
	screen.DrawImage(w.Camera.ColorTexture(), nil)
}

func (w *World) SetCamera(x, z, pitch, yaw float32) {
	w.Camera.SetLocalPosition(x, 1.45, z)
	w.Camera.SetLocalRotation(rotationMatrix(pitch, yaw))
}

func (w *World) UpsertRemotePlayer(player protocol.PlayerState, localUserID string) {
	if player.U == localUserID {
		return
	}
	remote := w.remotes[player.U]
	if remote == nil {
		remote = w.createRemotePlayer(player.U)
		w.remotes[player.U] = remote
	}
	remote.Root.SetLocalPosition(player.X, 0, player.Y)
	remote.Root.SetLocalRotation(rotationMatrix(player.Rotation.X, player.Rotation.Y))
	if remote.Name != player.Name || remote.Speaking != player.Speaking {
		remote.Label = makeNameLabelTexture(player.Name, player.Speaking)
	}
	remote.Name = player.Name
	remote.Speaking = player.Speaking
	if player.Speaking {
		remote.Body.Color = tetra3d.NewColor4(0.96, 1, 0.84, 1)
		remote.Facing.SetLocalScale(0.28, 0.28, 0.18)
		return
	}
	remote.Body.Color = tetra3d.NewColor4(0.42, 0.91, 0.78, 1)
	remote.Facing.SetLocalScale(0.18, 0.18, 0.12)
}

func (w *World) SyncRemotePlayers(players []protocol.PlayerState, localUserID string) {
	seen := map[string]bool{}
	for _, player := range players {
		if player.U == localUserID {
			continue
		}
		seen[player.U] = true
		w.UpsertRemotePlayer(player, localUserID)
	}
	for id := range w.remotes {
		if !seen[id] {
			w.RemoveRemotePlayer(id)
		}
	}
}

func (w *World) RemoveRemotePlayer(id string) {
	remote := w.remotes[id]
	if remote == nil {
		return
	}
	remote.Root.Unparent()
	delete(w.remotes, id)
}

func (w *World) createRemotePlayer(id string) *RemotePlayer {
	root := tetra3d.NewNode("remote-" + id)

	body := tetra3d.NewModel("body", tetra3d.NewCubeMesh(0.45, 1.2, 0.45))
	body.SetLocalPosition(0, 0.65, 0)
	body.Color = tetra3d.NewColor4(0.42, 0.91, 0.78, 1)

	facing := tetra3d.NewModel("facing", tetra3d.NewCubeMesh(0.18, 0.18, 0.12))
	facing.SetLocalPosition(0, 1.05, -0.28)
	facing.Color = tetra3d.NewColor4(1, 0.31, 0.48, 1)
	facing.Shadeless = true

	root.AddChildren(body, facing)
	w.Scene.Root.AddChildren(root)
	return &RemotePlayer{Root: root, Body: body, Facing: facing, Label: makeNameLabelTexture("", false)}
}

func (w *World) createGrid() *tetra3d.Node {
	root := tetra3d.NewNode("grid")
	half := float32(FloorSize) / 2
	const minorThickness float32 = 0.018
	const majorThickness float32 = 0.032

	for i := 0; i <= FloorSize; i++ {
		p := -half + float32(i)
		major := i == FloorSize/2
		thickness := minorThickness
		lineColor := tetra3d.NewColor4(0, 0.345, 0.176, 1)
		if major {
			thickness = majorThickness
			lineColor = tetra3d.NewColor4(0.459, 1, 0.824, 1)
		}

		xLine := tetra3d.NewModel("grid-x", tetra3d.NewCubeMesh(FloorSize, 0.012, thickness))
		xLine.SetLocalPosition(0, 0.035, p)
		xLine.Color = lineColor
		xLine.Shadeless = true

		zLine := tetra3d.NewModel("grid-z", tetra3d.NewCubeMesh(thickness, 0.012, FloorSize))
		zLine.SetLocalPosition(p, 0.036, 0)
		zLine.Color = lineColor
		zLine.Shadeless = true

		root.AddChildren(xLine, zLine)
	}

	return root
}

func GridLineColor() color.Color {
	return color.RGBA{117, 255, 210, 255}
}

func makeNameLabelTexture(name string, speaking bool) *ebiten.Image {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "guest"
	}
	const width = 256
	const height = 64
	img := ebiten.NewImage(width, height)

	border := color.RGBA{47, 240, 173, 255}
	if speaking {
		border = color.RGBA{245, 255, 215, 255}
	}
	ebitenutil.DrawRect(img, 0, 8, width, 42, color.RGBA{5, 6, 8, 209})
	ebitenutil.DrawLine(img, 1.5, 9.5, width-1.5, 9.5, border)
	ebitenutil.DrawLine(img, 1.5, 48.5, width-1.5, 48.5, border)
	ebitenutil.DrawLine(img, 1.5, 9.5, 1.5, 48.5, border)
	ebitenutil.DrawLine(img, width-1.5, 9.5, width-1.5, 48.5, border)

	face := basicfont.Face7x13
	label := trimRunes(name, 28)
	advance := font.MeasureString(face, label).Ceil()
	text.Draw(img, label, face, (width-advance)/2, 35, color.RGBA{248, 247, 220, 255})
	return img
}

func trimRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func rotationMatrix(pitch, yaw float32) tetra3d.Matrix4 {
	yawQ := tetra3d.NewQuaternionFromAxisAngle(tetra3d.Vector3{Y: 1}, yaw)
	pitchQ := tetra3d.NewQuaternionFromAxisAngle(tetra3d.Vector3{X: 1}, pitch)
	return yawQ.Mult(pitchQ).ToMatrix4()
}

func Forward(yaw float32) (float32, float32) {
	return float32(-math.Sin(float64(yaw))), float32(-math.Cos(float64(yaw)))
}

func Right(yaw float32) (float32, float32) {
	return float32(math.Cos(float64(yaw))), float32(-math.Sin(float64(yaw)))
}
