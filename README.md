# Send2NLM

> Send any webpage to Google NotebookLM as PDF — with one click.

[English](./README.md) | [简体中文](./README.zh-CN.md)

<p align="center">
  <img src="./assets/send2nlm.png" alt="Send2NLM" width="120">
</p>

<p align="center">
  <img src="./assets/architecture.svg" alt="Send2NLM Architecture" width="960">
</p>

---

## What is Send2NLM?

Send2NLM is a **Chrome browser extension (MV3)** that sends the current webpage you're browsing to [Google NotebookLM](https://notebooklm.google.com) as a PDF. It can also automatically trigger **Audio Overviews** (deep-dive podcasts) and **Slide Decks**.

A lightweight **local Go daemon** handles all the heavy lifting — URL → PDF conversion, uploading to NotebookLM, task generation, status polling, and artifact download. No browser automation. No fragile DOM manipulation.

---

## Features

- 🚀 **One-click Send** — Send the current page to any notebook with a single click. No copy-paste, no manual uploads.
- 🎙️ **AI-Powered Generation** — Turn any article into a deep-dive podcast, presentation slides, and more — all powered by NotebookLM's AI.
- 🔔 **Auto-delivery** — Once sent, the daemon monitors generation progress and delivers finished artifacts to wherever you need them — no manual polling required.
- 🔌 **Extensible Adapters** — Handle any site or integrate any delivery channel. Write a few lines of Go to teach Send2NLM how to convert a specific webpage or where to send the results.
- 🌐 **English & 简体中文** — Full i18n support out of the box.

---

## How It Works

```
   ┌──────────────────────┐
   │  Chrome Extension    │     ① Click extension icon
   │  ┌────────┐┌───────┐│     ② Select target notebook
   │  │Popup UI││ SW BG ││     ③ Click "Send"
   │  └────────┘└───────┘│
   └─────────┬────────────┘
             │ HTTP POST /jobs
             ▼
   ┌─────────────────────────────────────────┐
   │           Go Daemon (Pipeline)           │
   │                                          │
   │  ┌──────────┐   ┌──────────┐            │
   │  │PRODUCING │──▶│UPLOADING │──▶ ... ──▶│
   │  │URL → PDF │   │to NLM    │            │  DONE
   │  └──────────┘   └──────────┘            │
   │       │               │                  │
   │       ▼               ▼                  │
   │  Producer         notebooklm-py         │
   │  Adapters            CLI                │
   └─────────────────────────────────────────┘
```

1. **PRODUCING** — URL is converted to PDF. Custom adapters handle specific sites (e.g., Lark docs via `lark-cli`); a built-in Default adapter handles all other pages via HTTP fetch → Markdown → PDF.
2. **UPLOADING** — PDF is uploaded to the target NotebookLM notebook via `notebooklm-py`.
3. **TASKING** — Selected tasks (Audio Overview / Slide Deck) are triggered.
4. **POLLING** — Daemon polls every 30s until all tasks complete (or 40min timeout).
5. **DOWNLOADING** — Generated artifacts are downloaded to local disk.
6. **RECEIVING** — Artifacts are delivered to configured receivers (local Downloads folder, Telegram, etc.).

---

## Prerequisites

