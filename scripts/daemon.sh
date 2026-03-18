#!/bin/bash
# TermLive process management — manages Go Core + Node.js Bridge
set -euo pipefail

TERMLIVE_HOME="${HOME}/.termlive"
RUNTIME_DIR="${TERMLIVE_HOME}/runtime"
LOG_DIR="${TERMLIVE_HOME}/logs"
BIN_DIR="${TERMLIVE_HOME}/bin"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# SKILL_DIR is the repo root (one level up from scripts/)
SKILL_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Source config if exists
[ -f "${TERMLIVE_HOME}/config.env" ] && set -a && source "${TERMLIVE_HOME}/config.env" && set +a

TL_PORT="${TL_PORT:-8080}"
TL_TOKEN="${TL_TOKEN:-}"

ensure_dirs() {
  mkdir -p "$RUNTIME_DIR" "$LOG_DIR" "$BIN_DIR"
}

is_running() {
  local pidfile="$1"
  [ -f "$pidfile" ] && kill -0 "$(cat "$pidfile")" 2>/dev/null
}

wait_for_core() {
  for i in $(seq 1 20); do
    if curl -sf "http://localhost:${TL_PORT}/api/status" \
         -H "Authorization: Bearer ${TL_TOKEN}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
  done
  echo "ERROR: Go Core failed to start"
  return 1
}

start() {
  ensure_dirs

  if is_running "$RUNTIME_DIR/core.pid"; then
    echo "Go Core is already running (PID $(cat "$RUNTIME_DIR/core.pid"))"
  else
    echo "Starting Go Core..."
    if [ ! -x "$BIN_DIR/tlive-core" ]; then
      echo "ERROR: $BIN_DIR/tlive-core not found."
      echo "Build: cd ${SKILL_DIR}/core && go build -o ${BIN_DIR}/tlive-core ./cmd/tlive-core/"
      echo "Or run: /termlive setup"
      exit 1
    fi
    "$BIN_DIR/tlive-core" daemon --port "$TL_PORT" --token "$TL_TOKEN" \
      >> "$LOG_DIR/core.log" 2>&1 &
    echo $! > "$RUNTIME_DIR/core.pid"
    wait_for_core
    echo "Go Core started (PID $(cat "$RUNTIME_DIR/core.pid"))"
  fi

  if is_running "$RUNTIME_DIR/bridge.pid"; then
    echo "Bridge is already running (PID $(cat "$RUNTIME_DIR/bridge.pid"))"
  else
    echo "Starting Bridge..."
    local bridge_entry="${SKILL_DIR}/bridge/dist/main.mjs"
    if [ ! -f "$bridge_entry" ]; then
      echo "ERROR: Bridge not built."
      echo "Build: cd ${SKILL_DIR}/bridge && npm install && npm run build"
      echo "Or run: /termlive setup"
      exit 1
    fi
    node "$bridge_entry" >> "$LOG_DIR/bridge.log" 2>&1 &
    echo $! > "$RUNTIME_DIR/bridge.pid"
    echo "Bridge started (PID $(cat "$RUNTIME_DIR/bridge.pid"))"
  fi
}

stop() {
  if is_running "$RUNTIME_DIR/bridge.pid"; then
    echo "Stopping Bridge (PID $(cat "$RUNTIME_DIR/bridge.pid"))..."
    kill "$(cat "$RUNTIME_DIR/bridge.pid")" 2>/dev/null || true
    rm -f "$RUNTIME_DIR/bridge.pid"
  else
    echo "Bridge is not running"
  fi

  if is_running "$RUNTIME_DIR/core.pid"; then
    echo "Stopping Go Core (PID $(cat "$RUNTIME_DIR/core.pid"))..."
    kill "$(cat "$RUNTIME_DIR/core.pid")" 2>/dev/null || true
    rm -f "$RUNTIME_DIR/core.pid"
  else
    echo "Go Core is not running"
  fi
}

status() {
  echo "=== TermLive Status ==="
  if is_running "$RUNTIME_DIR/core.pid"; then
    echo "Go Core:  running (PID $(cat "$RUNTIME_DIR/core.pid"))"
    curl -sf "http://localhost:${TL_PORT}/api/status" \
      -H "Authorization: Bearer ${TL_TOKEN}" 2>/dev/null | head -1 || echo "  (API unreachable)"
  else
    echo "Go Core:  stopped"
  fi

  if is_running "$RUNTIME_DIR/bridge.pid"; then
    echo "Bridge:   running (PID $(cat "$RUNTIME_DIR/bridge.pid"))"
  else
    echo "Bridge:   stopped"
  fi
}

logs() {
  local n="${1:-50}"
  echo "=== Go Core (last $n lines) ==="
  tail -n "$n" "$LOG_DIR/core.log" 2>/dev/null || echo "(no log file)"
  echo ""
  echo "=== Bridge (last $n lines) ==="
  tail -n "$n" "$LOG_DIR/bridge.log" 2>/dev/null || echo "(no log file)"
}

case "${1:-}" in
  start)  start ;;
  stop)   stop ;;
  status) status ;;
  logs)   logs "${2:-50}" ;;
  *)      echo "Usage: $0 {start|stop|status|logs [N]}" ;;
esac
