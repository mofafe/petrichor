# WebRTC Negotiation

複数人通話は room 内の各 user pair ごとに 1 つの `RTCPeerConnection` を作る。
音声 media は WebRTC、`offer` / `answer` / `candidate` は signaling WebSocket で中継する。

## offer 衝突対策

peer pair ごとに決定的な役割を持つ。

- `localUserID < remoteUserID` の client は通常の offer 開始側
- `localUserID > remoteUserID` の client は polite peer
- polite peer は offer 衝突時に local offer を rollback して相手 offer を受ける
- impolite peer は offer 衝突時に相手 offer を無視し、自分の offer を続行する

片側だけ speaking の時は、発話側からも offer を開始できる。
同時発話や join 直後で offer が衝突しても、上の polite / impolite ルールで収束させる。

## peer ごとの状態

client は remote user ごとに以下を持つ。

- `makingOffer`: local offer 作成中
- `ignoreOffer`: glare で無視した offer がある状態
- `negotiationQueued`: signaling state が stable になるまで延期した交渉
- `pendingCandidates`: remote description 前に届いた ICE candidate
- retry count / retry timer: 接続失敗時の再試行制御

`signalingState !== "stable"` の間に新しい交渉が必要になった場合は、即時 offer を作らず `negotiationQueued` に積む。
`signalingstatechange` で stable に戻った時に再度 offer を試す。

## candidate 順序ずれ

ICE candidate は `remoteDescription` より先に届くことがある。

- `remoteDescription` 未設定なら `pendingCandidates` に保持する
- `offer` / `answer` 適用後に `pendingCandidates` を順番に `addIceCandidate()` する
- glare で無視した offer 由来の candidate は破棄またはエラー抑制する
- candidate 追加失敗は peer 全体を即 close せず、ログに残して交渉継続する

## 再交渉 / 再試行

接続が `failed` になった場合は peer を作り直して再交渉する。
`disconnected` は一時的なネットワーク揺れの可能性があるため、少し待ってから再試行する。

- 最大 retry: 3 回
- offer / answer 処理失敗: 800ms から retry count に応じて増加
- disconnected: 3000ms から retry count に応じて増加
- retry は通常 offer 開始側、または発話側の offer 開始条件に従う
- `connected` になったら retry count と timer を reset する
- remote user が退出したら retry state も破棄する

## 検証シナリオ

ローカルまたは staging で 3〜5 window を同じ room に参加させ、各 window で mic を順に ON にする。

- 2人: A -> B の順に mic ON、音声が双方向に成立する
- 3人: A/B/C を参加させ、A/B/C の順に mic ON、全 pair が connected へ進む
- 5人: 5 window を参加させ、全員 mic ON、既存接続が failed のまま残らない
- glare: 3〜5 window でほぼ同時に mic ON、offer collision 後も接続が収束する
- reconnect: mic ON 後に 1 window を reload し、残り window と再接続する
- leave: mic ON の user が退出し、他 user の peer/audio 要素が削除される

ブラウザ DevTools では `offer negotiation failed`、`offer handling failed`、`answer handling failed`、`voice negotiation retry exhausted` を確認する。
一時的な retry log は許容するが、最終的に retry exhausted が残る pair は失敗として扱う。
