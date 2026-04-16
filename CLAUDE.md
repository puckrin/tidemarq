# tidemarq — Claude Code Guide

## What this project is

tidemarq is a self-hosted directory synchronisation tool with a browser-based UI. It runs as a Docker Compose stack with no external cloud dependencies. Full functional requirements are in `sync_app_spec.md`.

---

## Tech stack

| Layer | Choice | Reason |
|---|---|---|
| Backend | Go | Single binary, low memory, trivial ARM64 cross-compilation, strong concurrency |
| Frontend | React + TypeScript + Vite | Component model suits the SPA, good Claude Code support |
| Database | SQLite (`modernc/sqlite`) | Pure Go — no CGO, required for cross-compilation |
| Migrations | `golang-migrate` | SQL migration files in `backend/migrations/` |
| FS watching | `fsnotify` | Cross-platform inotify/FSEvents/ReadDirectoryChangesW |
| WebSocket | `gorilla/websocket` | Progress streaming to the UI |
| Auth | `golang-jwt/jwt` + `bcrypt` | JWT sessions, bcrypt password storage |
| SFTP | `pkg/sftp` | SSH/SFTP mount support |
| SMB | `hirochachacha/go-smb2` | SMB/CIFS mount support |
| Hashing | BLAKE3 (default) + SHA-256 | Per-job algorithm selection; BLAKE3 for performance, SHA-256 for compatibility |
| Delta transfer | Adler-32 rolling checksum + BLAKE3 | rsync-style local delta; network mounts use full streaming |
| Config | `viper` | YAML/TOML config + env var support |
| Container | Docker multi-stage build | Final image from `gcr.io/distroless/static` |

---

## Project structure

```
tidemarq/
├── CLAUDE.md
├── docker-compose.yml
├── Dockerfile
├── tidemarq.example.yaml      # reference config with all options documented
├── sync_app_spec.md           # functional specification
│
├── backend/
│   ├── cmd/tidemarq/main.go   # entrypoint
│   ├── go.mod
│   ├── migrations/            # SQL migration files (up + down)
│   └── internal/
│       ├── api/               # HTTP handlers, router, middleware
│       ├── auth/              # JWT issuance, bcrypt, session validation
│       ├── config/            # Config struct, loading, validation
│       ├── crypt/             # AES-256-GCM encryption for credentials at rest
│       ├── db/                # Database connection, query helpers
│       ├── delta/             # Rolling-checksum delta transfer (Adler-32 + BLAKE3)
│       ├── engine/            # Core sync: delta transfer, checksums, idempotency
│       ├── hasher/            # Hash algorithm abstraction (BLAKE3, SHA-256)
│       ├── jobs/              # Job CRUD, lifecycle state machine, scheduler
│       ├── manifest/          # File manifest read/write
│       ├── mountfs/           # SMB and SFTP filesystem implementations
│       ├── mounts/            # Mount config management, credential encryption
│       ├── notifications/     # SMTP, webhook, Gotify notification dispatch
│       ├── watch/             # Filesystem event watching
│       ├── conflicts/         # Conflict detection and resolution
│       ├── versions/          # Version history, soft delete, quarantine
│       ├── audit/             # Audit log writes and queries
│       └── ws/                # WebSocket hub, progress event broadcasting
│
└── frontend/
    ├── index.html
    ├── package.json
    ├── vite.config.ts
    ├── tsconfig.json
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── api/               # Typed API client (REST + WebSocket)
        ├── components/        # Shared UI components (Button, Badge, Card, etc.)
        ├── views/             # Page-level components matching prototype views
        ├── hooks/             # Custom React hooks
        ├── store/             # Client state (React context or Zustand)
        └── styles/            # Design tokens as CSS custom properties
```

---

## Running the project

```bash
# Backend (from backend/)
go run ./cmd/tidemarq

# Frontend dev server (from frontend/) — proxies API to https://localhost:8717
npm install
npm run dev

# Full stack via Docker — UI at https://localhost:8717
docker compose up --build

# Backend tests
go test ./...

# Frontend tests
npm test
```

### Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8716 | HTTP | Redirects to HTTPS |
| 8717 | HTTPS | Main UI and API |

