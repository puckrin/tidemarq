#!/usr/bin/env bash
# Run the Playwright e2e suite end-to-end: start an isolated backend on the
# default port, wait for /health, run Playwright, then stop the backend.
#
# Usage:
#   scripts/e2e.sh                    # run all specs
#   scripts/e2e.sh e2e/filters.spec.ts  # run a specific spec
#   scripts/e2e.sh --headed            # any Playwright flag passes through
#
# Why this exists: dev runs need the backend on :8717 (Vite's proxy target),
# isolated data dir, a known admin password, and TIDEMARQ_BACKEND_URL set
# correctly (not TIDEMARQ_URL — that would override Playwright's baseURL).
# Getting any of these wrong produces silent failures. See
# memory/feedback_playwright_dev_runbook.md for the painful history.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"
FRONTEND_DIR="$REPO_ROOT/frontend"
E2E_DATA="$BACKEND_DIR/e2e-data"
BACKEND_URL="https://localhost:8717"

# 1. Guard against split-brain. If anything is already on :8717, the Vite
#    proxy will route browser tests to it instead of the backend we start,
#    and global-setup (which talks to BACKEND_URL directly) will operate on
#    a different DB. Refuse to run rather than producing mysterious failures.
port_in_use() {
  # Try both ss (Linux) and netstat (Windows/Mac) — whichever is available.
  if command -v ss >/dev/null 2>&1; then
    ss -tln 2>/dev/null | grep -q ":8717"
  else
    netstat -an 2>/dev/null | grep -iE "listen" | grep -q ":8717"
  fi
}

if port_in_use; then
  echo "Error: something is already listening on :8717." >&2
  echo "       Vite proxies to that port; an orphan backend causes split-brain." >&2
  echo "       Find the PID with 'netstat -ano | grep 8717' and kill it." >&2
  exit 1
fi

# 2. Wipe and recreate the isolated data dir. We never share state with the
#    user's regular dev backend at backend/dev-data/.
echo "[e2e] Recreating $E2E_DATA"
rm -rf "$E2E_DATA"
mkdir -p "$E2E_DATA"

# 3. Start the backend in the background. All paths are forced to the
#    isolated dir; the admin password is set to admin123 so global-setup
#    can bootstrap → TEST_PASS on the first login.
echo "[e2e] Starting backend"
(
  cd "$BACKEND_DIR"
  TIDEMARQ_SERVER_DATA_DIR="./e2e-data" \
  TIDEMARQ_DATABASE_PATH="./e2e-data/tidemarq.db" \
  TIDEMARQ_TLS_CERT_FILE="./e2e-data/certs/server.crt" \
  TIDEMARQ_TLS_KEY_FILE="./e2e-data/certs/server.key" \
  TIDEMARQ_ADMIN_PASSWORD="admin123" \
  exec go run ./cmd/tidemarq
) > "$E2E_DATA/backend.log" 2>&1 &
BACKEND_PID=$!

# Always stop the backend on exit, success or failure.
cleanup() {
  local exit_code=$?
  echo "[e2e] Stopping backend (PID $BACKEND_PID)"
  kill "$BACKEND_PID" 2>/dev/null || true
  wait "$BACKEND_PID" 2>/dev/null || true
  if [ $exit_code -ne 0 ]; then
    echo "[e2e] Backend log (last 30 lines):" >&2
    tail -30 "$E2E_DATA/backend.log" >&2 || true
  fi
  exit $exit_code
}
trap cleanup EXIT INT TERM

# 4. Wait for /health. Bail early if the backend dies before responding.
echo "[e2e] Waiting for /health"
for i in $(seq 1 60); do
  if curl -sk "$BACKEND_URL/health" >/dev/null 2>&1; then
    echo "[e2e] Backend ready after ${i}s"
    break
  fi
  if ! kill -0 "$BACKEND_PID" 2>/dev/null; then
    echo "Error: backend exited during startup." >&2
    exit 1
  fi
  sleep 1
done
if ! curl -sk "$BACKEND_URL/health" >/dev/null 2>&1; then
  echo "Error: backend never responded to /health within 60s." >&2
  exit 1
fi

# 5. Run Playwright. TIDEMARQ_BACKEND_URL points global-setup at the backend
#    we just started; Playwright's own baseURL stays at the Vite dev server
#    (http://localhost:5173 by default) because TIDEMARQ_URL is unset.
echo "[e2e] Running Playwright"
cd "$FRONTEND_DIR"
TIDEMARQ_BACKEND_URL="$BACKEND_URL" npx playwright test "$@"
