# Rooms

## Roomとは

複数ユーザーが同じ空間に存在する単位

## 種類

- `public`
  - 一覧に表示される
  - URL でも参加できる
- `private`
  - 一覧に表示されない
  - URL を知っている人だけ参加できる
- `invite`
  - 一覧に表示されない
  - 招待リンクからのみ参加できる
  - 招待リンクは無効化できる

## 最大人数

- MVP の全 Room は最大 6 人
- `full` になった Room には新規参加できない
- 常設VCや人数拡張は MVP の対象外

## 参加方法

- `public`
  - 一覧から参加
  - URL から参加
- `private`
  - URL から参加
- `invite`
  - 招待リンクから参加

## 退出

- ユーザーは任意のタイミングで退出できる
- 接続断やタブクローズ時は自動退出として扱う
- world WebSocket が切断されたら server 側で player state を削除し、残りの world client へ `leave` を通知する
- server は ping / pong timeout でタブクラッシュや回線断を検出する
- mobile sleep 復帰時に client の world WebSocket が閉じていた場合は、Room を reload して再参加する
- 再接続時に同じ座標へ戻すかは未定

## 空部屋

最後の1人が抜けたら5分後削除

## 同時会話

近距離VCにより1部屋内で複数会話可能

## Room状態

- `open`
  - 参加可能
- `full`
  - 定員到達で参加不可
- `locked`
  - owner が明示的に参加受付を止めた状態
  - 既存メンバーは残れる

## 権限

- `owner`
  - Room の作成者
  - `locked` の切り替えができる
  - 招待リンクの再発行や無効化ができる
- `member`
  - 通常参加者
  - 退出と移動のみできる

## 初期同期で必要な情報

- Room ID
- Room 種類
- Room 状態
- 自分の権限
- 現在参加中のメンバー一覧
- 各メンバーの位置、ミュート状態、発話状態

## 未決定

- 再接続時に同じ座標へ戻すか
- owner 不在時に権限を委譲するか
- `private` と `invite` を MVP から両方入れるか、どちらかに絞るか
