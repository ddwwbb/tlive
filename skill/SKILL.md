---
name: termlive
description: |
  Terminal live monitoring + IM bridge for AI coding tools.
  Monitor terminal sessions remotely, approve tool permissions from
  Telegram/Discord/Feishu, view real-time terminal output in browser.
  Use for: setting up, starting, stopping, or diagnosing the TermLive service;
  any phrase like "termlive", "terminal monitoring", "终端监控", "远程终端",
  "启动服务", "诊断", "查看日志", "配置IM".
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

You are managing the TermLive service — a terminal live monitoring platform with IM bridge.
User data is stored at `~/.termlive/`.

The skill directory (SKILL_DIR) is at `~/.claude/skills/termlive`.
If that path doesn't exist, fall back to Glob with pattern `**/skills/**/termlive/SKILL.md` and derive the root from the result.

## Command Parsing

Parse the user's intent from `$ARGUMENTS` into one of these subcommands:

| User says (examples) | Subcommand |
|---|---|
| `setup`, `configure`, `配置`, `帮我连接 Telegram`, `install` | setup |
| `start`, `启动`, `启动服务`, `start bridge` | start |
| `stop`, `停止`, `关闭`, `stop bridge` | stop |
| `status`, `状态`, `运行状态`, `怎么看状态` | status |
| `logs`, `logs 200`, `日志`, `查看日志` | logs |
| `reconfigure`, `重新配置`, `帮我改一下配置` | reconfigure |
| `doctor`, `diagnose`, `诊断`, `挂了`, `没反应了`, `出问题了` | doctor |

**Disambiguation: `status` vs `doctor`** — Use `status` when the user just wants to check if TermLive is running (informational). Use `doctor` when the user reports a problem or suspects something is broken (diagnostic). When in doubt and the user describes a symptom (e.g., "没反应了", "挂了"), prefer `doctor`.

Extract optional numeric argument for `logs` (default 50).

## Runtime Detection

Before executing any subcommand, detect which environment you are running in:

1. **Claude Code** — `AskUserQuestion` tool is available. Use it for interactive setup wizards.
2. **Other** — `AskUserQuestion` is NOT available. Fall back to non-interactive guidance: explain the steps, show `SKILL_DIR/config.env.example`, and ask the user to create `~/.termlive/config.env` manually.

## Config Check (all commands except `setup`)

Before running any subcommand other than `setup`, check if `~/.termlive/config.env` exists:

- **If it does NOT exist:**
  - In Claude Code: tell the user "No configuration found" and automatically start the `setup` wizard using AskUserQuestion.
  - In other environments: tell the user "No configuration found. Please create `~/.termlive/config.env` based on the example:" then show the contents of `SKILL_DIR/config.env.example` and stop.
- **If it exists:** proceed with the requested subcommand.

## Prerequisites Check

Before `setup`, verify dependencies:

```bash
# Check Node.js
node -v  # Must be >= 22

# Check Go Core binary (will be built/downloaded during setup if missing)
ls ~/.termlive/bin/tlive-core 2>/dev/null || echo "Go Core not found — will install during setup"
```

## Subcommands

### `setup`

Run an interactive setup wizard. Requires `AskUserQuestion`. If not available, show `SKILL_DIR/config.env.example` with field explanations and instruct the user to create the file manually.

When AskUserQuestion IS available, collect input **one field at a time**. After each answer, confirm the value back (masking secrets to last 4 chars).

**Step 1 — Install Go Core binary**

Check if `~/.termlive/bin/tlive-core` exists. If not:

```bash
# Build from source (if in the TermLive repo)
cd SKILL_DIR/../../core && go build -o ~/.termlive/bin/tlive-core ./cmd/tlive-core/
```

Or download from GitHub Releases:
```bash
node SKILL_DIR/../../scripts/postinstall.js
```

**Step 2 — Install Bridge dependencies**

```bash
cd SKILL_DIR/../../bridge && npm install && npm run build
```

