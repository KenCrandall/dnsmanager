# dnsmanager

`dnsmanager` is a control plane for `dnsmasq` that combines a web UI and a Go-based CLI for DNS, DHCP, TFTP, PXE, and operational visibility.

The target deployment is a two-container Docker Compose application:

- A `dnsmanager` app container running the backend API, embedded web UI, metrics collection, and config orchestration.
- A companion `dnsmasq` container that reads runtime configuration and related content from shared volumes managed by the app.

## Product surfaces

The web UI is being built to provide:

- A Pi-hole-inspired dashboard with DNS, DHCP, and service health graphs.
- A staged config editor with validation, diff preview, apply, rollback, and drift detection.
- Editors for DNS zones, DHCP settings, TFTP configuration, and PXE configuration.
- A DHCP lease manager and live log viewer.

The CLI is being built to provide:

- Remote task-oriented administration for common DNS and DHCP workflows.
- Resource CRUD for managed records, reservations, options, and revisions.
- Human-readable and automation-friendly output formats.

## Architecture

- Backend: Go service exposing REST APIs plus live event/log streams.
- Frontend: Svelte single-page application served by the Go backend.
- CLI: Go utility built with `spf13/cobra`.
- State: SQLite for users, tokens, audit events, staged revisions, managed objects, and short-retention metrics.
- Runtime: Docker Compose with shared config, data, and content volumes.

## Configuration model

The app is planned around a hybrid config ownership model:

- `managed/` contains structured config rendered from app-managed data.
- `manual/` preserves hand-managed or unsupported snippets.
- `generated/` contains rendered outputs and staging artifacts.

Changes will follow a staged apply pipeline:

1. Save edits as a staged revision.
2. Render config into staging.
3. Validate with `dnsmasq --test`.
4. Show a diff and validation result.
5. Publish to the shared volume atomically.
6. Trigger a controlled reload/restart of the companion `dnsmasq` container.

## Foundation slice

The repository now includes a verified foundation scaffold:

- `compose.yaml` for the `dnsmanager` app container and companion `dnsmasq` container.
- A Go backend with `healthz`, runtime status, shared-volume bootstrapping, and static UI serving.
- A Cobra-based CLI with an initial `status` command against the backend API.
- A Svelte/Vite frontend shell with a Pi-hole-inspired dashboard layout.
- A starter SQLite schema for settings, users, tokens, revisions, audit events, and metrics.

## Quick start

### Local backend + CLI

Build the frontend bundle first so the Go server can serve it directly:

```bash
cd web
npm install
npm run build
```

Start the backend from the repository root:

```bash
go run ./cmd/dnsmanagerd
```

Query the status API with the CLI:

```bash
go run ./cmd/dnsmanager status
```

### Docker Compose

Bring up the full foundation stack:

```bash
docker compose up --build
```

The app is exposed on `http://127.0.0.1:8080`.

The companion `dnsmasq` container is exposed on:

- `1053/tcp` and `1053/udp` for DNS
- `1067/udp` for DHCP
- `1069/udp` for TFTP

## Repository layout

- `cmd/dnsmanagerd`: backend entrypoint
- `cmd/dnsmanager`: Cobra CLI entrypoint
- `internal/config`: runtime configuration and shared-volume layout
- `internal/server`: HTTP handlers and filesystem bootstrapping
- `internal/client`: API client shared by CLI commands
- `db/schema.sql`: initial SQLite schema
- `docker/dnsmasq`: companion container build context
- `web`: Svelte/Vite frontend

## Roadmap

The canonical living roadmap is maintained in [docs/plan.md](docs/plan.md).

## Current status

The repository has completed its foundation milestone:

- Local Go builds and tests pass.
- The Svelte frontend builds successfully.
- The backend and Cobra CLI have been smoke-tested together.
- The Compose stack has been built and brought up successfully with the companion `dnsmasq` container reading the shared volume.

The next milestone is the controlled config lifecycle: import, backup, staged rendering, diff, validation, and apply/rollback behavior.
