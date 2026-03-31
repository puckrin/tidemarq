# tidemarq — Claude Code Guide

## What this project is

tidemarq is a self-hosted directory synchronisation tool with a browser-based UI. It runs as a Docker Compose stack with no external cloud dependencies. Full functional requirements are in `sync_app_spec.md`. The approved UI design is in `prototype.html` (reference this when building any frontend view). Design tokens, colours, and component specs are in `tidemarq-style-guide.docx`.

---

## Tech stack

| Layer | Choice | Reason |
|---|---|---|
| Backend | Go | Single binary, low memory, trivial ARM64 cross-compilation, strong concurrency |
| Frontend | React + TypeScript + Vite | Component model suits the SPA, good Claude Code support |
| Database | SQLite (`modernc/sqlite`) | Pure Go — no CGO, required for cross-compilation. PostgreSQL supported via config |
| Migrations | `golang-migrate` | SQL migration files in `backend/migrations/` |
| FS watching | `fsnotify` | Cross-platform inotify/FSEvents/ReadDirectoryChangesW |
| WebSocket | `gorilla/websocket` | Progress streaming to the UI |
| Auth | `golang-jwt/jwt` + `bcrypt` | JWT sessions, bcrypt password storage |
| SFTP | `pkg/sftp` | SSH/SFTP mount support |
| SMB | `hirochachacha/go-smb2` | SMB/CIFS mount support |
| Config | `viper` | YAML/TOML config + env var support |
| Container | Docker multi-stage build | Final image from `gcr.io/distroless/static` |

---

## Project structure

```
tidemarq/
├── CLAUDE.md
├── docker-compose.yml
├── Dockerfile
├── tidemarq.yaml              # default runtime config
├── sync_app_spec.md           # functional specification
├── prototype.html             # approved UI reference
├── tidemarq-style-guide.docx  # design tokens and component specs
│
├── backend/
│   ├── cmd/tidemarq/main.go   # entrypoint
│   ├── go.mod
│   ├── migrations/            # SQL migration files (up + down)
│   └── internal/
│       ├── api/               # HTTP handlers, router, middleware
│       ├── auth/              # JWT issuance, bcrypt, session validation
│       ├── config/            # Config struct, loading, validation
│       ├── db/                # Database connection, query helpers
│       ├── engine/            # Core sync: delta transfer, checksums, idempotency
│       ├── jobs/              # Job CRUD, lifecycle state machine, scheduler
│       ├── manifest/          # File manifest read/write
│       ├── mounts/            # SMB, SFTP mount management
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

# Frontend dev server (from frontend/)
npm install
npm run dev

# Full stack via Docker
docker compose up --build

# Backend tests
go test ./...

# Frontend tests
npm test
```

The backend serves the built frontend from `frontend/dist/` in production. In development the frontend dev server proxies API calls to the backend.

---

## Architectural rules — read these before writing any code

**Sync correctness**
- All sync operations must be idempotent. Running a job twice with no file changes must produce zero mutations and zero errors.
- Never mutate the destination without first updating the file manifest transactionally. A crash mid-sync must leave the system in a consistent, resumable state — not a corrupt one.
- All file integrity checks use SHA-256. Verify after transfer, not before.
- Soft delete always: never hard-delete a file from the destination directly. Move it to quarantine first.

**Concurrency**
- Directory scanning is parallelised across subdirectories using goroutines with a bounded worker pool. Do not scan serially.
- WebSocket progress events are broadcast via a hub pattern — handlers never write directly to connections.
- Job state transitions (idle → running → paused etc.) are protected by a mutex. No job state is mutated outside the jobs package.

**Security**
- Every HTTP handler (except `/health`) requires a valid JWT. Middleware enforces this — handlers do not check auth themselves.
- Passwords are always bcrypt-hashed. Never log or store plaintext credentials.
- Network mount credentials (SMB username/password, SSH keys) are stored encrypted at rest.
- No telemetry, no outbound connections except to configured mount targets and notification endpoints.

**Configuration**
- Config is loaded once at startup from `tidemarq.yaml` and environment variables. Env vars override the file. UI changes to sync defaults and user accounts write back to the config file.
- Never read config from the filesystem at request time. Pass config as a dependency.

**API design**
- REST API is versioned under `/api/v1/`.
- Use structured JSON error responses: `{ "error": "...", "code": "..." }`.
- WebSocket endpoint is `/ws` — authenticated via a short-lived token issued by `/api/v1/auth/ws-token`.

**Frontend**
- The approved UI is in `prototype.html` — match it exactly for layout, colours, and component behaviour. Do not invent new UI patterns.
- All design tokens (colours, typography, spacing) come from the style guide. They are defined as CSS custom properties in `src/styles/tokens.css`.
- File paths and directory names always render in monospace (Courier New / system-mono) at 13px.
- Never use red (`--coral`) for anything other than errors and destructive actions.

---

## Testing requirements

- All functions in `internal/engine/`, `internal/manifest/`, and `internal/conflicts/` must have unit tests.
- Integration tests that hit a real SQLite database are required for all database operations — do not mock the database.
- Frontend: component tests for all shared components in `src/components/`.
- Before marking any sync feature complete, verify idempotency: run the job twice, assert no mutations on the second run.

---

## Build phases

Work in this order. Complete and test each phase before starting the next. Do not mark a phase complete unless all exit criteria are met.

---