**Step 3 — Choose IM platforms**

Ask which channels to enable (telegram, discord, feishu). Accept comma-separated input. Briefly describe each:
- **Telegram** — Best for personal use. Streaming preview, inline permission buttons.
- **Discord** — Good for team use. Server/channel-level access control.
- **Feishu** (飞书/Lark) — For Feishu teams. Streaming cards, tool progress, inline buttons.

**Step 4 — Collect credentials per platform**

For each enabled channel, collect one credential at a time:

- **Telegram**: Bot Token (from @BotFather) → confirm (masked) → Chat ID (optional) → Allowed User IDs (optional). **Important:** At least one of Chat ID or Allowed User IDs should be set.
- **Discord**: Bot Token → confirm (masked) → Allowed User IDs → Allowed Channel IDs (optional). **Important:** At least one of Allowed User IDs or Allowed Channel IDs should be set.
- **Feishu**: App ID → confirm → App Secret → confirm (masked) → Allowed User IDs (optional).

**Step 5 — General settings**

- **Public URL**: For web monitoring links in IM messages (e.g., `https://termlive.example.com`). Optional.
- **Port**: Default 8080.
- **Token**: Auto-generate 32-char hex.
- **Runtime**: `claude` (default), `codex`, `auto`.

**Step 6 — Write config and verify**

1. Show final summary table (secrets masked)
2. Ask user to confirm
3. Create directories: `mkdir -p ~/.termlive/{bin,data,logs,runtime}`
4. Write `~/.termlive/config.env` with all settings
5. Set permissions: `chmod 600 ~/.termlive/config.env`
6. Copy status line script: `cp SKILL_DIR/../../scripts/statusline.sh ~/.termlive/bin/`
7. Report success: "Setup complete! Run `/termlive start` to start the service."

### `start`

**Pre-check:** Verify `~/.termlive/config.env` exists (see "Config check" above).

```bash
bash "SKILL_DIR/../../scripts/daemon.sh" start
```

Show output to user. If it fails, suggest:
- Run `doctor` to diagnose: `/termlive doctor`
- Check recent logs: `/termlive logs`

### `stop`

```bash
bash "SKILL_DIR/../../scripts/daemon.sh" stop
```

### `status`

```bash
bash "SKILL_DIR/../../scripts/daemon.sh" status
```

Also query the API if the daemon is running:
```bash
source ~/.termlive/config.env 2>/dev/null
curl -sf "http://localhost:${TL_PORT:-8080}/api/status" \
  -H "Authorization: Bearer ${TL_TOKEN}" 2>/dev/null | python3 -m json.tool
```

### `logs`

Extract optional line count N from arguments (default 50).

```bash
bash "SKILL_DIR/../../scripts/daemon.sh" logs N
```

### `reconfigure`

1. Read current config from `~/.termlive/config.env`
2. Show current settings in a table (secrets masked to last 4 chars)
3. Use AskUserQuestion to ask what to change
4. Update the config file
5. Remind user: "Run `/termlive stop` then `/termlive start` to apply changes."

### `doctor`

```bash
bash "SKILL_DIR/../../scripts/doctor.sh"
```

Show results and suggest fixes:
- Go Core binary missing → `cd SKILL_DIR/../../core && go build -o ~/.termlive/bin/tlive-core ./cmd/tlive-core/`
- Bridge not built → `cd SKILL_DIR/../../bridge && npm install && npm run build`
- Config missing → run `setup`
- Node.js too old → install Node.js >= 22

## Notes

- Always mask secrets in output (show only last 4 characters).
- Always check for config.env before starting — without it the daemon will fail.
- The Go Core daemon runs as a background process managed by `scripts/daemon.sh`.
- The Node.js Bridge runs as a separate background process.
- Config persists at `~/.termlive/config.env` — survives across sessions.
- Status line script at `~/.termlive/bin/statusline.sh` can be configured in Claude Code settings.
