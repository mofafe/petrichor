# MVP

## Goal

最初に公開する Iolite の最小構成版。  
「URLを開いて入室し、平面空間で近い人と自然に会話できる」体験を提供する。

## Core Experience

1. URLを開く
2. 表示名を入力する
3. Room に参加する
4. 平面空間で移動できる
5. 距離に応じて音量が変わる
6. 自然に会話が生まれる

## Included Features

### Room

- 1 Room 構成（初期）
- ランダムRoom URL
- 一定時間で失効する一時Room
- 人数制限あり

### User

- ログイン不要
- 初回参加時に表示名入力
- 内部IDは UUID を利用

### Voice Chat

- WebRTC による音声通話
- 距離減衰
- 近い人ほど聞こえやすい

### Map

- シンプルな2D平面
- ユーザー位置同期
- 自由移動

## Limits

- 1 Room 最大 6 人（仮）
- 常設Roomなし
- テキストチャットなし
- モバイル最適化なし

## Not Included

- アカウント登録
- フレンド機能
- 複数Room管理
- 高度なUI演出
- 課金機能
- 常設VC
- カスタムURL

## Initial Release Plan

### Phase 1

友達限定テスト公開

### Phase 2

小規模一般公開

## Success Criteria

- 友達同士で実際に会話して遊べる
- 距離減衰が体験として面白い
- 接続が安定している
- また使いたいと言われる

## Notes

MVPでは完成度よりも、体験の中核価値を優先する。
