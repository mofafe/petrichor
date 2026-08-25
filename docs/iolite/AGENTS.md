# Iolite - AI Agent Guide

## Project Overview
Iolite は平面型ボイスチャット Web アプリケーション。

- Backend: Go + Gin
- Frontend: Vite + React + Three.js
- Realtime Communication: WebSocket / WebRTC

距離減衰型音声コミュニケーションを中心としたリアルタイム空間を提供する。

---

## Current Focus
現在は MVP 開発を優先する。

主なフォーカス:
- 2人通話の安定化
- WebSocket signaling の安定化
- WebRTC offer / answer / ICE candidate フロー整理
- Room 参加・退出・切断処理の安定化
- 最小 UI による接続状態確認
- ローカル / テスト環境での通話検証

見た目より、まず「接続できる・会話できる」を優先する。

---

## Documentation
仕様・背景・設計メモは `docs/iolite/` 配下を参照する。

実装前に関連ドキュメントを確認すること。

docs と実装が矛盾している場合は、勝手に判断せず確認する。

主なドキュメント:
- `docs/iolite/overview.md`
  - プロジェクト全体概要
- `docs/iolite/concept.md`
  - プロダクトコンセプト
- `docs/iolite/mvp.md`
  - MVP の範囲・優先度
- `docs/iolite/rooms.md`
  - Room 管理
- `docs/iolite/websocket.md`
  - WebSocket / Signaling
- `docs/iolite/ui.md`
  - UI 方針

---

## Architecture Policy

### Realtime Communication
- WebSocket は signaling と room state 同期に利用する
- 音声メディア本体は WebRTC を利用する
- signaling message format を勝手に変更しない
- offer / answer / ICE candidate / join / leave の責務を分離する
- 切断時は WebSocket と WebRTC 両方の cleanup を確認する

### Frontend
- UI と通信処理をできるだけ分離する
- Three.js 側に通信ロジックを詰め込みすぎない
- 接続状態を UI から確認できるようにする
- 最初から過度に複雑な UI を作らない

### Backend
- Go 標準ライブラリを優先する
- 不要な依存追加を避ける
- 過剰な abstraction を避ける
- 可読性を優先する

---

## Code Organization
- 1ファイルに処理を詰め込みすぎない
- 責務が大きくなったら分割する
- ただし早すぎる分割は避ける
- 小さく整理しながら進める
- 無関係な変更を混ぜない

---

## Workflow
- 作業前に関連 Issue と docs を確認する
- 大きな変更は Issue または PR に目的を書く
- main へ直接 push しない
- feature branch で作業する
- PR 経由でマージする
- マージ前に `make ci` を実行する
- テスト失敗時は原因を確認する
- 無関係な format / refactor を混ぜない

---

## Boundaries
以下は勝手に変更しない。

- `.env*`
- 本番環境設定
- 認証方式
- 通信方式
- 主要アーキテクチャ

大規模リファクタは Issue 作成後のみ行う。

---

## Code Style
- 既存命名規則を尊重する
- コメントは必要な箇所だけ簡潔に書く
- 「何をしているか」より「なぜそうするか」を優先して書く
- Room 管理 / WebSocket / WebRTC / 並行処理は必要に応じて説明を書く
- 公開関数や複雑な関数には短い説明を付ける

---

## Commit Rules
Conventional Commits を使用する。

本文は日本語で記述する。

種類:
- feat
- fix
- docs
- chore

例:
- `feat: room join処理を追加`
- `fix: websocket切断時のcleanup修正`
- `docs: signaling仕様を更新`

---

## Change Rules
- 既存設計を尊重する
- 大きな変更は小さな差分に分ける
- 迷う変更は確認を優先する
- 変更理由がある場合は説明を書く
