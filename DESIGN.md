# Send2NLM — 顶层设计文档

> **版本**: v0.6
> **状态**: 设计阶段，待评审
> **变更**: v0.5→v0.6 — NotebookLM 交互层重构：基于 notebooklm-py CLI，直接调用 Google 内部 RPC API，消除浏览器 DOM 自动化

---

## 1. 概述

Send2NLM 是一个 Chrome 浏览器扩展 (MV3)，将用户当前浏览的网页以 PDF 格式发送到 Google NotebookLM，并可自动触发音频概览 / 幻灯片生成。

系统由两部分构成：

| 组件 | 技术栈 | 角色 |
|------|--------|------|
| Chrome 扩展 | JavaScript (MV3) | 用户界面：笔记本选择/创建、触发发送、进度展示 |
| 本地 Daemon | Go (net/http) | 业务编排：URL→PDF 转换、NotebookLM 操作、任务状态跟踪 |

**核心原则**：所有 NotebookLM 操作均通过 [notebooklm-py](https://github.com/teng-lin/notebooklm-py) 的 CLI 完成。该库直接调用 Google 内部 RPC API，**无需浏览器** —— 笔记本管理、源文件上传、任务生成、状态轮询、产物下载全部走 HTTP 调用。

### 1.1 依赖的外部 CLI

| CLI | 用途 |
|-----|------|
| `notebooklm list --json` | 获取笔记本列表 |
| `notebooklm create <title> --use --json` | 创建新笔记本（--use 激活笔记本上下文） |
| `notebooklm source add -n <id> <file> --json` | 上传 PDF 文件到笔记本（-n 在子命令后） |
| `notebooklm generate audio -n <id> <instructions> --json` | 触发音频概览生成 |
| `notebooklm generate slide-deck -n <id> --json` | 触发幻灯片生成 |
| `notebooklm artifact poll -n <id> <taskID> --json` | 轮询生成任务状态（纯 HTTP，无需浏览器） |
| `notebooklm download audio -n <id> <path> --latest --force` | 下载完成的音频文件 |
| `notebooklm download slide-deck -n <id> <path> --latest --force` | 下载完成的幻灯片 |
| `lark-cli drive +export --file-extension pdf` | 飞书文档导出 PDF |

> **注**: Default Producer 使用 `opencli web read` 抓取 Markdown 与配图，再通过 `pandoc` 将 Markdown 转换为 PDF。

---

## 2. 项目目录结构

```
send2nlm/
├── extension/                    # Chrome 扩展 (MV3)
│   ├── manifest.json
│   ├── popup/
│   │   ├── popup.html
│   │   ├── popup.css
│   │   └── popup.js
│   ├── background/
│   │   └── service-worker.js     # MV3 Service Worker
│   ├── shared/
│   │   ├── daemon-client.js      # 与 daemon 通信的封装
│   │   └── i18n.js               # 国际化辅助
│   ├── _locales/
│   │   ├── en/messages.json
│   │   └── zh_CN/messages.json
│   └── icons/
│       ├── icon16.png
│       ├── icon48.png
│       └── icon128.png
│
├── daemon/                       # Go 本地 Daemon
│   ├── go.mod
│   ├── go.sum
│   ├── main.go                   # 入口: CLI 分发 (send2nlm / daemon / stop)
│   ├── cmd/
│   │   ├── daemon.go             # daemon 模式: 启动 HTTP server
│   │   ├── oneshot.go            # one-shot 模式: send 命令直接执行
│   │   └── stop.go               # stop 模式: 发送 shutdown 信号
│   ├── sdk/                      # SDK 接口定义 (daemon 和适配器共享的类型)
│   │   ├── producer.go           # Producer 接口
│   │   └── receiver.go           # Receiver 接口
│   ├── server/
│   │   ├── server.go             # HTTP server 启动、路由、优雅关闭
│   │   ├── handler_notebooks.go  # GET/POST /notebooks
│   │   ├── handler_jobs.go       # POST /jobs, GET /jobs/:id
│   │   └── middleware.go         # CORS、日志、recovery
│   ├── core/
│   │   ├── pipeline.go           # 核心流水线编排
│   │   ├── types.go              # 共享类型定义
│   │   └── config.go             # 配置管理 (路径、端口等)
│   ├── store/
│   │   ├── db.go                 # SQLite 初始化、连接管理
│   │   ├── notebooks.go          # 笔记本缓存 CRUD
│   │   └── jobs.go               # Job 持久化 CRUD
│   ├── scriptmgr/                # 适配器管理器 (子进程编译缓存 + 文件监听)
│   │   ├── external.go           # 外部适配器发现、编译缓存、通信协议
│   │   ├── producer_registry.go  # Producer 注册表 + for-loop 调度
│   │   └── receiver_registry.go  # Receiver 注册表 + 调用链
│   ├── producer/
│   │   └── default.go            # 唯一内置 Producer (编译进 daemon, fallback)
│   ├── nlm/
│   │   ├── client.go             # NotebookLM 操作封装 (调用 notebooklm-py CLI)
│   │   ├── notebook.go           # 笔记本 CRUD
│   │   ├── source.go             # 添加 source
│   │   └── tasks.go              # 创建任务 + 状态轮询
│   ├── receiver/
│   │   └── builtin.go            # 内置 Download Receiver (复制到 ~/Downloads/send2nlm/)
│   ├── converter/
│   │   └── md2pdf.go             # Markdown → PDF 转换 (pandoc + xelatex)
│   └── resources/                # go:embed 内嵌资源 (首次使用时复制到 ~/.send2nlm/)
│       ├── producer/
│       │   └── lark.go           # Lark Producer 扩展适配器
│       └── receiver/
│           └── telegram.go       # Telegram Receiver 扩展适配器
│
├── scripts/                      # 适配器示例 (用户可直接复制到 ~/.send2nlm/)
│   ├── producer/
│   │   └── lark.go               # → ~/.send2nlm/producer/lark.go
│   ├── receiver/
│   │   ├── telegram.go           # → ~/.send2nlm/receiver/telegram.go
│   │   └── example.go            # Receiver 示例模板
│   └── README.md                 # 适配器编写指南
│
├── dev_assets/                   # 开发环境模拟目录 (替代 ~/.send2nlm)
│   ├── producer/                 # 开发用 producer 适配器 (.go)
│   ├── receiver/                 # 开发用 receiver 适配器 (.go)
│   └── send2nlm.db               # 开发用 SQLite 数据库
│
├── DESIGN.md                     # 本文档
└── README.md
```

---

## 3. 架构总览

```
┌──────────────────────────────────────────────────────────────┐
│                   Chrome 扩展 (MV3)                           │
│                                                               │
│  ┌──────────┐   ┌──────────────┐   ┌───────────────────┐    │
│  │ popup.js │──▶│daemon-client │──▶│ service-worker.js  │    │
│  │  (UI)    │   │    .js       │   │  (get tab URL)     │    │
│  └──────────┘   └──────┬───────┘   └───────────────────┘    │
│                         │                                     │
└─────────────────────────┼─────────────────────────────────────┘
                          │ HTTP (localhost:PORT)
                          ▼
┌──────────────────────────────────────────────────────────────┐
│                   Go Daemon (本地进程)                         │
│                                                               │
│  ┌──────────┐   ┌──────────────┐                             │
│  │  Server  │──▶│  Pipeline    │  编排引擎                     │
│  │  (HTTP)  │   │  (状态机)     │                              │
│  └──────────┘   └──────┬───────┘                             │
│                         │                                      │
│         ┌───────────────┼───────────────┐                     │
│         ▼               ▼               ▼                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐              │
│  │  Producer  │  │  NLM       │  │  Receiver  │              │
│  │  URL→PDF   │  │  Client    │  │  投递适配器  │              │
│  └────────────┘  └─────┬──────┘  └────────────┘              │
│         │              │         │                             │
│         │     ┌────────┼────────┐                             │
│         │     │  notebooklm-py  │                             │
│         │     │  CLI            │                             │
│         │     │  (Python)       │                             │
│         │     └────────┘        │                             │
│         │                       │                             │
│  ┌──────┴──────────────────────┴──────┐                       │
│  │           SQLite Store             │                       │
│  │  • notebook cache                  │                       │
│  │  • job state persistence           │                       │
│  └────────────────────────────────────┘                       │
└──────────────────────────────────────────────────────────────┘
```

### 3.1核心数据流

```
用户点击扩展图标
    │
    ▼
[Page 1] 获取笔记本列表
    │  popup → daemon GET /notebooks
    │  daemon → 优先从 SQLite 缓存返回 (首次/过期 → notebooklm list --json → 更新缓存)
    │  daemon → popup: JSON [{title, id, url}]
    │  底部提供 [刷新] 按钮: 触发重新从 NLM 拉取 + 覆盖缓存
    │  底部提供 [+新建笔记本] 入口
    │
    ▼
用户选择笔记本 → 右滑进入 [Page 2]
    │  显示: 目标笔记本名 + URL 预览 + 可选任务 + Send 按钮
    │  左上角 [←] 左滑返回 Page 1
    │
    ▼
用户点击 Send
    │  popup → daemon POST /jobs {notebook_id, url, tasks:["audio_overview","slide_deck"]}
    │  daemon 写入 SQLite job 记录 (status=pending) 并加入 FIFO 队列
    │  返回 {job_id, status:"accepted"}
    │
    ▼
Pipeline 异步执行 (阶段进入与产物生成后立即更新 SQLite):
    │
    ├─[Step 1] PRODUCING: URL → PDF
    │     1. 遍历 Producer 适配器链: for _, p := range producers { if p.Match(url) { pdf, err = p.Produce(url) } }
    │     2. 命中 → 使用适配器结果; 全 miss → fallback 到 Default Producer
    │     3. Default Producer: HTTP 抓取 → MD → 内置 MD→PDF 转换器 → PDF 文件
    │     4. 进入阶段时更新 job.status=producing；完成后写入 pdf_path
    │
    ├─[Step 2] UPLOADING: 上传 PDF 到 NotebookLM
    │     1. notebooklm -n <notebook_id> source add <pdf_path> --json
    │     2. 解析返回的 source_id
    │     3. 进入阶段时更新 job.status=uploading；完成后写入 source_id
    │
    ├─[Step 3] TASKING: 创建任务 (根据用户选择)
    │     1. if "audio_overview" in tasks:
    │          notebooklm -n <id> generate audio "..." --json
    │          记录 task_id, 初始 status
    │     2. if "slide_deck" in tasks:
    │          notebooklm -n <id> generate slide-deck --json
    │          记录 task_id, 初始 status
    │     3. 进入阶段时更新 job.status=tasking；完成后写入 task_results={...}
    │
    ├─[Step 4] POLLING: 先等待 10min，再轮询任务状态 (每 1min, 总上限 60min)
    │     loop {
    │       对于每个未完成的任务:
    │         notebooklm -n <id> artifact poll <taskID> --json （纯 HTTP，无需浏览器）
    │       更新 SQLite 中的 task_results
    │       if all completed: break
    │       if elapsed > 60min: 最后查询一次；仍未完成则标记超时
    │       sleep(1min)
    │     }
    │     进入阶段时更新 job.status=polling；全部完成后进入 DOWNLOADING
    │
    ├─[Step 5] DOWNLOADING: 下载生成资源
    │     1. notebooklm -n <id> download audio <path> --latest --force （纯 HTTP，无需浏览器）
    │     2. notebooklm -n <id> download slide-deck <path> --latest --force
    │     3. asset_path 写入 task_results.*.asset_path
    │     4. 更新 job.status=downloading；完成后进入 RECEIVING
    │
    └─[Step 6] RECEIVING: 资源投递 (可选，后续扩展)
          调用 Receiver 链发送本地资源；完成后 job.status=done
```

### 3.2 关键设计决策

| 决策 | 理由 |
|------|------|
| 使用 notebooklm-py CLI 而非浏览器自动化 | notebooklm-py 直接调用 Google 内部 RPC API。笔记本管理、生成、轮询、下载全部走 HTTP，无需启动或维持浏览器，稳定性和速度大幅提升 |
| PDF 作为上传格式 | 0.0.1 严格保证"上传笔记本的内容始终是 PDF"，不提供 URL/content 降级路径 |
| 通用 PDF 方案: HTTP 抓取 → MD → PDF | 比 browser print 更可控，跨平台无依赖系统打印对话框 |
| SQLite 持久化 notebook 缓存 + job 状态 | 重启后恢复未完成任务；笔记本列表秒开无需每次查 NLM |
| 异步 pipeline 而非同步等待 | Daemon 模式下多 job 可排队；one-shot 模式同步阻塞 |

---

## 4. Chrome 扩展 ↔ Daemon 通信协议

### 4.1 端口发现

Daemon 启动时将监听端口写入文件：

- **配置文件**: `~/.send2nlm/daemon.port` (纯文本，单个端口号)
- 扩展无法直接读文件系统，采用以下策略：
  1. 优先尝试固定默认端口 `18923`
  2. 若默认端口连接失败，再尝试 `chrome.storage.local` 中上次保存的自定义端口
  3. 扩展首次成功连接后，将端口写入 `chrome.storage.local`
- `manifest.json` 中声明 `"host_permissions": ["http://localhost:*/*"]`

### 4.1.1 0.0.1 本地访问边界

0.0.1 暂不实现鉴权 token、请求签名或 Native Messaging。Daemon 仅监听 `127.0.0.1`，并通过 CORS middleware 限制浏览器扩展访问；更完整的本地权限模型留到后续版本。

### 4.2 API 端点

#### `GET /health`

健康检查。

**响应** (200):
```json
{ "status": "ok", "version": "0.1.0" }
```

#### `GET /notebooks`

获取笔记本列表。优先从 SQLite 缓存返回。

**Query params**:
- `refresh=true` — 跳过缓存，重新从 NotebookLM 拉取并更新数据库

**响应** (200):
```json
{
  "notebooks": [
    {
      "id": "abc123-def456",
      "title": "Research Notes",
      "is_owner": true,
      "created_at": "2024-01-15",
      "url": "https://notebooklm.google.com/notebook/abc123-def456"
    }
  ],
  "from_cache": true,
  "cached_at": "2026-05-31T10:30:00Z"
}
```

#### `POST /notebooks`

创建新笔记本。

**请求体**:
```json
{
  "title": "My New Notebook",
  "emoji": "📒"
}
```

**响应** (201):
```json
{
  "id": "xyz789-ghi012",
  "title": "My New Notebook",
  "emoji": "📒",
  "url": "https://notebooklm.google.com/notebook/xyz789-ghi012"
}
```

#### `POST /jobs`

提交发送任务。

**请求体**:
```json
{
  "notebook_id": "abc123-def456",
  "url": "https://example.com/article",
  "tasks": ["audio_overview", "slide_deck"]
}
```

`tasks` 可选值：
- `audio_overview` — 音频概览 (Deep Dive podcast)
- `slide_deck` — AI 幻灯片/演示文稿

**响应** (202):
```json
{
  "job_id": "uuid-v4",
  "status": "accepted",
  "notebook_id": "abc123-def456",
  "url": "https://example.com/article"
}
```

#### `GET /jobs/:id`

查询任务状态与进度。

**响应** (200):
```json
{
  "job_id": "uuid-v4",
  "status": "polling",
  "notebook_id": "abc123-def456",
  "notebook_title": "Research Notes",
  "url": "https://example.com/article",
  "pdf_path": "/tmp/send2nlm/abc.pdf",
  "source_id": "src-xyz",
  "task_results": {
    "audio_overview": {
      "task_id": "aud-001",
      "status": "processing",
      "started_at": "...",
      "completed_at": "",
      "asset_path": ""
    },
    "slide_deck": {
      "task_id": "sld-001",
      "status": "done",
      "started_at": "...",
      "completed_at": "...",
      "asset_path": "/Users/me/.send2nlm/tmp/sld-001.pdf"
    }
  },
  "created_at": "2026-05-31T10:30:00Z",
  "updated_at": "2026-05-31T10:32:00Z",
  "error": null
}
```

#### `GET /jobs`

列出所有 job（活跃 + 历史）。

**Query params**: `status=pending|producing|uploading|tasking|polling|downloading|receiving|done|failed`

---

## 5. Daemon CLI 接口

```bash
# One-shot 模式: 同步执行完整 pipeline，输出结果后退出
send2nlm send --notebook "abc123" --url "https://..." --tasks audio_overview,slide_deck

# Daemon 模式: 启动 HTTP 服务，常驻后台
send2nlm daemon [--port 18923]

# 停止 daemon
send2nlm stop

# 恢复未完成的 job (daemon 启动后自动调用)
send2nlm resume

# 开发模式 (配置目录 = dev_assets/)
send2nlm daemon --dev

# 辅助
send2nlm version
send2nlm config --path    # 打印配置目录路径
```

### 5.1 端口管理

```
优先级:
  1. --port 显式指定
  2. 固定默认端口 18923
  3. 若 18923 被占用，daemon 启动失败并提示用户使用 --port 显式指定

最终端口写入 ~/.send2nlm/daemon.port，PID 写入 ~/.send2nlm/daemon.pid
```

### 5.2 守护进程生命周期

```
send2nlm daemon
  │
  ├─ 写入 daemon.pid / daemon.port
  ├─ 初始化 SQLite 数据库
  ├─ 执行 resume: 扫描 status NOT IN (done, failed) 的 job，恢复执行
  ├─ 启动 HTTP server (阻塞)
  │   ├─ 接收新 job → 写入 SQLite → 入队 → 串行 FIFO 执行
  │   └─ 每个 job 的阶段变更立即写入 SQLite
  │
  └─ 收到 SIGTERM / send2nlm stop
       ├─ 停止接收新请求
       ├─ 等待当前 job 完成当前阶段 (graceful timeout 60s)
       ├─ 记录当前 job 进度到 SQLite (下次 resume 可恢复)
       ├─ 清理临时文件
       └─ 退出
```

---

## 6. SQLite 持久化层

### 6.1 数据库位置

- 生产: `~/.send2nlm/send2nlm.db`
- 开发: `<project>/dev_assets/send2nlm.db`

### 6.2 表结构

```sql
-- 笔记本缓存
CREATE TABLE IF NOT EXISTS notebooks (
    id          TEXT PRIMARY KEY,       -- NLM notebook id
    title       TEXT NOT NULL,
    is_owner    INTEGER DEFAULT 1,
    created_at  TEXT,
    url         TEXT,
    emoji       TEXT DEFAULT '📒',
    cached_at   TEXT DEFAULT (datetime('now'))
);

-- 任务记录
CREATE TABLE IF NOT EXISTS jobs (
    id            TEXT PRIMARY KEY,       -- UUID
    notebook_id   TEXT NOT NULL,
    notebook_title TEXT DEFAULT '',
    url           TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    -- status 取值: pending → producing → uploading → tasking → polling → downloading → receiving → done | failed
    tasks         TEXT NOT NULL DEFAULT '[]',       -- JSON array
    pdf_path      TEXT DEFAULT '',
    source_id     TEXT DEFAULT '',
    task_results  TEXT DEFAULT '{}',                 -- JSON object
    error         TEXT DEFAULT '',
    retry_count   INTEGER DEFAULT 0,
    polling_started_at TEXT DEFAULT '',              -- polling 阶段进入时间，支持 daemon 重启恢复
    created_at    TEXT DEFAULT (datetime('now')),
    updated_at    TEXT DEFAULT (datetime('now')),
    completed_at  TEXT DEFAULT ''
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_notebooks_cached_at ON notebooks(cached_at);
```

### 6.3 操作封装

```go
// store/notebooks.go
func (s *Store) ListNotebooks() ([]Notebook, error)
func (s *Store) UpsertNotebooks(notebooks []Notebook) error   // 全量刷新缓存
func (s *Store) InsertNotebook(n Notebook) error

// store/jobs.go
func (s *Store) CreateJob(job *Job) error
func (s *Store) GetJob(id string) (*Job, error)
func (s *Store) ListJobs(statusFilter string) ([]Job, error)
func (s *Store) UpdateJobStatus(id, status string) error
func (s *Store) UpdateJobProgress(id string, updates map[string]interface{}) error
func (s *Store) GetPendingJobs() ([]Job, error)  // 用于 resume
```

---

## 7. Producer 适配器系统 (URL → PDF)

采用 **子进程 + 编译缓存** 方案实现开闭原则：用户将 `.go` 源码文件放入 `~/.send2nlm/producer/` 目录，daemon 自动编译为独立二进制并缓存，通过 stdio JSON 协议通信。**无需手动编译、无需重启 daemon**。

### 7.1 运行时架构

```
~/.send2nlm/producer/             daemon (scriptmgr)
┌─────────────────────┐          ┌──────────────────────────────┐
│ lark.go             │          │  fsnotify 监听目录变化        │
│ my_site.go          │ ──发现──▶│  go build 编译为独立二进制    │
│ ...                 │          │  缓存到 cache/producer/<hash>/ │
└─────────────────────┘          │  通过 stdin/stdout JSON       │
                                 │  调用子进程通信               │
                                 │  注册到 ProducerRegistry      │
                                 └──────────────────────────────┘
```

关键特性：
- **自动编译缓存**：daemon 对每个 `.go` 源码做 SHA256 哈希，编译结果缓存到 `~/.send2nlm/cache/producer/<hash>/plugin`。源码不变则直接复用缓存二进制，**零启动开销**。
- **文件监听热更新**：daemon 使用 `fsnotify` 监听适配器目录，新增/修改/删除 `.go` 文件时自动重新编译并替换注册表中的适配器实例。
- **子进程隔离**：每个适配器作为独立子进程运行，通过 stdin/stdout 传递 JSON 消息；单个适配器崩溃不影响 daemon 或其他适配器。
- **不修改本体**：适配器与 daemon 完全解耦，daemon 二进制无需重新构建。

### 7.2 SDK 接口定义

```go
// sdk/producer.go  —— 编译进 daemon，外部适配器通过子进程通信实现此接口

package sdk

import "context"

// Producer 是 URL → PDF 的适配器接口。
// 外部适配器需实现此接口，并通过 ServeProducer 暴露为子进程。
// 内置适配器直接实现此接口注册到 ProducerRegistry。
type Producer interface {
    Name() string
    Match(url string) bool
    Produce(ctx context.Context, url string) (pdfPath string, err error)
}
```

外部适配器使用 `sdk.ServeProducer()` 将实现封装为 stdio JSON 子进程：

```go
// sdk/plugin_stdio.go  —— 子进程通信协议

// ServeProducer 将 Producer 实现封装为 stdio JSON 子进程。
// 适配器的 main() 调用此函数即可与 daemon 通信。
func ServeProducer(p Producer)
```

**通信协议**（stdin → stdout，一行一条 JSON 消息）：

| 方法 | 请求 | 响应 |
|------|------|------|
| `metadata` | `{"method":"metadata"}` | `{"name":"lark"}` |
| `match` | `{"method":"match","url":"https://..."}` | `{"name":"lark","match":true}` |
| `produce` | `{"method":"produce","url":"https://..."}` | `{"name":"lark","pdf_path":"/tmp/..."}` |

### 7.3 适配器管理器 (scriptmgr)

```go
// scriptmgr/external.go
// 核心职责：发现 .go 源文件 → 编译缓存 → 子进程调用
type Loader struct {
    configDir string  // ~/.send2nlm
    cacheDir  string  // ~/.send2nlm/cache
    moduleDir string  // daemon 模块根目录 (go build 用)
}

func NewLoader(configDir, cacheDir string) *Loader
func (l *Loader) compile(path string, kind PluginKind) (*compiledPlugin, error)
    // 1. 读源码 → SHA256 哈希
    // 2. 检查缓存目录 <cacheDir>/<kind>/<hash>/plugin，已有则跳过编译
    // 3. 否则: 重写 package → main, 追加 main() { sdk.ServeProducer(Producer) }
    // 4. go build → 输出二进制到缓存目录
    // 5. 调用 metadata 获取适配器名称
    // 6. 返回 compiledPlugin (封装二进制路径 + 配置目录)

// scriptmgr/producer_registry.go
type ProducerRegistry struct {
    builtins []sdk.Producer   // Default (编译进 daemon, 唯一内置兜底)
    scripts  []sdk.Producer   // 子进程适配器 (编译缓存加载, 含 Lark)
    mu       sync.RWMutex     // 热更新时保护读写
}

// Resolve 按优先级 for-loop：外部适配器优先 (含 Lark) → 内置 Default fallback
// 首个 Match 且 Produce 成功立即返回
func (r *ProducerRegistry) Resolve(ctx context.Context, url string) (pdfPath string, err error)
```

### 7.4 适配器编写规范

适配器是标准 Go 源码文件，放入 `~/.send2nlm/producer/*.go` 即生效。daemon 自动编译缓存，无需手动操作。

#### 最小的 Producer 适配器

```go
// ~/.send2nlm/producer/my_site.go
// 无需手动编译！daemon 自动 go build 并缓存

package producer  // 包名随意，但不能是 main

import (
    "context"
    "strings"

    "send2nlm/sdk"
)

type MyProducer struct{}

func (p *MyProducer) Name() string { return "my-site" }

func (p *MyProducer) Match(url string) bool {
    return strings.Contains(url, "mysite.com")
}

func (p *MyProducer) Produce(ctx context.Context, url string) (string, error) {
    // 自定义逻辑：调用 CLI、HTTP API、文件操作等
    // 返回生成的 PDF 文件路径
    return "/tmp/send2nlm/output.pdf", nil
}

// 必须导出此变量 (名称必须是 "Producer")
var Producer sdk.Producer = &MyProducer{}
```

#### Lark 适配器 (随 daemon 分发的扩展，首次运行自动复制到 producer 目录)

```go
// resources/producer/lark.go → 首次运行时复制到 ~/.send2nlm/producer/lark.go

package producer

import (
    "context"
    "fmt"
    "os/exec"
    "regexp"

    "send2nlm/sdk"
)

type LarkProducer struct{}

func (p *LarkProducer) Name() string { return "lark" }

var larkURLPattern = regexp.MustCompile(
    `^https?://[^/]*\.(feishu|feishu\.cn|larksuite|larkoffice)\.(com|cn)/docx/([A-Za-z0-9]+)`,
)

