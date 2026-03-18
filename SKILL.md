---
name: termlive
description: |
  Terminal live monitoring + IM bridge for AI coding tools.
  Two components: Go Core (terminal PTY, Web UI, HTTP API) and Node.js Bridge
  (Claude Agent SDK, Telegram/Discord/Feishu, permission approval).
  Use for: setting up, starting, stopping, or diagnosing TermLive;
  any phrase like "termlive", "terminal monitoring", "终端监控", "远程终端",
  "启动服务", "诊断", "查看日志", "配置IM", "手机上看终端".
  Subcommands: setup, start, stop, status, logs, reconfigure, doctor.
  Do NOT use for: building standalone bots, webhook integrations, or general
  programming tasks — those are regular coding tasks.
argument-hint: "setup | start | stop | status | logs [N] | reconfigure | doctor"
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - AskUserQuestion
  - Grep
  - Glob
---

# TermLive Skill

You are managing TermLive — a terminal live monitoring platform with IM bridge.

TermLive has **two components**:
1. **Go Core** (`tlive-core`) — PTY management, Web UI dashboard, HTTP API, WebSocket streaming. Pure infrastructure.
2. **Node.js Bridge** — Claude Agent SDK, IM platform adapters (Telegram/Discord/Feishu), permission approval, message delivery.

They communicate via HTTP API + WebSocket. Core must start first, Bridge connects to it.

User data is stored at `~/.termlive/`.

The skill directory (SKILL_DIR) is the repo root where this SKILL.md lives.
If unclear, Glob `**/SKILL.md` with content match "termlive" to find it.

## Directory Layout

```
SKILL_DIR/                    # Repo root (= skill directory)
├── SKILL.md                  # This file
├── config.env.example        # Template for ~/.termlive/config.env
├── core/                     # Go source → builds to tlive-core binary
│   ├── cmd/tlive-core/
│   ├── internal/
│   └── web/                  # Embedded Web UI
├── bridge/                   # Node.js source → builds to bridge/dist/main.mjs
│   ├── src/
│   ├── package.json
│   └── dist/main.mjs         # Built bundle (after npm run build)
├── scripts/
│   ├── daemon.sh             # Process management (start/stop/status/logs)
│   ├── doctor.sh             # Diagnostics
│   └── statusline.sh         # Claude Code status line
└── docker-compose.yml
```

Runtime directory (`~/.termlive/`):
```
~/.termlive/
├── bin/
│   ├── tlive-core            # Go Core binary (built or downloaded)
│   └── statusline.sh         # Claude Code status line script
├── config.env                # Credentials & settings (chmod 600)
├── data/                     # Bridge persistence (sessions, bindings)
├── logs/
│   ├── core.log              # Go Core log
│   └── bridge.log            # Bridge log
└── runtime/
    ├── core.pid              # Go Core PID
    └── bridge.pid            # Bridge PID
```

## Command Parsing

Parse the user's intent from `$ARGUMENTS`:

| User says (examples) | Subcommand |
|---|---|
| `setup`, `configure`, `配置`, `帮我连接 Telegram`, `install` | setup |
| `start`, `启动`, `启动服务` | start |
| `stop`, `停止`, `关闭` | stop |
| `status`, `状态`, `运行状态` | status |
| `logs`, `logs 200`, `日志`, `查看日志` | logs |
| `reconfigure`, `重新配置`, `帮我改一下配置` | reconfigure |
| `doctor`, `diagnose`, `诊断`, `挂了`, `没反应了` | doctor |

**`status` vs `doctor`**: Use `status` for informational checks. Use `doctor` when the user reports a problem. Symptoms like "挂了", "没反应了" → `doctor`.

## Runtime Detection

1. **Claude Code** — `AskUserQuestion` is available. Use it for interactive wizards.
2. **Other** — show `SKILL_DIR/config.env.example` and instruct manual setup.

## Config Check (all commands except `setup`)

Before any subcommand other than `setup`, check `~/.termlive/config.env`:

