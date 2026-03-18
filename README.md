# TermLive

AI coding tool remote control platform — monitor terminal sessions, approve tool permissions, and interact with Claude Code from your phone via Telegram, Discord, or Feishu.

## What is TermLive?

TermLive bridges your AI coding sessions to instant messaging platforms. Run Claude Code on your server, get real-time streaming responses on your phone, approve tool permissions with one tap, and view live terminal output in the browser.

**Without TermLive:** You stare at a terminal waiting for Claude Code to finish.

**With TermLive:** You walk away. Your phone buzzes when Claude needs approval or finishes work.

## Features

- **Bidirectional IM** — chat with Claude Code from Telegram, Discord, or Feishu
- **Tool Permission Approval** — approve/deny file edits, command execution from IM buttons
- **Web Terminal** — real-time terminal access from any browser, including mobile
- **Multi-session Dashboard** — manage multiple terminal sessions in one view
- **Streaming Responses** — see Claude's output as it types, not after it finishes
- **Status Line** — Claude Code bottom bar showing sessions, costs, and IM status
- **Claude Code Skill** — `/termlive setup` to get started, `/termlive start` to run
- **Cross-platform** — Linux, macOS, Windows
- **Docker Ready** — `docker compose up` for one-click deployment

## Prerequisites

- **Node.js >= 22**
- **Go 1.24+** (for building Go Core from source, or download prebuilt binary)
- **Claude Code** (recommended) or Codex CLI

## Installation

### npx skills (recommended)

```bash
npx skills add termlive/termlive
```

### Git clone

```bash
git clone https://github.com/termlive/termlive.git ~/.claude/skills/termlive
cd ~/.claude/skills/termlive/bridge && npm install && npm run build
```

Clones the repo directly into your personal skills directory. Claude Code discovers it automatically.

### Symlink (for development)

```bash
git clone https://github.com/termlive/termlive.git ~/code/termlive
mkdir -p ~/.claude/skills
ln -s ~/code/termlive ~/.claude/skills/termlive
cd ~/code/termlive/bridge && npm install && npm run build
```

### Verify installation

Start a new Claude Code session and type `/` — you should see `termlive` in the skill list. Or ask Claude: "What skills are available?"

## Quick Start

### 1. Setup

```
/termlive setup
```

The wizard guides you through:
1. **Build/install Go Core** binary
2. **Build Bridge** (npm install + build)
3. **Choose IM platforms** — Telegram, Discord, Feishu
4. **Enter credentials** — one field at a time, with where-to-find instructions
5. **General settings** — public URL, port, runtime
6. **Validate and write config**

### 2. Start

```
/termlive start
```

Starts Go Core + Node.js Bridge in the background. You can close the terminal — they keep running.

### 3. Chat

Open your IM app and send a message to your bot. Claude Code will respond. When Claude needs to use a tool, you'll see permission buttons right in the chat.

### Docker (alternative)

```bash
git clone https://github.com/termlive/termlive.git
cd termlive
cp .env.example .env    # Fill in your tokens
docker compose up -d
```

## How It Works

```
┌─────────────────────────────────────────────────────────┐
│                    Claude Code CLI                       │
│  ┌──────────┐  ┌──────────────┐                         │
│  │ SKILL.md │  │ Status Line  │                         │
│  │ /termlive│  │ statusline.sh│                         │
│  └────┬─────┘  └──────┬───────┘                         │
└───────┼───────────────┼─────────────────────────────────┘
        │               │
        ▼               ▼
┌─────────────────────────────────────────────────────────┐
│              Node.js Bridge (IM + AI)                    │
│                                                          │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐ │
│  │ Claude SDK  │  │  Telegram    │  │  Permission    │ │
│  │ Provider    │  │  Discord     │  │  Gateway       │ │
│  │             │  │  Feishu      │  │                │ │
│  └─────────────┘  └──────────────┘  └────────────────┘ │
│  ┌─────────────────────────────────────────────────┐    │
│  │           Core Client (HTTP/WebSocket)           │    │
│  └──────────────────────┬──────────────────────────┘    │
└─────────────────────────┼───────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│              Go Core (tlive-core)                         │
│                                                          │
│  ┌──────────┐  ┌───────────┐  ┌──────────┐  ┌───────┐ │
│  │ PTY Mgr  │  │ Session   │  │ Web UI   │  │ Stats │ │
│  │          │  │ Manager   │  │ Dashboard│  │       │ │
│  └──────────┘  └───────────┘  └──────────┘  └───────┘ │
│  ┌──────────────────────────────────────────────────┐   │
│  │              HTTP API + WebSocket                 │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
        │                               │
        ▼                               ▼
   Terminal PTY                    Browser (xterm.js)
```

**Two components:**

- **Go Core** (`tlive-core`) — PTY management, Web UI, HTTP API, WebSocket streaming. Pure infrastructure, no AI logic.
- **Node.js Bridge** — Claude Agent SDK, IM platform adapters, permission approval, message delivery. The intelligence layer.

They communicate via HTTP API + WebSocket. Go Core runs standalone for terminal monitoring; add Bridge for AI + IM features.

## IM Interaction

