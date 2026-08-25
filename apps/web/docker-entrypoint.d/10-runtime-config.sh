#!/bin/sh
set -eu

CONFIG_FILE="${CONFIG_FILE:-/usr/share/nginx/html/config.js}"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

WORLD_WS_ORIGIN_VALUE="$(json_escape "${WORLD_WS_ORIGIN:-}")"
ICE_API_ORIGIN_VALUE="$(json_escape "${ICE_API_ORIGIN:-}")"

cat > "$CONFIG_FILE" <<EOF
window.__PETRICHOR_CONFIG__ = {
  worldWsOrigin: "$WORLD_WS_ORIGIN_VALUE",
  iceApiOrigin: "$ICE_API_ORIGIN_VALUE",
};
window.__FLATTALKING_CONFIG__ = window.__PETRICHOR_CONFIG__;
EOF