func (p *LarkProducer) Match(url string) bool {
    return larkURLPattern.MatchString(url)
}

func (p *LarkProducer) Produce(ctx context.Context, url string) (string, error) {
    // 1. 从 URL 提取 docx token
    matches := larkURLPattern.FindStringSubmatch(url)
    if len(matches) < 4 {
        return "", fmt.Errorf("cannot extract token from url: %s", url)
    }
    docType := "docx" // 0.0.1 固定只支持飞书 docx 链接
    token := matches[3]

    // 2. 调用 lark-cli 导出 PDF
    cmd := exec.CommandContext(ctx, "lark-cli", "drive", "+export",
        "--token", token,
        "--file-extension", "pdf",
        "--doc-type", docType,
        "--output-dir", "/tmp/send2nlm",
    )
    output, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("lark-cli export failed: %w\n%s", err, output)
    }
    return fmt.Sprintf("/tmp/send2nlm/%s.pdf", token), nil
}

var Producer sdk.Producer = &LarkProducer{}
```

### 7.5 唯一内置 Producer (编译进 daemon)

**仅 Default Producer 编译进 daemon 二进制**，作为最后的 fallback 保证——当所有外部适配器都无法处理某个 URL 时，由 Default 兜底。

Lark Producer 是**作为资源文件随 daemon 分发的扩展适配器**（见 7.4），与其他用户适配器一样通过子进程加载，首次运行时自动复制到 `~/.send2nlm/producer/`。

#### Default Producer (通用 fallback — 最低优先级)

```go
// producer/default.go
type DefaultProducer struct{}

