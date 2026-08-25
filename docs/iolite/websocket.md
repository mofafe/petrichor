# WebSocket

room ごとに WebSocket endpoint を分ける。

- world: room state と位置同期
- signaling: WebRTC の offer / answer / ICE candidate 中継
- chat: chat message

message format は共通で `{ "t": eventType, "u": userID, "d": payload }`。
`u` は server から client へ送る時に server が付与する。

## signal

endpoint `ws/rooms/signaling/{:roomID}?userID={worldUserID}`

- offer
- answer
- candidate

`userID` は world endpoint の `join` 応答で server が割り当てた ID を使う。
signaling endpoint では room state を更新しない。
`answer` と `candidate` は payload の `target` に入った user ID の signaling connection へ送る。
`offer` は `target` があればその相手へ送り、なければ送信者以外の signaling connection へ broadcast する。

WebRTC 交渉ルールと再試行方針は `docs/webrtc-negotiation.md` を参照する。

## world

endpoint `ws/rooms/world/{:roomID}`

- join
- leave
- move
- state_sync
- speak_start
- speak_stop
- ping
- pong
- room_full

world endpoint は player state を管理する。
signaling event や chat event は受け付けない。

## chat

endpoint `ws/rooms/chat/{:roomID}`

- chat

chat endpoint は chat event だけを扱う。
