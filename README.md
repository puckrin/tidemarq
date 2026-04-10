# tidemarq

A self-hosted directory synchronisation tool with a browser-based UI.
Runs as a single Docker container with no external cloud dependencies.

## Features

- **One-way backup** — copy files from source to destination; deletions are never propagated
- **One-way mirror** — keep destination an exact replica of source; removals move files to quarantine
- **Two-way sync** — propagate changes in both directions with configurable conflict resolution
- **Conflict resolution** — automatic strategies (newest-wins, largest-wins, source-wins, dest-wins) or manual review queue
- **Version history** — previous file versions stored and restorable
- **Quarantine** — soft-delete with configurable retention before permanent removal
- **Network mounts** — SMB/CIFS and SFTP sources and destinations
- **Scheduling** — cron triggers and filesystem-watch triggers
- **Live progress** — real-time transfer progress via WebSocket
- **Audit log** — full history of all sync activity, exportable as CSV or JSON
- **Role-based access** — admin, operator, and viewer roles
- **Dark / light mode** — respects OS preference

## Quick start

### Prerequisites

- Docker and Docker Compose

### 1. Create a `docker-compose.yml`

```yaml
services:
  tidemarq:
    image: ghcr.io/puckrin/tidemarq:latest
    ports:
      - "80:8080"
      - "8443:8443"
    volumes:
      - tidemarq_data:/data
    environment:
      - TIDEMARQ_AUTH_JWT_SECRET=<generate with: openssl rand -base64 32>
      - TIDEMARQ_ADMIN_PASSWORD=changeme
    restart: unless-stopped

volumes:
  tidemarq_data:
```

### 2. Start

```bash
docker compose up -d
```

### 3. Open the UI

Navigate to `https://localhost:8443` and log in with username `admin` and the
password you set in `TIDEMARQ_ADMIN_PASSWORD`.

A self-signed TLS certificate is generated automatically on first start.
Accept the browser warning or supply your own certificate via config.

---

## Configuration

All configuration can be supplied via environment variables or a `tidemarq.yaml`
file mounted at `/etc/tidemarq/tidemarq.yaml`. Environment variables take
precedence. See [`tidemarq.example.yaml`](tidemarq.example.yaml) for the full
reference.

### Required

| Environment variable | Description |
|---|---|
| `TIDEMARQ_AUTH_JWT_SECRET` | Secret used to sign session tokens. Minimum 32 characters. Generate with `openssl rand -base64 32`. |
| `TIDEMARQ_ADMIN_PASSWORD` | Password for the built-in admin account created on first start. |

### Optional

| Environment variable | Default | Description |
|---|---|---|
| `TIDEMARQ_SERVER_HTTP_PORT` | `8080` | HTTP port (redirects to HTTPS) |
| `TIDEMARQ_SERVER_HTTPS_PORT` | `8443` | HTTPS port |
| `TIDEMARQ_SERVER_DATA_DIR` | `/data` | Persistent storage root — mount a volume here |
| `TIDEMARQ_AUTH_JWT_TTL` | `24h` | Session token lifetime |
| `TIDEMARQ_ADMIN_USERNAME` | `admin` | Username for the built-in admin account |
| `TIDEMARQ_TLS_CERT_FILE` | *(auto-generated)* | Path to TLS certificate |
| `TIDEMARQ_TLS_KEY_FILE` | *(auto-generated)* | Path to TLS private key |

### Using your own TLS certificate

Mount your certificate and key into the container and point to them:

```yaml
environment:
  - TIDEMARQ_TLS_CERT_FILE=/data/certs/fullchain.pem
  - TIDEMARQ_TLS_KEY_FILE=/data/certs/privkey.pem
volumes:
  - /etc/letsencrypt/live/yourdomain:/data/certs:ro
  - tidemarq_data:/data
```

---

## Building from source

```bash
# Backend
cd backend
go build ./cmd/tidemarq

# Frontend
cd frontend
npm install
npm run build

# Full stack (Docker)
docker compose up --build
```

### Running in development

```bash
# Backend (from backend/)
go run ./cmd/tidemarq

# Frontend dev server with HMR (from frontend/)
npm run dev
```

The frontend dev server proxies API calls to the backend.

---

## Tech stack

| Layer | Technology |
|---|---|
| Backend | Go — single binary, no CGO |
| Frontend | React + TypeScript + Vite |
| Database | SQLite (pure Go, no CGO) — PostgreSQL supported via config |
| Container | Docker multi-stage — distroless final image |

---

## Contributing

Pull requests are welcome. For significant changes please open an issue first
to discuss the approach.

```bash
# Run backend tests
cd backend && go test ./...

# Run frontend tests
cd frontend && npm test
```

---

## License

[MIT](LICENSE)