func (p *DefaultProducer) Name() string  { return "default" }
func (p *DefaultProducer) Match(url string) bool { return true }
func (p *DefaultProducer) Produce(ctx context.Context, url string) (string, error) {
    // 1. opencli web read --download-images true -f json
    //    → 将 Markdown 与配图保存到输出目录，并从 JSON 的 saved 字段取得 .md 路径
    // 2. converter.MDFile2PDF(savedMarkdown, title, outputDir)
    //    → pandoc 读取 Markdown 文件
    //    → --resource-path 指向 Markdown 所在目录以解析相对图片路径
    //    → xelatex 将 Markdown 转为 PDF
    //    → 输出到 TempDir/<slug>.pdf
    // 3. 返回 PDF 路径
}
```

**MD → PDF 转换器技术选型**：

| 方案 | 优点 | 缺点 | 选用 |
|------|------|------|------|
| pandoc + xelatex (exec) | 原生支持 Markdown、相对图片、表格和代码块；PDF 输出稳定 | 需要系统安装 pandoc 与 TeX 引擎 | ✅ 首选 |

`pandoc` 或 `xelatex` 不可用时直接失败，并向 UI 返回可操作错误。0.0.1 不使用 `add-source --url` 或 `add-source --content` 降级，确保上传到 NotebookLM 的内容始终是 PDF。

### 7.6 资源文件首次安装

Daemon 启动时检查 `~/.send2nlm/producer/` 目录：
- 若 `lark.go` 不存在，从 Go 内嵌资源 (`//go:embed resources/producer/lark.go`) 复制到该目录
- 适配器目录下的所有 `.go` 文件由 `scriptmgr` 自动发现、编译缓存并加载

