#!/usr/bin/env bash
# Smoke test for the precast wall grouting backend. It builds the server, starts
# it on a local port, probes its health and API endpoints, exercises a real
# create/read task flow, and cleans up every process and temporary file. It uses
# no external network and never pipes curl into grep (responses are captured in
# variables and asserted).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP="$(mktemp -d)"
SERVER_BIN="$TMP/server"
LOG="$TMP/events.log"
SNAP="$TMP/snapshot.bin"
PORT="${PORT:-18473}"
ADDR="127.0.0.1:$PORT"
SERVER_PID=""

cleanup() {
  if [[ -n "$SERVER_PID" ]]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

# Build the server using the local Go toolchain.
(cd "$HERE" && go build -o "$SERVER_BIN" ./cmd/server)

# Start the service with a logical clock and file persistence.
"$SERVER_BIN" -addr "$ADDR" -event-log "$LOG" -snapshot "$SNAP" -clock logical &
SERVER_PID=$!

# Wait for the health endpoint to come up (bounded, deterministic).
ready=0
for _ in $(seq 1 100); do
  if curl -sf "http://$ADDR/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.05
done
if [[ "$ready" != "1" ]]; then
  echo "server did not become ready" >&2
  exit 1
fi

# Assert liveness.
health="$(curl -sf "http://$ADDR/healthz")"
if [[ "$health" != *'"status":"ok"'* ]]; then
  echo "healthz unexpected: $health" >&2
  exit 1
fi

# Assert readiness (reports the event log and snapshot state).
ready_resp="$(curl -sf "http://$ADDR/readyz")"
if [[ "$ready_resp" != *'"Ready":true'* ]]; then
  echo "readyz unexpected: $ready_resp" >&2
  exit 1
fi

# Exercise a real API flow: create a task, then read it back.
created="$(curl -sf -X POST -H 'Content-Type: application/json' \
  -d '{"taskId":"SMOKE-1","building":"B1","level":"L1","wallPanel":"W1"}' \
  "http://$ADDR/api/v1/tasks")"
if [[ "$created" != *'"SMOKE-1"'* ]]; then
  echo "create task failed: $created" >&2
  exit 1
fi

task="$(curl -sf "http://$ADDR/api/v1/tasks/SMOKE-1")"
if [[ "$task" != *'"SMOKE-1"'* ]]; then
  echo "get task failed: $task" >&2
  exit 1
fi

# Persistence files must have been created by the committed write.
if [[ ! -s "$SNAP" ]]; then
  echo "snapshot file was not written" >&2
  exit 1
fi

echo "smoke ok"