Send messages to Claude Code from your phone. Get streaming responses. Approve tool permissions with buttons.

```
You (Telegram):  "Fix the login bug in auth.ts"
                         │
Claude Code:     Analyzes code, finds the issue...
                         │
TermLive (TG):   🔧 Claude is editing src/auth.ts
                 Streaming: "I found the issue. The token
                 validation was missing the expiry check..."
                         │
TermLive (TG):   🔒 Permission Required
                 Tool: Edit
                 File: src/auth.ts, lines 42-58
                 [Allow] [Allow Session] [Deny]
                 🖥 View Terminal ↗
                         │
You:             Tap [Allow]
                         │
TermLive (TG):   ✅ Task Complete
                 Fixed auth.ts, all tests pass
                 📊 12.3k/8.1k tok | $0.08 | 2m 34s
                 📋 Dashboard ↗  🖥 Terminal ↗
```

### Platform Support

| Feature | Telegram | Discord | Feishu |
|---------|----------|---------|--------|
| Streaming | Edit-based, 700ms | Edit-based, 1500ms | CardKit v2, 200ms |
| Permission buttons | Inline keyboard | Button components | Interactive card |
| Image support | Yes | Yes | Yes |
| Character limit | 4096/chunk | 2000/chunk | 30000/chunk |

## Configuration

All configuration via `~/.termlive/config.env`:

```env
# Core
TL_PORT=8080
TL_TOKEN=auto-generated-during-setup
TL_PUBLIC_URL=https://termlive.example.com

# IM Platforms (configure one or more)
TL_ENABLED_CHANNELS=telegram,discord,feishu

# Telegram
TL_TG_BOT_TOKEN=your-bot-token
TL_TG_ALLOWED_USERS=123456789

# Discord
TL_DC_BOT_TOKEN=your-bot-token
TL_DC_ALLOWED_CHANNELS=channel-id

# Feishu
TL_FS_APP_ID=your-app-id
TL_FS_APP_SECRET=your-app-secret
```

See `.env.example` for the full list.

## Claude Code Skill Commands

| Command | Function |
|---------|----------|
| `/termlive setup` | Interactive configuration wizard |
| `/termlive start` | Start Go Core + Node.js Bridge |
| `/termlive stop` | Stop all services |
| `/termlive status` | Show service status and connections |
| `/termlive logs [N]` | View last N log lines |
| `/termlive doctor` | Run diagnostic checks |
| `/termlive reconfigure` | Change IM platform settings |

Supports English and Chinese: `启动`, `停止`, `状态`, `诊断` all work.

## Status Line

TermLive adds a status bar to Claude Code's bottom:

```
TL: 2sess | bridge:on | 12.3k/8.1k tok | $0.08
```

And a real-time status bar to the Web UI dashboard:

```
● 3 sessions │ TG ● DC ● FS ● │ 12.3k/8.1k tok │ $0.08 │ 2m 34s
```

## Development

### Prerequisites

- Go 1.24+ (for Core)
- Node.js 22+ (for Bridge)

### Build

```bash
# Go Core
cd core && go build -o tlive-core ./cmd/tlive-core/

# Node.js Bridge
cd bridge && npm install && npm run build
```

### Test

```bash
# Go tests
cd core && go test ./... -v -timeout 30s

# Bridge tests (122 tests)
cd bridge && npm test
```

### Project Structure

```
termlive/
├── core/                    # Go Core → tlive-core binary
│   ├── cmd/tlive-core/      # CLI entry point
│   ├── internal/
│   │   ├── daemon/          # HTTP API, session manager, bridge, stats, tokens
│   │   ├── server/          # WebSocket handlers, status stream
│   │   ├── session/         # Session state and output buffer
│   │   ├── hub/             # Broadcast hub (fan-out to clients)
│   │   ├── pty/             # PTY abstraction (Unix + Windows ConPTY)
│   │   ├── config/          # TOML configuration
│   │   └── notify/          # ANSI utilities
│   └── web/                 # Embedded Web UI (dashboard + terminal + status bar)
│
├── bridge/                  # Node.js Bridge
│   └── src/
│       ├── providers/       # Claude Agent SDK + CLI fallback
│       ├── channels/        # Telegram, Discord, Feishu adapters
│       ├── engine/          # Conversation engine, router, bridge manager
│       ├── permissions/     # Permission gateway + broker
│       ├── delivery/        # Message chunking, retry, rate limiting
│       ├── markdown/        # IR → per-platform rendering
│       └── store/           # JSON file persistence
│
├── skill/                   # Claude Code skill definition
│   └── SKILL.md
│
├── scripts/                 # CLI, daemon, diagnostics, status line
├── docker-compose.yml       # One-click Docker deployment
└── package.json             # npm: termlive
```

## Security

- **Bearer token auth** for all API endpoints (auto-generated during setup)
- **Scoped tokens** for IM web links (1-hour TTL, read-only, session-specific)
- **IM user whitelist** per platform (`TL_TG_ALLOWED_USERS`, etc.)
- **Secret redaction** in all log output
- **Config file permissions** `chmod 600` on `config.env`

## License

MIT
