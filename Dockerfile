# ── Frontend build ────────────────────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json frontend/package-lock.json* ./
RUN npm ci --ignore-scripts
COPY frontend/ .
RUN npm run build

# ── Backend build ─────────────────────────────────────────────────────────────
FROM golang:1.23-alpine AS backend-builder

ARG VERSION=dev

WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/tidemarq/tidemarq/internal/api.Version=${VERSION}" \
    -o tidemarq ./cmd/tidemarq

# ── Final image ───────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12

COPY --from=backend-builder /build/tidemarq /tidemarq
COPY --from=frontend-builder /frontend/dist /app/frontend/dist
COPY tidemarq.example.yaml /etc/tidemarq/tidemarq.yaml

EXPOSE 8080 8443

ENTRYPOINT ["/tidemarq"]
