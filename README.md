# web-nfc-bridge

[English](#english) | [繁體中文](#繁體中文)

---

## English

A working example of reading and writing NFC cards from a web browser — using a localhost connector architecture instead of the experimental Web NFC API.

```
Browser (HTTPS)  ←→  Localhost Connector (Go)  ←→  PC/SC  ←→  NFC Reader
```

### Why

The [Web NFC API](https://developer.mozilla.org/en-US/docs/Web/API/Web_NFC_API) is Chrome-only, Android-only, and still experimental. This project demonstrates an alternative approach: a lightweight Go service running on `127.0.0.1` that bridges any web app to a USB NFC reader via the standard PC/SC interface.

This architecture works on **macOS, Windows (x64/ARM64), and Linux** — anywhere a browser and a USB reader can coexist.

### Features

- **Read** — Extract UID and JSON-encoded NDEF data from NFC Type 2 cards
- **Write** — Write structured JSON payloads as NDEF records with read-back verification
- **Real-time events** — WebSocket stream for card presence, reader status, and operation results
- **Security by design** — HMAC-signed short-lived tokens, origin allowlist, no arbitrary APDU passthrough
- **Cross-platform** — PC/SC driver for macOS/Windows/Linux, plus a Direct IOCTL driver for Windows ARM64

### Architecture

```
┌─────────────────────────────────────────────┐
│  Browser                                    │
│  Nuxt 4 + Nuxt UI (deployed to Cloudflare)  │
└──────────────┬──────────────────────────────┘
               │ HTTP / WebSocket
               ▼
┌──────────────────────────────────────┐
│  Localhost Connector (Go)            │
│  http://127.0.0.1:42619             │
│                                      │
│  ┌────────────┐  ┌────────────────┐  │
│  │ PCSC Driver │  │ Direct Driver  │  │
│  │ (all OS)    │  │ (Win ARM64)    │  │
│  └──────┬─────┘  └───────┬────────┘  │
└─────────┼────────────────┼───────────┘
          ▼                ▼
   ┌─────────────────────────────┐
   │  NFC Reader (ACR1252U-M1)   │
   │  ISO-14443 Type 2 cards     │
   └─────────────────────────────┘
```

### Quick Start

#### Prerequisites

- [Node.js](https://nodejs.org/) 22+ and [pnpm](https://pnpm.io/) 10+
- [Go](https://go.dev/) 1.26+
- A PC/SC-compatible NFC reader (tested with ACS ACR1252U-M1)
- PC/SC service: built-in on macOS/Windows; install `pcscd` on Linux

#### Run in Development

```bash
# Install frontend dependencies
pnpm install

# Terminal 1 — Start the web app
pnpm dev

# Terminal 2 — Start the connector
pnpm connector:dev
```

Open http://localhost:3000 — the web UI will automatically detect the connector and list available readers.

#### Build

```bash
# Build everything (web app + connector + installers)
pnpm build

# Or build individually
pnpm build:app              # Nuxt web app (Node.js)
pnpm build:cf               # Nuxt web app (Cloudflare Workers)
pnpm connector:build        # Go connector binary
```

#### Build Installers

```bash
pnpm installers:build:macos          # macOS .pkg
pnpm installers:build:windows-x64    # Windows x64 .msi
pnpm installers:build:windows-arm64  # Windows ARM64 .msi
pnpm installers:build:linux-x64      # Linux .deb
```

### Connector HTTP API

Base URL: `http://127.0.0.1:42619`

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Connector health check |
| `GET` | `/readers` | List available NFC readers |
| `POST` | `/session/connect` | Start a reader session |
| `POST` | `/card/read` | Read card UID + NDEF data |
| `POST` | `/card/write` | Write NDEF payload to card |
| `WS` | `/events` | Real-time event stream |

All endpoints (except `/health`) require an `X-Bridge-Token` header with a signed ticket issued by the web app's server route `POST /api/connector-ticket`.

See [docs/connector-contract.md](docs/connector-contract.md) for full API details.

### Write Payload Example

```json
{
  "sessionId": "1710000000000000000",
  "operation": "ndef-v1",
  "payload": {
    "version": 1,
    "type": "web-nfc-bridge/demo",
    "label": "MO-20260309-001",
    "content": {
      "documentNo": "MO-20260309-001",
      "itemCode": "FG-1001",
      "workstation": "PACK-01",
      "quantity": 24,
      "status": "ready"
    },
    "updatedAt": "2026-03-09T01:35:40Z"
  }
}
```

Constraints:
- `content` must be a flat JSON object (no nesting), max 8 fields
- Values: string, number, boolean, or null only
- Max encoded NDEF size: 256 bytes
- Sensitive fields (`password`, `secret`, `email`, etc.) are rejected

### Security Model

1. **No raw APDU** — The connector only accepts pre-defined operations, never arbitrary card commands
2. **Short-lived tokens** — HMAC-signed tickets expire in 60 seconds, scoped to `read`, `write`, `events`, or `all`
3. **Origin allowlist** — Only whitelisted origins can communicate with the connector
4. **Payload validation** — Write payloads are validated for structure, size, and forbidden fields before any card I/O
5. **Read-back verification** — After writing, the connector reads the card back to confirm data integrity

### Tech Stack

| Layer | Technology |
|-------|-----------|
| Frontend | [Nuxt 4](https://nuxt.com/) + [Nuxt UI](https://ui.nuxt.com/) + Tailwind CSS |
| Connector | [Go](https://go.dev/) + [gorilla/websocket](https://github.com/gorilla/websocket) |
| Smart Card | [ebfe/scard](https://github.com/ebfe/scard) (PC/SC binding) |
| Deployment | [Cloudflare Workers](https://workers.cloudflare.com/) (web app only) |

### Platform Support

| Platform | Architecture | Driver | Status |
|----------|-------------|--------|--------|
| macOS | Apple Silicon / Intel | PC/SC | Validated |
| Windows | ARM64 | Direct IOCTL | Validated |
| Windows | x64 | PC/SC | Supported |
| Linux | x64 | PC/SC | Supported |

### Project Structure

```
├── pages/                  # Nuxt pages (web UI)
├── composables/            # Vue composables (connector communication)
├── server/api/             # Nuxt server routes (ticket issuance)
├── connector/
│   ├── cmd/nfc-connector/  # Go entrypoint
│   └── internal/
│       ├── api/            # HTTP + WebSocket server
│       ├── auth/           # Token verification
│       └── bridge/         # Driver layer (PCSC, Direct IOCTL, NDEF codec)
├── scripts/                # Installer build scripts
└── docs/                   # Architecture & API documentation
```

### Configuration

Environment variables for the connector:

| Variable | Default | Description |
|----------|---------|-------------|
| `NFC_CONNECTOR_ADDR` | `127.0.0.1:42619` | Connector listen address |
| `NFC_CONNECTOR_SHARED_SECRET` | `development-shared-secret` | HMAC signing key |
| `NFC_CONNECTOR_ALLOWED_ORIGINS` | localhost + 127.0.0.1 | Comma-separated origin allowlist |

### Documentation

- [Architecture](docs/architecture.md) — System layers and design decisions
- [Connector Contract](docs/connector-contract.md) — Full HTTP/WebSocket API specification
- [Connector Operations](docs/connector-operations.md) — Operational details
- [Platform Support](docs/platform-support.md) — Platform validation matrix

---

## 繁體中文

一個從網頁瀏覽器讀寫 NFC 卡片的完整範例 — 採用 localhost connector 架構，取代實驗性的 Web NFC API。

```
瀏覽器 (HTTPS)  ←→  本機 Connector (Go)  ←→  PC/SC  ←→  NFC 讀卡機
```

### 為什麼做這個

[Web NFC API](https://developer.mozilla.org/en-US/docs/Web/API/Web_NFC_API) 僅限 Chrome、僅限 Android，且仍處於實驗階段。本專案示範另一種做法：在 `127.0.0.1` 執行一個輕量 Go 服務，透過標準 PC/SC 介面將任何 Web 應用程式橋接到 USB NFC 讀卡機。

此架構可在 **macOS、Windows（x64/ARM64）及 Linux** 上運作 — 只要有瀏覽器和 USB 讀卡機的環境都能使用。

### 功能特色

- **讀卡** — 從 NFC Type 2 卡片擷取 UID 及 JSON 編碼的 NDEF 資料
- **寫卡** — 將結構化 JSON 以 NDEF 記錄寫入卡片，並回讀驗證
- **即時事件** — 透過 WebSocket 串流卡片感應、讀卡機狀態與操作結果
- **安全設計** — HMAC 簽章的短效 token、來源白名單、禁止任意 APDU 直通
- **跨平台** — macOS/Windows/Linux 使用 PC/SC 驅動，Windows ARM64 另有 Direct IOCTL 驅動

### 架構

```
┌───────────────────────────────────────────────┐
│  瀏覽器                                       │
│  Nuxt 4 + Nuxt UI（部署至 Cloudflare）         │
└──────────────┬────────────────────────────────┘
               │ HTTP / WebSocket
               ▼
┌──────────────────────────────────────┐
│  本機 Connector (Go)                 │
│  http://127.0.0.1:42619             │
│                                      │
│  ┌────────────┐  ┌────────────────┐  │
│  │ PCSC 驅動   │  │ Direct 驅動    │  │
│  │ (全平台)    │  │ (Win ARM64)    │  │
│  └──────┬─────┘  └───────┬────────┘  │
└─────────┼────────────────┼───────────┘
          ▼                ▼
   ┌─────────────────────────────┐
   │  NFC 讀卡機 (ACR1252U-M1)   │
   │  ISO-14443 Type 2 卡片      │
   └─────────────────────────────┘
```

### 快速開始

#### 前置需求

- [Node.js](https://nodejs.org/) 22+ 及 [pnpm](https://pnpm.io/) 10+
- [Go](https://go.dev/) 1.26+
- PC/SC 相容的 NFC 讀卡機（已測試 ACS ACR1252U-M1）
- PC/SC 服務：macOS/Windows 內建；Linux 需安裝 `pcscd`

#### 開發模式執行

```bash
# 安裝前端依賴
pnpm install

# 終端機 1 — 啟動 Web 應用程式
pnpm dev

# 終端機 2 — 啟動 Connector
pnpm connector:dev
```

開啟 http://localhost:3000 — Web 介面會自動偵測 Connector 並列出可用的讀卡機。

#### 建置

```bash
# 建置全部（Web 應用程式 + Connector + 安裝包）
pnpm build

# 或個別建置
pnpm build:app              # Nuxt Web 應用程式（Node.js）
pnpm build:cf               # Nuxt Web 應用程式（Cloudflare Workers）
pnpm connector:build        # Go Connector 執行檔
```

#### 建置安裝包

```bash
pnpm installers:build:macos          # macOS .pkg
pnpm installers:build:windows-x64    # Windows x64 .msi
pnpm installers:build:windows-arm64  # Windows ARM64 .msi
pnpm installers:build:linux-x64      # Linux .deb
```

### Connector HTTP API

Base URL：`http://127.0.0.1:42619`

| 方法 | 路徑 | 說明 |
|------|------|------|
| `GET` | `/health` | Connector 健康檢查 |
| `GET` | `/readers` | 列出可用的 NFC 讀卡機 |
| `POST` | `/session/connect` | 建立讀卡工作階段 |
| `POST` | `/card/read` | 讀取卡片 UID + NDEF 資料 |
| `POST` | `/card/write` | 寫入 NDEF 資料至卡片 |
| `WS` | `/events` | 即時事件串流 |

除 `/health` 外，所有端點皆需在 `X-Bridge-Token` header 中帶入由 Web 應用程式的 `POST /api/connector-ticket` 簽發的 token。

完整 API 規格請參閱 [docs/connector-contract.md](docs/connector-contract.md)。

### 寫卡 Payload 範例

```json
{
  "sessionId": "1710000000000000000",
  "operation": "ndef-v1",
  "payload": {
    "version": 1,
    "type": "web-nfc-bridge/demo",
    "label": "MO-20260309-001",
    "content": {
      "documentNo": "MO-20260309-001",
      "itemCode": "FG-1001",
      "workstation": "PACK-01",
      "quantity": 24,
      "status": "ready"
    },
    "updatedAt": "2026-03-09T01:35:40Z"
  }
}
```

限制條件：
- `content` 必須是單層 JSON 物件（不可巢狀），最多 8 個欄位
- 值僅允許字串、數字、布林或 null
- NDEF 編碼後上限 256 bytes
- 敏感欄位（`password`、`secret`、`email` 等）會被拒絕

### 安全模型

1. **禁止原始 APDU** — Connector 僅接受預先定義的操作，不允許任意卡片指令
2. **短效 Token** — HMAC 簽章的 ticket 60 秒過期，可指定 `read`、`write`、`events` 或 `all` 範圍
3. **來源白名單** — 僅允許白名單中的來源與 Connector 通訊
4. **Payload 驗證** — 寫卡 payload 在任何卡片 I/O 前先驗證結構、大小與禁止欄位
5. **回讀驗證** — 寫入後 Connector 會回讀卡片以確認資料完整性

### 技術棧

| 層級 | 技術 |
|------|------|
| 前端 | [Nuxt 4](https://nuxt.com/) + [Nuxt UI](https://ui.nuxt.com/) + Tailwind CSS |
| Connector | [Go](https://go.dev/) + [gorilla/websocket](https://github.com/gorilla/websocket) |
| 智慧卡 | [ebfe/scard](https://github.com/ebfe/scard)（PC/SC binding） |
| 部署 | [Cloudflare Workers](https://workers.cloudflare.com/)（僅 Web 應用程式） |

### 平台支援

| 平台 | 架構 | 驅動 | 狀態 |
|------|------|------|------|
| macOS | Apple Silicon / Intel | PC/SC | 已驗證 |
| Windows | ARM64 | Direct IOCTL | 已驗證 |
| Windows | x64 | PC/SC | 支援 |
| Linux | x64 | PC/SC | 支援 |

### 專案結構

```
├── pages/                  # Nuxt 頁面（Web 介面）
├── composables/            # Vue composables（Connector 通訊）
├── server/api/             # Nuxt server routes（token 簽發）
├── connector/
│   ├── cmd/nfc-connector/  # Go 進入點
│   └── internal/
│       ├── api/            # HTTP + WebSocket 伺服器
│       ├── auth/           # Token 驗證
│       └── bridge/         # 驅動層（PCSC、Direct IOCTL、NDEF 編解碼）
├── scripts/                # 安裝包建置腳本
└── docs/                   # 架構與 API 文件
```

### 設定

Connector 環境變數：

| 變數 | 預設值 | 說明 |
|------|--------|------|
| `NFC_CONNECTOR_ADDR` | `127.0.0.1:42619` | Connector 監聽位址 |
| `NFC_CONNECTOR_SHARED_SECRET` | `development-shared-secret` | HMAC 簽章金鑰 |
| `NFC_CONNECTOR_ALLOWED_ORIGINS` | localhost + 127.0.0.1 | 以逗號分隔的來源白名單 |

### 文件

- [架構](docs/architecture.md) — 系統分層與設計決策
- [Connector 契約](docs/connector-contract.md) — 完整 HTTP/WebSocket API 規格
- [Connector 操作](docs/connector-operations.md) — 運作細節
- [平台支援](docs/platform-support.md) — 平台驗證矩陣

---

## Credits

Built by [Yudefine - 域定資訊工作室](https://yudefine.com.tw)

## License

[MIT](LICENSE)
