# Iolite Ebiten Lab

Ebitengine + Tetra3D + Pion/WebRTC による Iolite UI Lab の desktop 移植実験。

`apps/web/iolite/index.html` の entry / room UI と `apps/web/src` の world / signaling protocol を Go 側へ移している。

## Run

別 terminal で server を起動する。

```sh
make server
```

lab を起動する。

```sh
cd apps/ebiten-lab
go run . -name guest -room debug
```

## Controls

- `Tab`: entry の入力欄切り替え
- `Enter`: room に join
- `WASD`: 移動
- `Left / Right`: 視点回転
- `Click`: mouse look capture
- `M`: mic state toggle と WebRTC negotiation 開始
- `I`: Invite URL を clipboard へコピー
- `F11` / `Alt+Enter`: fullscreen 切り替え
- `Esc`: leave

## Notes

- world WebSocket は `/ws/rooms/world/{roomID}` を使う。
- signaling WebSocket は join 後の server assigned user ID で `/ws/rooms/signaling/{roomID}?userID={id}` を開く。
- default 接続先は `https://petrichor.example.com`。ローカル検証時は `-server http://localhost:8080` を指定する。
- ICE server は `{server}/api/ice` から取得し、失敗時は public STUN に fallback する。
- マイク入力は malgo で 48kHz mono S16 を取り、Pion Opus encoder で 20ms packet にして WebRTC track へ流す。
- remote 音声は Pion Opus decoder で PCM に戻し、malgo playback mixer で再生する。
- remote 音量はブラウザ版と同じ 2m-14m の距離減衰で mixer source に反映する。