### Phase 1 — Scaffold & auth

**Build:** Docker Compose stack, project structure, DB schema + migrations, JWT auth, user management API, `/health` endpoint.

**Exit criteria:**
- `docker compose up` starts cleanly; server is reachable at `https://localhost:8443`
- HTTP on port 80 redirects to HTTPS; a self-signed certificate is auto-generated on first start
- `GET /health` returns JSON with app version, database connectivity status, and uptime
- `POST /api/v1/auth/login` with valid credentials returns a signed JWT
- `POST /api/v1/auth/login` with invalid credentials returns 401
- Any API endpoint called without a token returns 401; expired or tampered tokens also return 401
- A default admin account is created on first start if no users exist (credentials from config)
- Full user CRUD (`GET /PUT /POST /DELETE /api/v1/users`) works and is Admin-only
- Role enforcement is verified: Operator and Viewer tokens are rejected by Admin-only endpoints
- Stopping and restarting the container preserves all data; SQLite file lives in the volume mount
- All migrations run automatically on startup with no manual steps
- `go test ./...` passes

---

### Phase 2 — Core sync engine

**Build:** File manifest, one-way backup mode, checksum verification, delta transfer.

**Exit criteria:**
- A sync job can be created via API with a source path, destination path, and mode `one-way-backup`
- Running the job copies all files from source to destination
- File integrity is verified via SHA-256 after each transfer; a mismatch fails the job with an error
- Running the same job a second time with no file changes produces zero mutations (idempotency)
- Modifying a file on the source and re-running syncs only the changed file
- Deleting a file on the source and re-running does not remove it from the destination (backup mode)
- Transfers respect a configured bandwidth limit (KB/s) when set
- File metadata is preserved: last modified timestamp and POSIX permissions
- `go test ./internal/engine/... ./internal/manifest/...` passes including idempotency tests

---

### Phase 3 — Job management

**Build:** Job CRUD, cron + FS watch triggers, lifecycle state machine, WebSocket progress streaming.

**Exit criteria:**
- Full job CRUD via API; jobs persist across restarts
- A job with a cron trigger fires at the scheduled time after a container restart
- A job with a filesystem watch trigger fires within 5 seconds of a file change on the source
- A running job can be paused and resumed via API; no data corruption results from interruption
- A paused or cancelled job resumes from a safe checkpoint on next trigger
- WebSocket at `/ws` streams live progress events (files transferred, rate, ETA) during an active job
- Job state is always one of: `idle`, `running`, `paused`, `error`, `disabled`
- All scheduled jobs resume automatically after a container restart
- `go test ./internal/jobs/...` passes

---

### Phase 4 — Conflict & versioning

**Build:** Two-way sync, conflict detection, quarantine, soft delete, version history restore.

**Exit criteria:**
- Two-way sync propagates changes from source to destination and vice versa
- A file modified on both sides since the last sync is detected as a conflict
- Conflicts are not auto-resolved when strategy is `ask-user`; both versions are preserved and the destination copy is renamed with a `.conflict.<timestamp>` suffix
- All configured auto-resolution strategies (newest-wins, largest-wins, source-wins, destination-wins) resolve correctly
- Deleting a file in mirror or two-way mode moves it to quarantine, not permanent deletion
- A soft-deleted file can be restored via API during its retention period
- Previous file versions are stored and restorable via API
- `go test ./internal/conflicts/... ./internal/versions/...` passes

---

### Phase 5 — Frontend

**Build:** React SPA wired to the API, converting `prototype.html` view by view.

**Exit criteria:**
- All six views are implemented and match `prototype.html`: Dashboard, Sync Jobs, Job Detail, Conflict Queue, Audit Log, Settings
- New Job wizard completes all 5 steps and creates a real job via the API
- Live job progress updates via WebSocket without page refresh
- Conflict resolution actions (keep source, keep dest, keep both) call the API and update the UI
- Dark and light mode work correctly; OS preference is respected on first load
- The app is fully functional with JavaScript from the built `frontend/dist/` served by the backend

---

### Phase 6 — Mounts & notifications

**Build:** SMB/SFTP mount config, SMTP/webhook/Gotify notifications, audit log export.

**Exit criteria:**
- A named SMB mount can be configured, tested for connectivity, and used as a job source or destination
- A named SFTP mount can be configured with password or SSH key auth, and used as a job source or destination
- Configured notification events (job failed, conflict detected, etc.) fire to SMTP and webhook targets
- The audit log is searchable and filterable by job, event type, and date range via the API
- Audit log entries can be exported as CSV and JSON

---

### Phase 7 — Hardening

**Build:** Multi-arch Docker images, ACME TLS, PostgreSQL support, performance validation.

**Exit criteria:**
- Docker images build and run correctly on both `linux/amd64` and `linux/arm64`
- ACME certificate provisioning and renewal works when an ACME domain is configured
- Switching database type to PostgreSQL in config works without code changes
- The file manifest can be built and maintained for a directory tree of 1,000,000+ files without UI degradation
- `go test ./...` passes on both architectures

---

## What not to do

- Do not add features, abstractions, or config options not in the spec.
- Do not mock the database in integration tests.
- Do not use CGO — the build must cross-compile cleanly.
- Do not write to the destination filesystem outside of the engine package.
- Do not add cloud storage destinations (out of scope for v1.0, per spec §1.3).
- Do not skip soft delete and go straight to permanent deletion.
