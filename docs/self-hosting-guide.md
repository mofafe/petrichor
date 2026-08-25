# Self-hosting Guide

Petrichor は Docker Compose で web / server / coturn をまとめて起動できます。

## Requirements

- Docker
- Docker Compose
- HTTPS を終端できる reverse proxy
- TURN 用の公開 IP アドレス
- TURN 用の TLS 証明書

## Setup

`.env.example` をコピーして `.env` を作成します。

```bash
cp .env.example .env
```

`.env` に以下を設定します。

```env
STAGING_ORIGIN=https://your-petrichor.example.com
TURN_EXTERNAL_IP=203.0.113.10
TURN_SECRET=replace-with-a-random-secret
TURN_REALM=turn.example.com
TURN_CERT_DIR=/etc/letsencrypt/live/turn.example.com
TURN_CERT_ARCHIVE_DIR=/etc/letsencrypt/archive/turn.example.com
WORLD_WS_ORIGIN=
ICE_API_ORIGIN=
```

`TURN_SECRET` は公開しないでください。十分に長いランダム文字列を使います。

## Start

```bash
docker compose up -d --build
```

staging 用の override を使う場合:

```bash
docker compose -f compose.yaml -f compose.staging.yaml up -d --build
```

または Makefile から起動できます。

```bash
make deploy-staging
```

## Reverse Proxy

公開 URL では、web コンテナに HTTPS で到達できるようにします。
web コンテナ内の nginx が同一 origin の `/api/` と `/ws/` を server に proxy します。

外部 reverse proxy では WebSocket upgrade を有効にしてください。

必要な公開経路:

- `/`
- `/iolite/`
- `/iolite/r`
- `/api/`
- `/ws/`

## TURN Ports

coturn は以下を公開します。

- `3478/tcp`
- `3478/udp`
- `5349/tcp`
- `5349/udp`
- `49160-49200/udp`

サーバーの firewall でも同じ port を許可してください。

## Check

`.env` の `STAGING_ORIGIN` を設定した状態で疎通確認できます。

```bash
make staging-check
```

`make staging-check` は HTTP endpoint と WebSocket upgrade を確認します。

## Notes

- `.env` は commit しないでください。
- `TURN_SECRET` を変更した場合は coturn と server を再起動してください。
- ブラウザでマイクを使うには HTTPS が必要です。ただし `localhost` は例外として扱われます。