The backend serves the built frontend from `frontend/dist/` in production. In development the frontend dev server proxies API calls to the backend.

---

## Architectural rules — read these before writing any code

**Sync correctness**
- All sync operations must be idempotent. Running a job twice with no file changes must produce zero mutations and zero errors.
- Never mutate the destination without first updating the file manifest transactionally. A crash mid-sync must leave the system in a consistent, resumable state — not a corrupt one.
- File integrity checks use BLAKE3 by default; SHA-256 is supported per job for compatibility. Verify after transfer, not before.
- Soft delete always: never hard-delete a file from the destination directly. Move it to quarantine first.

**Concurrency**
- Directory scanning is parallelised across subdirectories using goroutines with a bounded worker pool. Do not scan serially.
- WebSocket progress events are broadcast via a hub pattern — handlers never write directly to connections.
- Job state transitions (idle → running → paused etc.) are protected by a mutex. No job state is mutated outside the jobs package.

**Security**
- Every HTTP handler (except `/health`) requires a valid JWT. Middleware enforces this — handlers do not check auth themselves.
- Passwords are always bcrypt-hashed. Never log or store plaintext credentials.
- Network mount credentials (SMB username/password, SSH keys) are stored encrypted at rest using AES-256-GCM (`internal/crypt`).
- The JWT signing secret is auto-generated on first start and persisted to `<data_dir>/.jwt_secret`. It can be overridden via config or `TIDEMARQ_AUTH_JWT_SECRET`.
- No telemetry, no outbound connections except to configured mount targets and notification endpoints.

**Configuration**
- Config is loaded once at startup from `tidemarq.yaml` and environment variables. Env vars override the file.
- Never read config from the filesystem at request time. Pass config as a dependency.

**API design**
- REST API is versioned under `/api/v1/`.
- Use structured JSON error responses: `{ "error": "...", "code": "..." }`.
- WebSocket endpoint is `/ws` — authenticated via a short-lived token issued by `/api/v1/auth/ws-token`.

**Frontend**
- Match the existing implemented views for layout, colours, and component behaviour. Do not invent new UI patterns.
- All design tokens (colours, typography, spacing) are defined as CSS custom properties in `src/styles/tokens.css`.
- File paths and directory names always render in monospace (Courier New / system-mono) at 13px.
- Never use red (`--coral`) for anything other than errors and destructive actions.

---

## Testing requirements

- All functions in `internal/engine/`, `internal/manifest/`, `internal/conflicts/`, and `internal/delta/` must have unit tests.
- Integration tests that hit a real SQLite database are required for all database operations — do not mock the database.
- Frontend: component tests for all shared components in `src/components/`.
- Idempotency must be verified for any sync feature: run the job twice, assert no mutations on the second run.

---

## What exists

The following describes the current implemented state of the application. Use this as the reference for what is already built before adding anything new.

---

### Phase 1 — Scaffold & auth ✓

- Docker Compose stack; server runs at `https://localhost:8717` (HTTP 8716 redirects)
- Self-signed TLS certificate auto-generated on first start if none is configured
- JWT signing secret auto-generated on first start, persisted to `<data_dir>/.jwt_secret`
- `GET /health` returns app version, database connectivity, and uptime
- `POST /api/v1/auth/login` issues a signed JWT; invalid credentials return 401
- All endpoints except `/health` require a valid JWT; expired or tampered tokens return 401
- Default admin account created on first start if no users exist (username and password from config or env)
- Full user CRUD at `/api/v1/users` — Admin-only; Operator and Viewer tokens are rejected
- SQLite database lives in the volume mount; data survives container restarts
- All schema migrations run automatically on startup via `golang-migrate`

---

### Phase 2 — Core sync engine ✓