| Dependency | Installation | Notes |
|------------|-------------|-------|
| [Go](https://go.dev/dl/) | ≥ 1.21 | To build the daemon |
| [notebooklm-py](https://github.com/teng-lin/notebooklm-py) | `uv tool install "notebooklm-py[cookies]"` | Google RPC API client |
| [opencli](https://github.com/anthropics/opencli) | `npm i -g opencli` | Web content fetching |
| Chrome / Chromium | ≥ 110 | MV3 support; also used for cookie-based auth |
| `wkhtmltopdf` | `brew install wkhtmltopdf` | Optional; preferred for Markdown→PDF |

### One-time setup

```bash
# 1. Install notebooklm-py and authenticate via browser cookies
uv tool install "notebooklm-py[cookies]"

# 2. List available Google accounts from your browser
notebooklm auth inspect --browser chrome

# 3. Login by reusing browser cookies (no new browser window)
notebooklm login --browser-cookies chrome --account your@gmail.com

# 4. Verify authentication
notebooklm auth check --test --json
```

---

## Installation

### Build from source

```bash
git clone https://github.com/your-org/send2nlm.git
cd send2nlm

# Build the Go daemon
cd daemon
go build -o send2nlm .
sudo cp send2nlm /usr/local/bin/

# Load the Chrome extension
# 1. Open chrome://extensions
# 2. Enable "Developer mode"
# 3. Click "Load unpacked" → select the `extension/` directory
```

### Start the daemon

```bash
# Start as background service
send2nlm daemon

# Or with a custom port
send2nlm daemon --port 18923

# One-shot mode (no daemon needed)
send2nlm send --notebook "abc123" --url "https://example.com" --tasks audio_overview,slide_deck
```

---

## Usage

1. Click the **Send2NLM icon** 🧩 in your Chrome toolbar
2. **Select a notebook** from the list (or create a new one)
3. Toggle **Audio Overview** and/or **Slide Deck**
4. Click **Send**
5. Watch the progress — the pipeline processes each step automatically
6. When done, click **View in NotebookLM** to open the notebook

---

## Configuration

All configuration lives in `~/.send2nlm/`:

```
~/.send2nlm/
├── config.json          # Daemon & receiver settings
├── producer/            # Custom URL→PDF adapters (*.go)
├── receiver/            # Custom delivery adapters (*.go)
├── cache/               # Compiled adapter binaries
├── tmp/                 # Temporary PDFs & downloads
├── send2nlm.db          # SQLite database
├── daemon.port          # Current daemon port
└── daemon.pid           # Current daemon PID
```

### `config.json`

```json
{
  "receivers": {
    "download": { "enabled": true },
    "telegram": {
      "enabled": false,
      "bot_token": "YOUR_BOT_TOKEN",
      "chat_id": "YOUR_CHAT_ID"
    }
  }
}
```

---

## Custom Adapters

### Write a Producer (URL → PDF)

Put a `.go` file in `~/.send2nlm/producer/`. The daemon auto-compiles and caches it.

```go
// ~/.send2nlm/producer/my_site.go
package producer

import (
    "context"
    "strings"
    "send2nlm/sdk"
)

type MyProducer struct{}

func (p *MyProducer) Name() string          { return "my-site" }
func (p *MyProducer) Match(url string) bool { return strings.Contains(url, "mysite.com") }
func (p *MyProducer) Produce(ctx context.Context, url string) (string, error) {
    // Generate PDF from URL → return file path
    return "/tmp/send2nlm/output.pdf", nil
}

var Producer sdk.Producer = &MyProducer{}
```

### Write a Receiver (Artifact Delivery)

Put a `.go` file in `~/.send2nlm/receiver/`:

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
    // Deliver artifacts: email, Slack, webhook, etc.
    cfg := sdk.LoadConfig().Receivers["my-receiver"]
    _ = cfg
    return nil
}

var Receiver sdk.Receiver = &MyReceiver{}
```

Toggle it in `config.json`:

```json
{
  "receivers": {
    "my-receiver": { "enabled": true }
  }
}
```

---

## Development

```bash
# Start daemon in dev mode (uses dev_assets/ instead of ~/.send2nlm/)
cd daemon && go run . daemon --dev

# Or with environment variable
SEND2NLM_DEV=1 go run . daemon

# Load extension
# Chrome → chrome://extensions → "Load unpacked" → extension/
```

### Project Structure

```
send2nlm/
├── extension/           # Chrome Extension (MV3)
│   ├── popup/           #   UI: notebook select, send, progress
│   ├── background/      #   Service worker
│   └── shared/          #   Daemon client & i18n helpers
├── daemon/              # Go daemon
│   ├── server/          #   HTTP server & API handlers
│   ├── core/            #   Pipeline orchestration & config
│   ├── store/           #   SQLite persistence
│   ├── nlm/             #   notebooklm-py CLI wrapper
│   ├── producer/        #   Built-in Default producer
│   ├── receiver/        #   Built-in Download receiver
│   ├── converter/       #   Markdown → PDF converter
│   ├── scriptmgr/       #   Adapter discovery, compile, cache
│   ├── sdk/             #   Shared interfaces & stdio protocol
│   └── resources/       #   Embedded adapters (Lark, Telegram)
├── scripts/             # Adapter examples
├── dev_assets/          # Dev environment mock
└── DESIGN.md            # Full design document
```
