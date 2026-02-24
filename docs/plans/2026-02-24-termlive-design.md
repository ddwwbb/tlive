# TermLive 设计文档

## 概述

TermLive 是一个终端实时监控与远程交互工具。通过命令包装器模式（`tl <command>`）捕获终端 I/O，提供 Web UI 实时查看和远程交互，并通过微信/飞书 Bot 推送空闲通知。

**核心场景：** AI 终端工具（Claude Code 等）运行时间长、需要手动确认，用户希望离开电脑后仍能通过手机监控并交互。

## 技术选型

- **语言：** Go（单二进制部署，goroutine 并发模型适合流式数据广播）
- **PTY：** `creack/pty`
- **WebSocket：** `gorilla/websocket`
- **前端：** xterm.js（纯 HTML/CSS/JS，内嵌到 Go 二进制）
- **CLI：** `spf13/cobra`
- **配置：** TOML（`go-toml/v2`）

## 架构

```
┌─────────────────────────────────────────────────┐
│                    tl (Go Binary)                │
│                                                  │
│  ┌──────────┐   ┌───────────┐   ┌────────────┐  │
│  │ PTY      │──▶│ Hub       │──▶│ WebSocket  │──▶ 浏览器 (xterm.js)
│  │ Manager  │   │ (广播中心) │   │ Server     │  │
│  │          │◀──│           │◀──│            │◀── 用户输入
│  └──────────┘   └─────┬─────┘   └────────────┘  │
│       │               │         ┌────────────┐  │
│       │               ├────────▶│ HTTP       │  │
│       ▼               │         │ Server     │──▶ Web UI 静态文件
│  本地终端输出          │         └────────────┘  │
│                       │         ┌────────────┐  │
│                       └────────▶│ Notifier   │  │
│                                 │ (Bot通知)   │──▶ 微信/飞书
│                                 └────────────┘  │
└─────────────────────────────────────────────────┘
```

### 组件职责

| 组件 | 职责 |
|------|------|
| PTY Manager | 创建伪终端，启动子命令，读写 PTY 数据流 |
| Hub | 中央广播器。PTY 输出 → 所有 WebSocket 客户端；WebSocket 输入 → PTY |
| WebSocket Server | 与浏览器建立 WS 连接，双向传输终端数据 |
| HTTP Server | 提供 Web UI 静态文件（embed.FS 内嵌） |
| Notifier | 监测 PTY 空闲超时，通过微信/飞书 Webhook 发送通知 |

### 数据流

1. `tl claude` → 创建 PTY → 在 PTY 中启动 `claude`
2. PTY 输出 → 同时写入本地终端 + 广播到所有 WebSocket 客户端
3. WebSocket 客户端键盘输入（包括方向键、Tab 等） → 写入 PTY stdin
4. PTY 空闲超时（无新输出超过 N 秒） → Notifier 发送 Bot 通知

## CLI 接口

```bash
# 基本用法
tl <command> [args...]
tl claude
tl -- npm run dev

# 选项
tl -p 3000 claude          # 指定端口（默认 8080）
tl -t 60 claude            # 空闲通知超时秒数（默认 30）

# 管理
tl ls                      # 列出活跃会话
tl config                  # 查看配置
tl attach <session-id>     # 连接已有会话的 Web
```

### 多会话管理

- 每次 `tl <cmd>` 创建一个 Session（唯一 ID、PTY 实例、输出 buffer）
- 第一个 `tl` 进程启动 Web 服务并作为 daemon
- 后续 `tl` 进程通过 Unix socket / named pipe 注册新会话到 daemon
- Web UI 首页展示所有活跃会话列表

### 配置文件

路径：`~/.termlive/config.toml`

```toml
[server]
port = 8080
host = "0.0.0.0"

[notify]
idle_timeout = 30

[notify.wechat]
webhook_url = "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"

[notify.feishu]
webhook_url = "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
```

## Web UI

### 会话列表页