- Jobs support `one-way-backup` and `one-way-mirror` modes
- **Hash algorithm abstraction** (`internal/hasher`): BLAKE3 is the default; SHA-256 is supported for per-job compatibility. The algorithm used is stored in the manifest per entry
- **Delta transfer** (`internal/delta`): rolling Adler-32 + BLAKE3 strong hash for rsync-style local transfers. Only changed byte regions are written. Falls back to full streaming for network mounts or when delta offers less than 10% savings
- **CDC deduplication** (`feature/cdc-deduplication`): chunk-level content-defined chunking using BLAKE3 — in development on the current branch
- File integrity verified after each transfer; hash mismatch fails the job with an error
- Running the same job twice with no file changes produces zero mutations (idempotency)
- `one-way-backup` mode never removes destination files when a source file is deleted
- Transfers respect a configured bandwidth limit (KB/s)
- File metadata preserved: last modified timestamp and POSIX permissions
- `go test ./internal/engine/... ./internal/manifest/... ./internal/delta/...` passes including idempotency tests

---

### Phase 3 — Job management ✓

- Full job CRUD via API; jobs persist across restarts
- Cron triggers using `robfig/cron/v3`; schedules survive container restarts
- Filesystem watch triggers via `fsnotify` with a 3-second debounce quiet period
- Jobs can be paused mid-transfer via API; the engine checks for pause signals between file transfers and mid-file at chunk boundaries
- Paused or cancelled jobs resume from a safe checkpoint on next trigger
- WebSocket at `/ws` streams live progress events: files transferred, bytes, rate, ETA, current file
- WebSocket connections authenticated via a short-lived token from `/api/v1/auth/ws-token`
- Job states: `idle`, `running`, `paused`, `error`, `disabled`; transitions are mutex-protected in the jobs package
- All scheduled jobs re-register their triggers automatically after a container restart

---

### Phase 4 — Conflict & versioning ✓

- **Two-way sync**: changes propagate source→destination and destination→source (local filesystem only; network mounts fall back to one-way)
- Conflict detection compares source hash, destination hash, and last-synced hash; a file changed on both sides since the last sync is a conflict
- `ask-user` strategy: destination copy renamed with `.conflict.<timestamp>` suffix; source version written; no auto-resolution
- Auto-resolution strategies implemented: `newest-wins`, `largest-wins`, `source-wins`, `destination-wins`
- Deleted files in mirror or two-way mode move to quarantine (`.tidemarq-quarantine/`), never permanently deleted
- Quarantine entries tracked in DB with configurable retention (default 30 days); restorable via `/api/v1/quarantine/{id}/restore`
- Previous file versions snapshotted before each overwrite to `<data_dir>/versions/<jobID>/<relPath>/`; restorable via `/api/v1/versions/{id}/restore`

---

### Phase 5 — Frontend ✓

- React SPA served from `frontend/dist/` in production; dev server proxies to the backend
- Nine views implemented: Dashboard, Sync Jobs, Job Detail, Conflict Queue, Quarantine, Audit Log, Settings, Mounts, Login
- New Job wizard (multi-step) creates jobs via the API
- Live job progress via WebSocket — no page refresh required
- Conflict resolution actions (keep source, keep dest, keep both) call the API and update the UI inline
- Dark and light mode; OS preference respected on first load
- Component tests for all shared components in `src/components/`

---

### Phase 6 — Mounts & notifications ✓

- Named SMB mounts: configurable, connectivity-testable, usable as job source or destination
- Named SFTP mounts: password or SSH key auth; usable as job source or destination
- Mount credentials encrypted at rest with AES-256-GCM (`internal/crypt`)
- Notification targets: SMTP, webhook (HTTP POST/PUT with custom headers), Gotify
- Notification rules map events (`job_failed`, `conflict_detected`, etc.) to one or more targets
- Audit log searchable and filterable by job, event type, and date range via `/api/v1/audit`
- Audit log export as CSV and JSON

---

### Phase 7 — Hardening ✓

- Pure Go build (`CGO_ENABLED=0`, `modernc/sqlite`): cross-compiles cleanly for `linux/amd64` and `linux/arm64`
- Final Docker image from `gcr.io/distroless/static-debian12` — minimal attack surface
- `go test ./...` passes on both architectures

---

## What not to do

- Do not add features, abstractions, or config options not in the spec.
- Do not mock the database in integration tests.
- Do not use CGO — the build must cross-compile cleanly.
- Do not write to the destination filesystem outside of the engine package.
- Do not add cloud storage destinations (out of scope for v1.0, per spec §1.3).
- Do not skip soft delete and go straight to permanent deletion.
