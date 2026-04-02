# dnsmanager Living Plan

## Summary

`dnsmanager` will be a Docker Compose-based control plane for `dnsmasq` with:

- A `dnsmanager` application container for API, web UI, metrics, and orchestration.
- A companion `dnsmasq` container that reads configuration and related content from shared volumes.
- Two operator surfaces: a web UI and a Go CLI built with `spf13/cobra`.

The web UI will provide a dashboard, staged configuration management, DNS/DHCP/TFTP/PXE editing, DHCP lease management, and log viewing. The CLI will provide remote task-oriented administration and CRUD access over the same backend API.

## Success Criteria

- Operators can manage `dnsmasq` safely through staged changes rather than live file editing.
- The app owns managed config generation while preserving manual/legacy snippets cleanly.
- The companion `dnsmasq` container can consume config from shared volumes without manual intervention.
- The dashboard provides Pi-hole-inspired operational visibility with short-retention analytics.
- The CLI and web UI share the same backend semantics for validation, apply, audit, and visibility.

## Current Defaults

### Deployment model

- Docker Compose with a `dnsmanager` app container and a companion `dnsmasq` container.
- Shared volumes:
  - Config volume for rendered dnsmasq config tree.
  - Data volume for SQLite, backups, staged revisions, and import metadata.
  - Content volume for TFTP/PXE files and future asset lifecycle management.
- `dnsmanager` is the writer for managed outputs.
- `dnsmasq` is the reader of published config and content.

### Planned stack

- Backend: Go
- Frontend: Svelte SPA
- CLI: Go with `spf13/cobra`
- Persistence: SQLite
- Streaming: SSE and/or WebSockets for live events and logs

### Config ownership

- Hybrid model with `managed/`, `manual/`, and `generated/` areas.
- Conservative first-run import with backups before any rewrite.
- Staged apply flow with validation, diff preview, atomic publish, and controlled reload/restart.

### Product defaults

- Dashboard inspired by Pi-hole graph layouts and operational summaries.
- DNS editor support for `A`, `AAAA`, `CNAME`, `PTR`, `TXT`, `SRV`, and host overrides.
- PXE v1 includes boot directives, boot entry management, and simple iPXE file creation.
- Full PXE/TFTP boot-asset lifecycle management is a later milestone and should not be blocked by v1 schema choices.
- CLI is API-backed only in v1 and authenticates with API tokens.
- Metrics retention target is 24 hours in v1.

## Milestones

| Milestone | Status | Notes |
| --- | --- | --- |
| 1. Repository bootstrap | done | Git repo initialized, README added, living plan established, minimal `.gitignore` added. |
| 2. Foundation | done | Compose scaffold, shared volumes, backend shell, Svelte shell, SQLite schema, base CLI client, and Compose/runtime verification completed. |
| 3. Controlled config lifecycle | done | SQLite-backed revisions, staged rendering, dnsmasq validation, apply/rollback primitives, CLI/API exposure, and Compose validation verification completed. |
| 4. Managed editors and APIs | planned | DNS, DHCP, TFTP, PXE/iPXE object model, validation, rendering, REST endpoints. |
| 5. CLI v1 | planned | Cobra command tree, token config, task commands, CRUD commands, output formatting. |
| 6. Operations views | planned | Lease manager, live logs, apply status, drift warnings. |
| 7. Dashboard and analytics | planned | Query-log ingestion, 24h roll-ups, Pi-hole-style graphs, CLI dashboard summary. |
| 8. Future PXE/TFTP expansion | planned | Full boot-asset lifecycle management, upload, versioning, organization, validation, reference tracking. |

## Current Implementation Focus

The next implementation slice should establish the first managed editors and APIs:

- Introduce structured DNS record models and rendering into managed fragments.
- Add CRUD endpoints for common local DNS record types on top of the revision lifecycle.
- Define how managed object changes create or update draft revisions rather than writing files directly.
- Extend the frontend beyond the dashboard shell into the first real editor surface.
- Keep CLI commands aligned with the same managed-object APIs and revision workflow.

## Acceptance Criteria For Next Slice

- The backend can store at least one managed DNS object type separately from raw rendered text.
- Managed DNS changes can be rendered into the generated config fragment via the revision lifecycle.
- CRUD APIs exist for the first managed DNS editor surface.
- The frontend can create and review at least one managed DNS change.
- The CLI can inspect or mutate the same managed DNS objects through the API.

## Open Questions

- Which live-update mechanism should be preferred first: SSE everywhere, or WebSockets only where needed?
- What initial token and local-auth bootstrap flow should be used for the first operator account?
- Which config diff representation will be most useful across both UI and CLI?
- How much existing dnsmasq config should the first import pass promote versus preserve as manual snippets?
- Which managed DNS object shape should come first: host overrides only, or a broader common-record model immediately?

## Deferred Scope

- Full PXE/TFTP boot-asset lifecycle management.
- SSO/OIDC auth providers.
- Multi-host or centrally managed remote `dnsmasq` fleets.
- Long-term analytics retention beyond the initial 24-hour target.
- Direct host-management mode in the CLI.

## Decision Log

| Date | Decision |
| --- | --- |
| 2026-04-01 | Use Docker Compose with separate `dnsmanager` and companion `dnsmasq` containers. |
| 2026-04-01 | Use a shared-volume model where `dnsmanager` writes managed config and `dnsmasq` reads it. |
| 2026-04-01 | Use Go for the backend and CLI, and Svelte for the web UI. |
| 2026-04-01 | Keep the initial repository slice documentation-first. |
| 2026-04-01 | Treat `docs/plan.md` as the canonical living roadmap. |
| 2026-04-01 | Use a custom Alpine-based companion `dnsmasq` image for the initial Compose scaffold. |
| 2026-04-01 | Verify the foundation slice with local Go/CLI/frontend builds plus a successful Compose stack startup. |
| 2026-04-01 | Back the first config lifecycle with SQLite revisions plus filesystem staging instead of direct live-file mutation. |
| 2026-04-01 | Install `dnsmasq` in the app container so Compose validation can run `dnsmasq --test` directly. |

## Test Checklist

- [x] The repository bootstrap docs reflect the agreed architecture and roadmap.
- [x] The living plan records milestones, current defaults, open questions, and the next slice.
- [x] Foundation scaffold exists and can be executed locally.
- [x] Companion container/shared-volume integration is verified.
- [x] API, CLI, and frontend foundation pieces are connected.
- [x] Draft config revisions can be created and stored in SQLite.
- [x] Validation, apply, and rollback can be exercised through the API and Cobra CLI.
- [x] Compose validation returns a real `dnsmasq --test` success result inside the app container.
