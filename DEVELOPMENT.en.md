# DEVELOPMENT.en.md

## Status

In development (early stage)

Current main progress:

- Development is focused first on two-person voice chat

## Features (Planned)

- Voice chat on a flat map
- Volume changes based on distance
- Multi-person rooms
- Join by invitation URL
- Simple, lightweight UI
- Available in the browser

## Tech Stack

- Go
- Gin
- WebSocket
- WebRTC
- Vite
- React
- Three.js

The frontend is being developed with Vite + React + Three.js.

## Development

Start the server:

```bash
make server
```

Start the frontend:

```bash
make web
```

Start both:

```bash
make dev
```

Start staging for multi-person connection testing:

```bash
cp .env.example .env
# Set STAGING_ORIGIN / TURN_* in .env
make deploy-staging
make staging-check
```

`make staging-check` reads `STAGING_ORIGIN` from `.env` and checks connectivity. In staging, share only the `STAGING_ORIGIN` URL with participants. The web container's nginx proxies `/api/*` and `/ws/*` to the Go server, so HTTPS, WSS, and the ICE API appear as the same origin from the browser.

Format and test:

```bash
make fmt
make ci
```

## Workflow

- Development is issue-based
- Work is done on feature branches
- Do not push directly to main
- Merge through pull requests

## Vision

Rather than "joining a voice chat,"  
Iolite aims to create a space where **conversation starts naturally because you are there**.

## Notes

This project is still under development, so specifications and structure may change.

Project name, logo, and branding are not licensed for reuse.
