# DEVELOPMENT.md

## Status

開発中（初期段階）

現在の主な進捗:

- 2人通話を最初の目標として開発中

## Features (Planned)

- 平面マップ上でのボイスチャット
- 距離に応じた音量変化
- 複数人ルーム
- 招待URLで参加
- シンプルで軽いUI
- ブラウザで利用可能

## Tech Stack

- Go
- TypeScript
- Gin
- WebSocket
- WebRTC
- Vite
- React
- Three.js

フロントエンドは Vite + React + Three.js ベースで開発中です。  

## Development

サーバー起動:

```bash
make server
```

フロントエンド起動:

```bash
make web
```

同時起動:

```bash
make dev
```

複数人で接続確認する staging 起動:

```bash
cp .env.example .env
# .env の STAGING_ORIGIN / TURN_* を設定
make deploy-staging
make staging-check
```

`make staging-check` は `.env` の `STAGING_ORIGIN` を読んで疎通確認します。staging では参加者に `STAGING_ORIGIN` の URL だけを共有します。web コンテナの nginx が `/api/*` と `/ws/*` を Go server に proxy するため、ブラウザからは HTTPS / WSS / ICE API が同じ origin に見えます。

整形・テスト:

```bash
make fmt
make ci
```

## Workflow

- Issue ベースで開発します
- 作業は feature branch で行います
- main へ直接 push しません
- PR 経由でマージします

## Vision

「VCに入る」よりも、  
**そこに居たら自然に会話が始まる空間** を目指しています。

## Notes

このプロジェクトは開発途中のため、仕様・構成は変更される場合があります。

Project name, logo, and branding are not licensed for reuse.