- **Missing**: In Claude Code → auto-start `setup`. Otherwise → show config.env.example and stop.
- **Exists**: proceed.

## Subcommands

### `setup`

Interactive setup wizard. Handles **both** component installation and IM configuration.

**Phase 1 — Build components**

Step 1: Create directories:
```bash
mkdir -p ~/.termlive/{bin,data,logs,runtime}
```

Step 2: Build Go Core binary:
```bash
# Check if already exists
if [ ! -x ~/.termlive/bin/tlive-core ]; then
  # Option A: Build from source (requires Go 1.24+)
  if command -v go &>/dev/null; then
    cd SKILL_DIR/core && go build -o ~/.termlive/bin/tlive-core ./cmd/tlive-core/
  else
    echo "Go not found. Please install Go 1.24+ or download prebuilt binary."
    echo "See: https://github.com/termlive/termlive/releases"
  fi
fi
```

Step 3: Build Node.js Bridge:
```bash
cd SKILL_DIR/bridge && npm install && npm run build
```

Verify both built:
```bash
~/.termlive/bin/tlive-core --help 2>&1 | head -1
ls SKILL_DIR/bridge/dist/main.mjs
```

**Phase 2 — Configure IM platforms**

Collect input **one field at a time**. After each answer, confirm the value (mask secrets to last 4 chars).

Step 4: Choose channels:
```
AskUserQuestion: "Which IM platforms to enable?
1. Telegram — streaming preview, inline permission buttons
2. Discord — team use, channel-level access control
3. Feishu (飞书) — streaming cards, tool progress
Enter numbers (e.g., 1,3):"
```

Step 5: Collect credentials per channel:

**Telegram**: Bot Token (from @BotFather) → Chat ID (optional) → Allowed User IDs (optional).
At least one of Chat ID or Allowed User IDs should be set.

**Discord**: Bot Token → Allowed User IDs → Allowed Channel IDs (optional).
At least one of User IDs or Channel IDs should be set. Enable **Message Content Intent** in Developer Portal.

**Feishu**: App ID → App Secret → Allowed User IDs (optional).
Must configure bot capability and permissions in Feishu console.

**Phase 3 — General settings**

Step 6: Collect:
- **Public URL** for web monitoring links in IM messages (e.g., `https://termlive.example.com`). Optional but recommended.
- **Port**: default 8080.

Auto-generate: TL_TOKEN (32-char hex).

**Phase 4 — Write config and verify**

Step 7:
1. Show summary table (secrets masked)
2. Ask user to confirm
3. Write `~/.termlive/config.env` with all settings, `chmod 600`
4. Copy status line script: `cp SKILL_DIR/scripts/statusline.sh ~/.termlive/bin/ && chmod +x ~/.termlive/bin/statusline.sh`
5. Tell user: "Setup complete! Run `/termlive start` to start the service."

### `start`

Starts **both** Go Core and Node.js Bridge in the correct order.

**Pre-checks:**
1. Config exists (`~/.termlive/config.env`)
2. Go Core binary exists (`~/.termlive/bin/tlive-core`)
3. Bridge is built (`SKILL_DIR/bridge/dist/main.mjs`)

**Startup sequence:**
```
1. Start Go Core (tlive-core daemon --port $TL_PORT --token $TL_TOKEN)
   └── Background process, logs to ~/.termlive/logs/core.log
   └── PID written to ~/.termlive/runtime/core.pid
2. Wait for Core HTTP API to be healthy
   └── Poll GET /api/status every 0.5s, up to 10s
3. Start Node.js Bridge (node SKILL_DIR/bridge/dist/main.mjs)
   └── Background process, logs to ~/.termlive/logs/bridge.log
   └── PID written to ~/.termlive/runtime/bridge.pid
   └── Bridge registers with Core (POST /api/bridge/register)
   └── Bridge connects to IM platforms
```

Execute:
```bash
bash "SKILL_DIR/scripts/daemon.sh" start
```

