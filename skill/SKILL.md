---
name: termlive
description: >
  Terminal live monitoring + IM bridge for AI coding tools.
  Monitor terminal sessions remotely via web browser, approve tool permissions
  from Telegram/Discord/Feishu, view real-time terminal output, and track
  context usage and costs.
argument-hint: "setup | start | stop | status | logs [N] | reconfigure | doctor | notify <msg>"
allowed-tools:
  - Bash
  - Read
  - Write
  - Edit
  - AskUserQuestion
  - Grep
  - Glob
---

# TermLive

Terminal live monitoring + IM bridge for AI coding tools.

## Command Routing

Parse the user's argument against this table. Match the FIRST hit.

| User says (EN) | User says (CN) | Command |
|----------------|----------------|---------|
| `setup`, `configure`, `install`, `帮我连接 Telegram` | `配置`, `安装`, `设置` | → setup |
| `start`, `run`, `launch` | `启动`, `启动服务`, `开始` | → start |
| `stop`, `shutdown`, `kill` | `停止`, `关闭`, `停` | → stop |
| `status`, `info` | `状态`, `运行状态` | → status |
| `logs`, `log`, `logs 50` | `日志`, `看日志` | → logs |
| `reconfigure`, `reconfig` | `重新配置` | → reconfigure |
| `doctor`, `diagnose`, `check`, `debug` | `诊断`, `检查`, `挂了`, `没反应了` | → doctor |
| `notify <msg>` | `通知 <msg>` | → notify |

If no match and input looks like a natural language request, interpret intent.

## Commands

### setup
Interactive configuration wizard.

**Pre-check:** If `~/.termlive/config.env` exists, ask: "Config already exists. Reconfigure? (y/n)"

**Step 1 — Choose IM platforms:**
```
AskUserQuestion: "Which IM platforms do you want to enable? (comma-separated)
1. Telegram
2. Discord
3. Feishu (飞书)
Enter numbers (e.g., 1,3):"
```

**Step 2 — Collect credentials per platform:**

For Telegram:
```
AskUserQuestion: "Enter your Telegram Bot Token (from @BotFather):"
AskUserQuestion: "Enter your Telegram Chat ID (or leave blank for any):"
AskUserQuestion: "Enter allowed user IDs (comma-separated, or blank for all):"
```

For Discord:
```
AskUserQuestion: "Enter your Discord Bot Token:"
AskUserQuestion: "Enter allowed user IDs (comma-separated, or blank for all):"
AskUserQuestion: "Enter allowed channel IDs (comma-separated, or blank for all):"
```

For Feishu:
```
AskUserQuestion: "Enter your Feishu App ID:"
AskUserQuestion: "Enter your Feishu App Secret:"
```

**Step 3 — General settings:**
```
AskUserQuestion: "Enter your public URL for web links (e.g., https://termlive.example.com, or blank to skip):"
```

Auto-generate: TL_TOKEN (32-char hex), TL_PORT (default 8080).

**Step 4 — Write config and verify:**
- Write `~/.termlive/config.env` with chmod 600
- Check Go Core binary exists at `~/.termlive/bin/tlive-core`
- If not, download via postinstall logic
- Send test notification to each configured platform
- Install status line script

### start
```bash
bash scripts/daemon.sh start
```
Report output to user.

### stop
```bash
bash scripts/daemon.sh stop
```

### status
```bash
bash scripts/daemon.sh status
```
Also call `curl -s http://localhost:${TL_PORT}/api/status -H "Authorization: Bearer ${TL_TOKEN}"` and format the JSON response nicely.

### logs
```bash
bash scripts/daemon.sh logs ${N:-50}
```

### reconfigure
Same as `setup` but skip the "Config already exists" check.

### doctor
```bash
bash scripts/doctor.sh
```

### notify
```bash
# Forward to Bridge for delivery to all IM channels
curl -s -X POST "http://localhost:${TL_PORT}/api/notify" \
  -H "Authorization: Bearer ${TL_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"message\": \"${MSG}\"}"
```

## Config Check Gate

All commands EXCEPT `setup` must first verify:
1. `~/.termlive/config.env` exists
2. `TL_TOKEN` is set

If either is missing, say: "TermLive is not configured. Run `/termlive setup` first."
