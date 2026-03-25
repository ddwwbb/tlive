# tlive

[![npm version](https://img.shields.io/npm/v/tlive)](https://www.npmjs.com/package/tlive)
[![CI](https://github.com/y49/tlive/actions/workflows/ci.yml/badge.svg)](https://github.com/y49/tlive/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

[English](README.md)

**Terminal Live** — 从 Telegram、Discord、飞书监控和操控 AI 编码工具（Claude Code、Codex）。

三大功能，按需组合：

| 功能 | 说明 | 访问方式 |
|------|------|---------|
| **Web 终端** | `tlive <cmd>` — 包装任何命令，手机浏览器访问 | 浏览器 / 手机 |
| **IM 桥接** | `/tlive` — 在手机上与 Claude Code 双向对话 | Telegram / Discord / 飞书 |
| **Hook 审批** | 在手机上审批 Claude Code 工具权限 | Telegram / Discord / 飞书 |

<!-- TODO: 添加演示截图/GIF -->
<!-- ![tlive 演示](docs/images/demo.gif) -->

## 快速开始

```bash
# 1. 安装
npm install -g tlive

# 2. 配置 IM 平台（交互式引导）
tlive setup

# 3. 注册 hooks + Claude Code 技能
tlive install skills

# 4. 在 Claude Code 中启动桥接
/tlive
```

> **推荐：** 在 Claude Code 中运行 `/tlive setup`，AI 会一步步引导你完成配置。

平台配置指南：[Telegram](docs/setup-telegram-cn.md) · [Discord](docs/setup-discord-cn.md) · [飞书](docs/setup-feishu-cn.md) · [完整入门指南](docs/getting-started-cn.md)

## Web 终端

包装长时间运行的命令，手机浏览器远程访问。

```bash
tlive claude                  # 包装 Claude Code
tlive python train.py         # 包装训练脚本
tlive npm run build           # 包装构建
```

```
$ tlive claude --model opus

  TLive Web UI:
    Local:   http://localhost:8080?token=abc123
    Network: http://192.168.1.100:8080?token=abc123
  Session: claude (ID: a1b2c3)
```

多会话共享仪表盘。Daemon 自动启动，空闲 15 分钟自动退出。

<!-- TODO: 添加 Web 终端截图 -->
<!-- ![Web 终端](docs/images/web-terminal.png) -->

## IM 桥接

手机上与 Claude Code 对话。发起新任务，获取实时流式响应和工具可视化。

```
你 (Telegram):    "修复 auth.ts 里的登录 bug"

TLive (TG):       🔍 Grep → 📖 Read → ✏️ Edit → 🖥️ Bash
                   ──────────────────
                   我发现了问题。
                   Token 验证缺少过期检查...

TLive (TG):       ✅ 任务完成
                   已修复 auth.ts，测试通过
                   📊 12.3k/8.1k tok | $0.08 | 2m 34s
```

**详细度控制：** `/verbose 0|1|2` — 安静（仅最终回复）/ 正常（工具名称）/ 详细（工具名称 + 输入参数）。

<!-- TODO: 添加 IM 桥接截图 -->
<!-- ![IM 桥接](docs/images/im-bridge.png) -->

## Hook 审批

在手机上审批 Claude Code 工具权限。再也不会被 `[y/N]` 提示卡住。

```
你在终端正常运行 Claude Code（不需要任何包装）
  │
  ├── Claude 想执行一个命令
  │   → Hook 触发 → Go Core 接收 → Bridge 发送到手机：
  │
  │   🔒 需要权限
  │   工具: Bash
  │   ┌──────────────────────────┐
  │   │ rm -rf node_modules &&   │
  │   │ npm install              │
  │   └──────────────────────────┘
  │   [✅ 允许]  [❌ 拒绝]
  │
  ├── 你点 [允许] → Claude Code 继续执行
  │
  └── 离开电脑。Claude 继续工作。
      只有需要审批时手机才会响。
```

**安全设计：**
- 超时默认**拒绝**（不是允许）
- 审批前显示具体工具名和命令内容
- Hook 脚本先检查 Go Core 是否运行 — 没运行则直接放行（零影响）
- 适用于任何 Claude Code 会话，不需要包装器

**坐在电脑旁时暂停：**

```bash
tlive hooks pause              # 自动放行
tlive hooks resume             # 恢复 IM 审批
```

<!-- TODO: 添加 Hook 审批截图 -->
<!-- ![Hook 审批](docs/images/hook-approval.png) -->

## 支持平台

| | Telegram | Discord | 飞书 |
|---|----------|---------|------|
| IM 桥接 | ✅ | ✅ | ✅ |
| Hook 审批 | ✅ | ✅ | ✅ |
| 流式响应 | 编辑消息 | 编辑消息 | CardKit v2 |
| 工具可视化 | ✅ | ✅ | ✅ |
| 输入状态 | ✅ | ✅ | — |
| 权限按钮 | 内联键盘 | Button 组件 | 互动卡片 |

## 命令

### CLI

```bash
tlive <cmd>                # Web 终端
tlive setup                # 配置 IM 平台
tlive install skills       # 注册 hooks + Claude Code 技能
tlive start                # 启动 Bridge 守护进程
tlive stop                 # 停止守护进程
tlive status               # 查看状态
tlive logs [N]             # 查看最近 N 行日志
tlive doctor               # 运行诊断
tlive hooks                # 查看 Hook 状态
tlive hooks pause          # 暂停 Hook（自动放行）
tlive hooks resume         # 恢复 Hook（IM 审批）
```

### Claude Code 技能

```
/tlive                     # 启动 IM Bridge
/tlive setup               # AI 引导配置
/tlive stop                # 停止 Bridge
/tlive status              # 查看状态
/tlive doctor              # 诊断

/verbose 0|1|2             # 设置详细度
/new                       # 开始新对话
/hooks pause|resume        # 切换 Hook 审批
```

## 配置

统一配置文件 `~/.tlive/config.env`（由 `tlive setup` 创建）：

```env
TL_PORT=8080
TL_TOKEN=自动生成
TL_HOST=127.0.0.1
TL_PUBLIC_URL=https://example.com

TL_ENABLED_CHANNELS=telegram,discord
TL_TG_BOT_TOKEN=...
TL_TG_CHAT_ID=...
TL_DC_BOT_TOKEN=...
TL_FS_APP_ID=...
TL_FS_APP_SECRET=...
```

完整配置项参见 [config.env.example](config.env.example)。

## 架构

```
                    ┌──────────────────────┐
                    │   Claude Code (本地)  │
                    │                      │
                    │  PreToolUse Hook ────────────┐
                    │  Notification Hook ──────────┤
                    └──────────────────────┘       │
                                                   ▼
┌─ Go Core (tlive) ───────────────────────────────────────────┐
│                                                              │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────┐│
│  │ PTY 管理  │  │ Web UI      │  │ Hook 管理器            ││
│  │ (包装    │  │ (仪表盘 +   │  │ (接收 hooks,           ││
│  │  命令)   │  │  xterm.js)   │  │  长轮询, 解析)         ││
│  └──────────┘  └──────────────┘  └────────────────────────┘│
│                                                              │
│  HTTP API + WebSocket                                        │
└──────────────────────────┬───────────────────────────────────┘
                           │ Bridge 轮询 /api/hooks/pending
                           ▼
┌─ Node.js Bridge ────────────────────────────────────────────┐
│                                                              │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────┐ │
│  │ Agent SDK   │  │ Telegram     │  │ Hook 轮询          │ │
│  │ (IM 发起   │  │ Discord      │  │ (转发到 IM,        │ │
│  │  新任务)    │  │ 飞书         │  │  点击后解析)        │ │
│  └─────────────┘  └──────────────┘  └────────────────────┘ │
└──────────────────────────────────────────────────────────────┘
                           │
                           ▼
                    ┌──────────────┐
                    │  你的手机     │
                    │  (IM 应用)   │
                    └──────────────┘
```

## 开发

```bash
# Go Core
cd core && go build -o tlive ./cmd/tlive/ && go test ./...

# Bridge
cd bridge && npm install && npm run build && npm test
```

### 项目结构

```
tlive/
├── SKILL.md                # Claude Code / Codex 技能
├── config.env.example
├── core/                   # Go → tlive 二进制
│   ├── cmd/tlive/          # CLI（Web 终端、停止、配置、安装）
│   ├── internal/
│   │   ├── daemon/         # HTTP 服务器、会话、Hook 管理器
│   │   ├── server/         # WebSocket 处理
│   │   ├── session/        # 会话状态 + 输出缓冲
│   │   ├── hub/            # 广播中心
│   │   └── pty/            # PTY（Unix + Windows ConPTY）
│   └── web/                # 内嵌 Web UI
├── bridge/                 # Node.js → Bridge 守护进程
│   └── src/
│       ├── providers/      # Claude Agent SDK
│       ├── channels/       # Telegram、Discord、飞书适配器
│       ├── engine/         # 对话引擎、Bridge 管理器、流式控制
│       ├── permissions/    # 权限网关 + 代理
│       ├── delivery/       # 分块、重试、限速
│       └── markdown/       # 各平台渲染
├── scripts/
│   ├── hook-handler.sh     # PreToolUse hook → Go Core
│   ├── notify-handler.sh   # Notification hook → Go Core
│   ├── daemon.sh           # Bridge 进程管理
│   └── statusline.sh       # Claude Code 状态行
├── package.json            # npm: tlive
└── docker-compose.yml
```

## 安全

- 默认绑定 `127.0.0.1`（需要局域网访问显式设置 `--host 0.0.0.0`）
- 自动生成认证 token
- Hook 超时默认**拒绝**（不是允许）
- IM 用户白名单
- 日志自动脱敏
- 配置文件 `chmod 600`
- Claude CLI 子进程环境隔离

## 许可证

MIT