Show output. If it fails:
- "Go Core failed to start" → check `~/.termlive/logs/core.log`, run `/termlive doctor`
- "Bridge not built" → `cd SKILL_DIR/bridge && npm run build`

**Start Core only (without IM bridge):**
Users who just want terminal monitoring + Web UI without IM can run Core standalone:
```bash
~/.termlive/bin/tlive-core daemon --port 8080 --token <token>
```

### `stop`

Stops **Bridge first, then Core** (reverse order):

```bash
bash "SKILL_DIR/scripts/daemon.sh" stop
```

```
1. Kill Bridge (SIGTERM) → Bridge disconnects from IM, stops heartbeat
2. Kill Core (SIGTERM) → Core stops HTTP server, closes sessions
```

Order matters: Bridge should disconnect cleanly before Core shuts down.

### `status`

```bash
bash "SKILL_DIR/scripts/daemon.sh" status
```

Also query API for detailed info:
```bash
source ~/.termlive/config.env 2>/dev/null
curl -sf "http://localhost:${TL_PORT:-8080}/api/status" \
  -H "Authorization: Bearer ${TL_TOKEN}" 2>/dev/null
```

Shows:
- Go Core: running/stopped, PID, port, active sessions, version
- Bridge: running/stopped, PID, connected IM channels
- Stats: token usage, cost

### `logs`

```bash
bash "SKILL_DIR/scripts/daemon.sh" logs ${N:-50}
```

Shows both Core and Bridge logs interleaved.

### `reconfigure`

1. Read current `~/.termlive/config.env`
2. Show current settings (secrets masked to last 4 chars)
3. AskUserQuestion: "What do you want to change?"
4. Update config
5. Remind: "Run `/termlive stop` then `/termlive start` to apply."

### `doctor`

```bash
bash "SKILL_DIR/scripts/doctor.sh"
```

Checks and fixes:

| Check | Fix |
|---|---|
| Node.js < 22 | Install Node.js >= 22 |
| Go Core binary missing | `cd SKILL_DIR/core && go build -o ~/.termlive/bin/tlive-core ./cmd/tlive-core/` |
| Bridge not built | `cd SKILL_DIR/bridge && npm install && npm run build` |
| Config missing | Run `/termlive setup` |
| Stale PID file | `/termlive stop` then `/termlive start` |
| Core API unreachable | Check core.log for errors |
| Bridge not connected | Check bridge.log, verify IM tokens |

## Process Lifecycle

```
/termlive start
    │
    ├── 1. Go Core starts (background)
    │       ├── Listens on :8080 (HTTP API + WebSocket)
    │       ├── Serves Web UI dashboard at /
    │       ├── Terminal WebSocket at /ws/session/:id
    │       ├── Status WebSocket at /ws/status
    │       └── PID → ~/.termlive/runtime/core.pid
    │
    ├── 2. Wait for Core healthy (poll /api/status, up to 10s)
    │
    └── 3. Node.js Bridge starts (background)
            ├── Reads ~/.termlive/config.env
            ├── POST /api/bridge/register → Core
            ├── Heartbeat every 15s → POST /api/bridge/heartbeat
            ├── Connects to enabled IM platforms
            ├── Routes IM messages → Claude Agent SDK → IM responses
            └── PID → ~/.termlive/runtime/bridge.pid

/termlive stop
    │
    ├── 1. Kill Bridge (SIGTERM) → clean disconnect
    └── 2. Kill Core (SIGTERM) → close sessions

Core crash → Bridge enters degraded mode, retries (backoff, max 5min)
Bridge crash → Core continues serving Web UI, marks Bridge disconnected
```

## Notes

- Always mask secrets in output (last 4 chars only).
- Go Core can run standalone for terminal monitoring without Bridge/IM.
- Bridge requires Core to be running — it connects to Core's HTTP API.
- Both processes log to `~/.termlive/logs/` with secret redaction.
- Config persists at `~/.termlive/config.env` — survives across sessions.