---

## 8. NotebookLM 操作封装 (nlm 包)

所有 NLM 交互通过 `os/exec` 调用 [notebooklm-py](https://github.com/teng-lin/notebooklm-py) CLI (`notebooklm`)，该库直接调用 Google 内部 RPC API，**无需浏览器**。

### 8.0 前置条件

用户在安装 daemon 前需要一次性完成 notebooklm-py 的认证：

```bash
pip install "notebooklm-py[browser]"
playwright install chromium          # 仅首次登录需要浏览器
notebooklm login                     # 打开浏览器完成 Google 登录
notebooklm auth check --test --json  # 验证: 应返回 {"status":"ok"}
```

完成后，`notebooklm` CLI 将 cookie 保存至 `~/.notebooklm/` 目录，daemon 直接使用无需额外认证。

### 8.1 命令调用约定

```go
// nlm/client.go

// 通用命令执行器：调用 notebooklm-py CLI
func execNotebookLM(ctx context.Context, args ...string) ([]byte, error)

// 启动时检查 notebooklm 是否已安装并已认证
func CheckNotebookLMAvailable(ctx context.Context) error
    // 执行: notebooklm status --json
    // 失败 → 提示用户安装 notebooklm-py 并运行 notebooklm login

// 所有命令统一使用 --json 输出格式
```

### 8.2 操作接口

```go
// nlm/notebook.go
func ListNotebooks(ctx context.Context) ([]Notebook, error)
    // 执行: notebooklm list --json
    // 解析 JSON 输出（兼容数组和 {notebooks: [...]} 两种格式）

func CreateNotebook(ctx context.Context, title, emoji string) (*Notebook, error)
    // 执行: notebooklm create <title> --use --json
    // 返回: {id, title, url}（id 优先取 id 字段，fallback 到 active_notebook_id）

// nlm/source.go
func AddFileSource(ctx context.Context, notebookID, filePath string) (sourceID string, err error)
    // 执行: notebooklm -n <notebookID> source add <filePath> --json
    // 返回: .source_id
    // notebooklm-py 内部通过 Google Drive 分块上传协议上传文件

// nlm/tasks.go
func GenerateAudio(ctx context.Context, notebookID string) (*GenTaskResponse, error)
    // 执行: notebooklm -n <id> generate audio "Create a deep-dive audio overview summarizing the content" --json
    // 返回: {task_id, status, task_type="audio_overview"}
    // 该命令立即返回 task_id，生成在后台异步进行

func GenerateSlides(ctx context.Context, notebookID string) (*GenTaskResponse, error)
    // 执行: notebooklm -n <id> generate slide-deck --json
    // 返回: {task_id, status, task_type="slide_deck"}

func PollArtifact(ctx context.Context, notebookID, taskID string) (*PollResponse, error)
    // 执行: notebooklm -n <id> artifact poll <taskID> --json
    // 返回: {task_id, status}  — status 为 "completed"/"failed"/"generating" 等
    // 纯 HTTP 调用，无需浏览器。每次调用约 1-2s

func PollUntilReady(ctx context.Context, notebookID string, tasks map[string]*GenTaskResponse, initialDelay, timeout, interval time.Duration) error
    // for-loop 调用 PollArtifact，直到全部 completed 或超时
    // 默认 initialDelay=10min, interval=1min, totalLimit=60min
    // 超过 totalLimit 时会做一次最终 artifact poll，防止 daemon 停机期间任务已完成却被误标失败
    // polling_started_at 持久化在 SQLite，daemon 重启后按 elapsed 继续等待/轮询
    // 在此期间 daemon 仍可处理其他 API 请求

func DownloadArtifacts(ctx context.Context, notebookID, outputDir string, tasks map[string]*GenTaskResponse) ([]DownloadedArtifact, error)
    // audio_overview: notebooklm -n <id> download audio <outputPath> --latest --force
    // slide_deck:    notebooklm -n <id> download slide-deck <outputPath> --latest --force
    // 纯 HTTP 下载，无需浏览器
    // 返回本地文件路径列表
```

### 8.3 上传策略

| 模式 | 参数 | NotebookLM 行为 |
|------|------|----------------|
| `source add <file>` | 本地 PDF 文件路径 | 通过 Google Drive 分块上传协议上传文件 |

**推荐策略**：
1. 始终走 URL → PDF → `source add` 上传流程（内容完全可控）
2. PDF 生成失败即 job 失败，不自动改用 URL source 或 Text source

---

## 9. Pipeline 编排与任务持久化

### 9.1 状态机

```
                    ┌─────────┐
                    │ PENDING │  ← 新 job 入库
                    └────┬────┘
                         │
                         ▼
              ┌──────────────┐
              │  PRODUCING   │  URL → PDF (producer 链)
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │  UPLOADING   │  add-source --file
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │   TASKING    │  generate-audio / generate-slides
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │   POLLING    │  先等待 10min，再每 1min 轮询 (artifact poll)，总上限 60min
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │ DOWNLOADING  │  下载 audio/slides 到本地 TempDir
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐
              │  RECEIVING   │  调用 Receiver 链 (可选)
              └──────┬───────┘
                     │
                     ▼
              ┌──────────────┐      ┌──────────┐
              │    DONE      │      │  FAILED  │  ← 任意阶段不可恢复错误
              └──────────────┘      └──────────┘
```

### 9.2 持久化策略

**每个阶段进入时写入 status，阶段产物生成后立即写入对应字段**，确保 crash 后能尽量从最近检查点恢复：

```go
func (p *Pipeline) Execute(job *Job) error {
    // 从上次中断点恢复
    switch job.Status {
    case "pending":
        goto PRODUCING
    case "producing":
        // 检查 pdf_path 是否已存在且有效
        if job.PDFPath != "" && fileExists(job.PDFPath) {
            goto UPLOADING
        }
        goto PRODUCING
    case "uploading":
        if job.SourceID != "" {
            goto TASKING
        }
        goto UPLOADING
    // ... 以此类推
    }

PRODUCING:
    p.store.UpdateJobStatus(job.ID, "producing")
    pdf, err := producer.Resolve(ctx, job.URL, producers)
    if err != nil { goto FAIL }
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "pdf_path": pdf,
    })
    job.PDFPath = pdf
    // fall through to UPLOADING...

UPLOADING:
    p.store.UpdateJobStatus(job.ID, "uploading")
    sourceID, err := nlm.AddFileSource(ctx, job.NotebookID, job.PDFPath)
    if err != nil { goto FAIL }
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "source_id": sourceID,
    })
    job.SourceID = sourceID

TASKING:
    p.store.UpdateJobStatus(job.ID, "tasking")
    taskResults, err := nlm.GenerateTasks(ctx, job.NotebookID, job.Tasks)
    if err != nil { goto FAIL }
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "task_results": taskResults,
    })

POLLING:
    p.store.UpdateJobStatus(job.ID, "polling")
    taskResults, err = nlm.PollUntilDone(ctx, job.NotebookID, taskResults)
    if err != nil { goto FAIL }
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "task_results": taskResults,
    })

DOWNLOADING:
    p.store.UpdateJobStatus(job.ID, "downloading")
    resources, err := nlm.DownloadTaskAssets(ctx, job.NotebookID, taskResults)
    if err != nil { goto FAIL }
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "task_results": attachAssetPaths(taskResults, resources),
    })

RECEIVING:
    p.store.UpdateJobStatus(job.ID, "receiving")
    receiver.Deliver(ctx, resources) // receiver 失败只记录日志
    p.store.UpdateJobProgress(job.ID, map[string]interface{}{
        "status": "done",
        "completed_at": now(),
    })
}
```

### 9.3 恢复机制 (Resume)

Daemon 启动时自动调用 `Resume()`:

```go
func (p *Pipeline) Resume() error {
    jobs, _ := p.store.GetPendingJobs() // status NOT IN (done, failed)
    for _, job := range jobs {
        p.Execute(&job) // 串行恢复，避免 session 冲突
    }
    return nil
}
```

### 9.4 超时与重试

| 阶段 | 超时 | 重试 | 失败处理 |
|------|------|------|----------|
| PRODUCING | 120s | 1 | 重试失败 → FAILED，记录 error |
| UPLOADING | 300s | 2 | 重试失败 → FAILED |
| TASKING | 60s | 1 | 重试失败 → FAILED |
| POLLING | 60min 总量（前 10min 不查询，之后每 1min 查询） | N/A | 超时前最后查询一次；仍未完成 → FAILED（保留最后状态） |
| DOWNLOADING | 300s | 2 | 重试失败 → FAILED（Receiver 需要本地资源） |
| RECEIVING | 60s | 2 | 仅日志告警，不影响 job 最终状态 |

---

## 10. Receiver 适配器系统 (资源投递)

与 Producer 对称，Receiver 也采用 **子进程 + 编译缓存** 方案。**仅 Download Receiver 编译进 daemon**（内置兜底）。Telegram 接收器作为资源文件随 daemon 分发，与其他用户适配器一样通过子进程加载。外部接收器以 `.go` 源码文件形式放在 `~/.send2nlm/receiver/`，**自动编译缓存、无需重启**。

### 10.1 SDK 接口定义

```go
// sdk/receiver.go

package sdk

import "context"

// Resource 是 pipeline 完成后产出的单个资源文件。
type Resource struct {
    TaskType       string // "audio_overview" | "slide_deck"
    AssetPath      string // 资源文件本地路径
    MimeType       string // 如 "audio/wav", "application/pdf"
    NotebookTitle  string
    NotebookURL    string
    SourceURL      string
}

// Config 是开放给适配器读取的配置结构。
// 适配器只能读取配置；写入配置由 daemon/CLI 负责。
type Config struct {
    Receivers map[string]map[string]interface{} `json:"receivers"`
}

func LoadConfig() Config

// Receiver 是资源投递适配器接口。
// 外部适配器需实现此接口，并通过 ServeReceiver 暴露为子进程。
// 内置适配器直接实现此接口注册到 ReceiverRegistry。
type Receiver interface {
    Name() string
    Receive(ctx context.Context, resources []Resource) error
}
```

### 10.2 适配器管理器

```go
// scriptmgr/external.go
// 同时管理 producer 和 receiver 目录的编译缓存与加载
func (l *Loader) compile(path string, kind PluginKind) (*compiledPlugin, error)

// scriptmgr/receiver_registry.go
type ReceiverRegistry struct {
    builtins []sdk.Receiver   // DownloadReceiver (编译进 daemon)
    scripts  []sdk.Receiver   // 子进程适配器 (编译缓存加载)
    mu       sync.RWMutex
}

// Deliver 依次调用所有已注册的 Receiver。
// 根据 config.json 中每个 receiver 的 enabled 字段决定是否执行。
// 单个 Receiver 失败仅记录日志，不影响后续。
func (r *ReceiverRegistry) Deliver(ctx context.Context, resources []sdk.Resource) []error
```

### 10.3 唯一内置 Receiver — Download (编译进 daemon)

**仅 Download Receiver 编译进 daemon 二进制**。Telegram 接收器是随 daemon 分发的扩展适配器（见 10.5），通过子进程加载。

Daemon 内置一个最低保证的 Receiver：将所有生成资源复制到 `~/Downloads/send2nlm/` 目录。

```go
// receiver/builtin.go

type DownloadReceiver struct {
    outputDir string  // 默认 ~/Downloads/send2nlm
}

func (r *DownloadReceiver) Name() string { return "download" }

func (r *DownloadReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
    // 1. 确保 ~/Downloads/send2nlm/ 目录存在
    // 2. 对每个 Resource: 复制 AssetPath → ~/Downloads/send2nlm/<timestamp>_<filename>
    // 3. 日志输出复制路径
}
```

### 10.4 Receiver 配置 (统一 toggle)

所有 Receiver（内置 + 外部适配器）的启用/禁用统一在 `~/.send2nlm/config.json` 中管理。

```json
{
  "receivers": {
    "download": {
      "enabled": true
    },
    "telegram": {
      "enabled": false,
      "bot_token": "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
      "chat_id": "-1001234567890"
    }
  }
}
```

规则：
- 每个 Receiver 由其 `Name()` 返回值作为配置 key
- `enabled: false` → 跳过，不参与 Deliver
- 内置 `download` receiver 默认 `enabled: true`；外部适配器默认 `enabled: true`
- 适配器通过 `sdk.LoadConfig()` 读取自身配置段

### 10.5 Telegram 适配器 (随 daemon 分发)

Telegram 接收器作为 Go 适配器实现，源码在 `resources/receiver/telegram.go`，首次运行时复制到 `~/.send2nlm/receiver/telegram.go`。

#### 适配器源码

```go
// ~/.send2nlm/receiver/telegram.go
// 无需手动编译！daemon 自动 go build 并缓存

package receiver

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    "path/filepath"

    "send2nlm/sdk"
)

type TelegramReceiver struct {
    token  string
    chatID string
    client *http.Client
}

func (r *TelegramReceiver) Name() string { return "telegram" }

func (r *TelegramReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
    // 1. 从 sdk.LoadConfig().Receivers["telegram"] 读取 bot_token 和 chat_id
    // 2. 若未配置 → 返回 nil (静默跳过)
    // 3. 发送摘要消息 (MarkdownV2):
    //    POST https://api.telegram.org/bot{token}/sendMessage
    // 4. 按 MimeType 发送文件:
    //    audio/*  → sendAudio (multipart)
    //    其他     → sendDocument
    //    文件 > 50MB → 跳过
    return nil
}

var Receiver sdk.Receiver = &TelegramReceiver{}
```

**Telegram Bot API 关键端点 (https://core.telegram.org/bots/api)**：

| 方法 | 用途 | 大小上限 |
|------|------|----------|
| `sendMessage` | 文本消息 (MarkdownV2/HTML) | — |
| `sendDocument` | 通用文件 | 50 MB |
| `sendAudio` | 音频文件 | 50 MB |
| `sendPhoto` | 图片 | 10 MB |

#### 用户配置

```json
// ~/.send2nlm/config.json
{
  "receivers": {
    "telegram": {
      "enabled": true,
      "bot_token": "YOUR_BOT_TOKEN",
      "chat_id": "YOUR_CHAT_ID"
    }
  }
}
```

获取凭据：`@BotFather` 创建机器人获取 token；`@userinfobot` 获取 Chat ID。

### 10.6 外部适配器开发

放入 `~/.send2nlm/receiver/*.go` 即生效，daemon 自动编译缓存，无需手动操作。

```go
// ~/.send2nlm/receiver/my_receiver.go

package receiver

import (
    "context"

    "send2nlm/sdk"
)

type MyReceiver struct{}

func (r *MyReceiver) Name() string { return "my-receiver" }

func (r *MyReceiver) Receive(ctx context.Context, resources []sdk.Resource) error {
    cfg := sdk.LoadConfig().Receivers["my-receiver"]
    // 自定义投递逻辑：邮件、Slack、企业微信、Webhook...
    _ = cfg
    return nil
}

var Receiver sdk.Receiver = &MyReceiver{}
```

在 `config.json` 中可通过 `"receivers": {"my-receiver": {"enabled": false}}` 禁用。

---

## 11. Chrome 扩展 UI 设计

### 11.1 页面结构 (360×480px)

#### Page 1: 笔记本选择 (`#page-notebooks`)

```
┌─────────────────────────────────┐
│  🔗 Send2NLM                    │
│  ─────────────────────────────  │
│                                 │
│  ┌───────────────────────────┐  │
│  │ 📓 Research Notes         │  │
│  │    3 sources · 2 days ago │  │
│  ├───────────────────────────┤  │
│  │ 📓 Reading List           │  │
│  │    12 sources · 1 week ago│  │
│  ├───────────────────────────┤  │
│  │ 📓 Project X              │  │
│  │    just now               │  │
│  ├───────────────────────────┤  │
│  │ 📓 ...                    │  │
│  └───────────────────────────┘  │
│                                 │
│  [+ Create Notebook]            │
│  [🔄 Refresh]                   │
└─────────────────────────────────┘
```

点击 [+ Create Notebook] → 弹出内联输入框，输入标题+选 emoji → 确认创建

#### Page 2: 发送确认 (`#page-send`)

```
┌─────────────────────────────────┐
│  ← Back          Send2NLM       │
│  ─────────────────────────────  │
│                                 │
│  📓 Sending to: Research Notes  │
│                                 │
│  📄 https://example.com/art...  │
│                                 │
│  Options:                       │
│  ☑ Audio Overview               │
│  ☐ Slide Deck                   │
│                                 │
│  ┌───────────────────────────┐  │
│  │         🚀  SEND          │  │
│  └───────────────────────────┘  │
│                                 │
└─────────────────────────────────┘
```

#### Page 3: 进度/结果 (`#page-result`)

```
┌─────────────────────────────────┐
│                         Send2NLM│
│  ─────────────────────────────  │
│                                 │
│  📓 Research Notes              │
│  📄 example.com/article         │
│                                 │
│  ✅ PDF generated               │
│  ✅ Uploaded to notebook        │
│  ⏳ Audio Overview (gen...)     │
│  ⬜ Slide Deck (waiting)        │
│                                 │
│  [ View in NotebookLM ↗ ]       │
└─────────────────────────────────┘
```

### 11.2 页面切换动画

CSS `transform: translateX()` + `transition: 300ms cubic-bezier(0.4, 0, 0.2, 1)`：

- 容器 `overflow: hidden`，内部 `#pages` 宽 `300%`，三个 page panel 各占 `33.33%`
- 状态跟踪: `currentPage = 0 | 1 | 2`
- 切换: `#pages.style.transform = translateX(-${currentPage * 33.333}%)`
- 方向感知: 从 Page1→Page2 左滑，Page2→Page1 右滑。通过 `data-direction="forward|back"` 属性微调动画曲线

### 11.3 国际化

```json
// _locales/en/messages.json
{
  "appName":            { "message": "Send2NLM" },
  "selectNotebook":     { "message": "Select a notebook" },
  "createNotebook":     { "message": "Create Notebook" },
  "refreshList":        { "message": "Refresh" },
  "sendButton":         { "message": "SEND" },
  "backButton":         { "message": "Back" },
  "audioOverview":      { "message": "Audio Overview" },
  "slideDeck":          { "message": "Slide Deck" },
  "statusGenerating":   { "message": "Generating…" },
  "statusDone":         { "message": "Done" },
  "viewInNotebookLM":   { "message": "View in NotebookLM" },
  "pageTitle":          { "message": "Send to NotebookLM" }
}

// _locales/zh_CN/messages.json
{
  "appName":            { "message": "Send2NLM" },
  "selectNotebook":     { "message": "选择笔记本" },
  "createNotebook":     { "message": "新建笔记本" },
  "refreshList":        { "message": "刷新" },
  "sendButton":         { "message": "发送" },
  "backButton":         { "message": "返回" },
  "audioOverview":      { "message": "音频概览" },
  "slideDeck":          { "message": "幻灯片" },
  "statusGenerating":   { "message": "生成中…" },
  "statusDone":         { "message": "已完成" },
  "viewInNotebookLM":   { "message": "在 NotebookLM 中查看" },
  "pageTitle":          { "message": "发送到 NotebookLM" }
}
```

---

## 12. 配置路径与环境

### 12.1 路径解析

```go
// core/config.go

var DevMode bool

func ConfigDir() string {
    if DevMode || os.Getenv("SEND2NLM_DEV") == "1" {
        return ProjectRoot + "/dev_assets"  // 编译时 ldflags 注入或 os.Getwd 推导
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".send2nlm")
}

func ProducerDir() string  { return filepath.Join(ConfigDir(), "producer") }
func ReceiverDir() string  { return filepath.Join(ConfigDir(), "receiver") }
func TempDir() string      { return filepath.Join(ConfigDir(), "tmp") }
func DBPath() string       { return filepath.Join(ConfigDir(), "send2nlm.db") }
func PortFile() string     { return filepath.Join(ConfigDir(), "daemon.port") }
func PIDFile() string      { return filepath.Join(ConfigDir(), "daemon.pid") }
```

### 12.2 开发环境

```bash
# 启动 daemon (开发模式: 所有路径指向 dev_assets/)
cd daemon && go run . daemon --dev

# 或设置环境变量
SEND2NLM_DEV=1 go run . daemon

# 加载扩展: Chrome → chrome://extensions → "Load unpacked" → extension/
```

---

## 13. 实现计划 (里程碑)

### Phase 1: 骨架搭建
- [ ] 项目仓库初始化 (go.mod / manifest.json / 目录结构)
- [ ] Go daemon CLI 入口 (main.go: daemon / stop / send 命令)
- [ ] HTTP server 骨架 (health 端点、CORS、端口管理)
- [ ] Chrome 扩展: manifest.json、popup 框架、三页滑动动画
- [ ] `daemon-client.js` 通信层

### Phase 2: 笔记本管理 + 持久化
- [ ] SQLite 初始化 (migrations, store 包)
- [ ] nlm 包: ListNotebooks / CreateNotebook
- [ ] GET /notebooks: 缓存优先 + 刷新逻辑
- [ ] POST /notebooks: 创建笔记本
- [ ] Chrome 扩展: Page 1 UI (笔记本列表 + 创建 + 刷新)

### Phase 3: 适配器子系统 + URL → PDF
- [ ] SDK 包: Producer 接口定义 (`sdk/producer.go`)
- [ ] SDK 包: 子进程通信协议 (`sdk/plugin_stdio.go`) — ServeProducer / ServeReceiver
- [ ] `scriptmgr/external.go`: 外部适配器发现、编译缓存、通信协议
- [ ] `scriptmgr/producer_registry.go`: 注册表 + for-loop Resolve
- [ ] Default Producer: 内置 HTTP 抓取 + MD→PDF 转换器
- [ ] Lark Producer 适配器 (`resources/producer/lark.go` → 首次复制到 `~/.send2nlm/producer/`)
- [ ] 适配器开发指南 (`scripts/README.md`)

### Phase 4: 核心 Pipeline
- [ ] nlm 包: AddFileSource / GenerateAudio / GenerateSlides
- [ ] Pipeline 编排 + 状态机 + SQLite 持久化
- [ ] Chrome 扩展: Page 2 (Send 界面)
- [ ] POST /jobs + GET /jobs/:id

### Phase 5: 任务轮询 + 下载 + 恢复
- [ ] 轮询逻辑 (`PollUntilReady`: 先等待 10min，再每 1min `artifact poll`，总上限 60min；失败前最终查询一次；支持 daemon 重启恢复)
- [ ] 下载逻辑 (`DownloadArtifacts`: `download audio`/`download slide-deck` → 本地 TempDir)
- [ ] Resume 机制 (启动时恢复未完成 job)
- [ ] Chrome 扩展: Page 3 (进度展示, 轮询 daemon)

### Phase 6: Receiver 适配器 + Telegram + 发布
- [ ] SDK 包: Receiver 接口定义 (`sdk/receiver.go`)
- [ ] SDK 配置读取: `sdk.LoadConfig()` + 只读 `Config`
- [ ] `scriptmgr/receiver_registry.go`: 注册表 + 调用链 + config.json toggle
- [ ] 内置 Download Receiver: 复制资源到 `~/Downloads/send2nlm/`
- [ ] Telegram 适配器 (`resources/receiver/telegram.go` → 首次复制到 `~/.send2nlm/receiver/`)
- [ ] `~/.send2nlm/config.json` 配置管理 (receivers 段, enabled toggle)
- [ ] i18n 完整覆盖
- [ ] 错误处理完善、友好提示
- [ ] Chrome 扩展打包 (zip → CWS 发布)

---

## 14. 开放问题

1. **编译缓存一致性**: 适配器源码通过 SHA256 哈希识别版本。修改源码后 daemon 自动重新编译，但大文件（如内嵌大量资源）可能编译耗时较长。首版设定 `go build` 超时为 2 分钟。

2. **文件监听防抖**: `fsnotify` 在 macOS/Linux 上行为良好，但需注意编辑器保存时的原子写入问题（vim 的 swap 文件、VSCode 的临时写入）。采用 300ms 防抖窗口合并连续事件，避免重复编译。

3. **notebooklm-py CLI 参数位置** (E2E 已发现并修复): `-n`/`--notebook` 参数必须放在子命令**之后**（如 `notebooklm source add -n <id>` 而非 `notebooklm -n <id> source add`）。

4. **notebooklm-py JSON 输出结构** (E2E 已验证):
   - `list --json` → `{notebooks: [{id, title, created_at}], count}`（嵌套在 `notebooks` key 内）
   - `create --use --json` → `{notebook: {id, title}, active_notebook_id}`（id 嵌套在 `notebook` 内）
   - `source add --json` → `{source: {id, title, type}}`（source_id 在 `source.id` 下）
   - `generate audio --json` → `{task_id, status: "pending"}`（平铺）
   - `artifact poll --json` → `{task_id, status}`（status: pending/in_progress/completed/failed）

5. **MD → PDF 保真度**: 内置 HTTP 抓取 + Markdown 转换方案对复杂页面的还原度如何？Phase 3 需评估。

6. **Pipeline panic 保护** (E2E 已修复): pipeline goroutine panic 会导致整个 daemon 崩溃。已添加 recover 保护。

7. **Lark wiki 链接**: `lark-cli drive +inspect` 会自动将 wiki 链接解包为 docx token，无需额外处理。

8. **更多 artifact 类型**: notebooklm-py 支持 video、quiz、flashcards、infographic、mind-map 等。0.0.1 仅实现 audio_overview 和 slide_deck。

9. **Receiver 日志**: 当前 receiver 错误被 pipeline 以 `_ =` 丢弃。需添加日志以便排查 Telegram 等投递问题。

---

> **下一步**: 评审更新后的设计，确认后进入 Phase 1 编码实现。
