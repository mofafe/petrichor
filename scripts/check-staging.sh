#!/bin/sh
set -eu

origin="${1:-${STAGING_ORIGIN:-}}"
room_id="${2:-debug}"

if [ -z "$origin" ]; then
  echo "usage: STAGING_ORIGIN=https://dev.example.com $0 [origin] [room_id]" >&2
  exit 2
fi

origin="${origin%/}"
ws_origin="$(printf '%s' "$origin" | sed 's#^https://#wss://#; s#^http://#ws://#')"

echo "checking web: $origin/"
curl -fsS "$origin/" >/dev/null

echo "checking ice api: $origin/api/ice"
curl -fsS "$origin/api/ice" >/dev/null

echo "checking websocket upgrade: $ws_origin/ws/rooms/world/$room_id"
tmp_body="$(mktemp)"
trap 'rm -f "$tmp_body"' EXIT

status="$(
  curl -sS -o "$tmp_body" -w '%{http_code}' --max-time 5 --http1.1 \
    -H 'Connection: Upgrade' \
    -H 'Upgrade: websocket' \
    -H 'Sec-WebSocket-Version: 13' \
    -H 'Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==' \
    "$origin/ws/rooms/world/$room_id"
)"

if [ "$status" != "101" ]; then
  echo "websocket upgrade failed: status=$status" >&2
  if [ -s "$tmp_body" ]; then
    echo "response body:" >&2
    cat "$tmp_body" >&2
    echo >&2
  fi
  exit 1
fi

echo "staging checks passed"