- 展示所有活跃会话卡片
- 每张卡片显示：命令名、PID、运行时长、最近输出预览
- 点击卡片进入终端视图

### 终端视图

- xterm.js 全屏渲染，完整终端交互能力
- 所有按键（方向键、Tab、Ctrl+组合键）通过 WebSocket 发送到 PTY
- 移动端响应式布局，点击终端区域弹出软键盘
- 启动时显示局域网 IP + 二维码，手机扫码直达

### 前端技术

- 纯 HTML/CSS/JS，无框架依赖
- 通过 Go `embed.FS` 内嵌到二进制
- xterm.js + xterm-addon-fit（终端渲染和自适应尺寸）

## Bot 集成

### MVP 策略

第一版 Bot 只做**单向 Webhook 通知**，双向交互走 Web UI。避免申请企业应用的复杂流程。

### 通知触发

空闲检测器：PTY 无新输出超过配置的超时时间后发送一次通知，避免重复通知。

```go
type IdleDetector struct {
    timeout    time.Duration
    lastOutput time.Time
    notified   bool
}
```

- PTY 每次有输出时重置计时器和 notified 标记
- 超时后发送通知并标记 notified = true
- 下次 PTY 有输出后重新开始检测周期

### 通知内容

```
[TermLive 通知]
🔔 终端等待输入 (空闲 30s)

会话: claude (PID: 12345)
运行时长: 15m 32s

最近输出:
> ? Do you want to proceed? [Y/n]

👉 打开 Web 终端: http://192.168.1.5:8080/s/abc123
```

### 后续扩展（非 MVP）

- 企业微信应用 + 飞书应用：支持 Bot 双向消息
- 智能解析终端选择菜单，生成快捷回复按钮
- 特殊指令（`/up` `/down` `/enter`）映射为按键

## 安全

- 默认监听 `0.0.0.0`（局域网），可配置 `127.0.0.1`
- 启动时生成随机 token，URL 带 token 参数认证
- WebSocket 连接验证 token

## 项目结构

```
termlive/
├── cmd/
│   └── tl/
│       └── main.go              # CLI 入口
├── internal/
│   ├── pty/
│   │   └── manager.go           # PTY 创建与管理
│   ├── session/
│   │   └── session.go           # 会话生命周期
│   ├── hub/
│   │   └── hub.go               # 广播中心
│   ├── server/
│   │   ├── server.go            # HTTP + WebSocket
│   │   └── handler.go           # 路由处理
│   ├── notify/
│   │   ├── notifier.go          # 通知接口
│   │   ├── wechat.go            # 企业微信 Webhook
│   │   ├── feishu.go            # 飞书 Webhook
│   │   └── idle.go              # 空闲检测
│   └── config/
│       └── config.go            # 配置加载
├── web/
│   ├── index.html               # 会话列表页
│   ├── terminal.html            # 终端页
│   ├── css/style.css
│   └── js/
│       ├── app.js
│       └── vendor/              # xterm.js 等
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 核心依赖

| 库 | 用途 |
|----|------|
| `github.com/creack/pty` | PTY 创建（Unix） |
| `github.com/gorilla/websocket` | WebSocket |
| `github.com/spf13/cobra` | CLI 参数解析 |
| `github.com/pelletier/go-toml/v2` | TOML 配置 |

## 关键技术点

### PTY 数据流

```go
go func() {
    buf := make([]byte, 4096)
    for {
        n, err := ptmx.Read(buf)
        if err != nil { break }
        data := buf[:n]
        os.Stdout.Write(data)   // 本地终端
        hub.Broadcast(data)     // WebSocket
        idle.Reset()            // 重置空闲计时
    }
}()

hub.OnInput(func(data []byte) {
    ptmx.Write(data)
})
```

### 终端尺寸同步

- Web 客户端发送 resize 事件 → 服务端调用 `pty.Setsize()`
- 多客户端时取第一个连接的尺寸或最小公共尺寸
