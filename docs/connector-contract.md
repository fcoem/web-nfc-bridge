# Local Connector Contract

## Base URL

暫定 base URL：`http://127.0.0.1:42619`

## HTTP API

### GET /health

用途：回報 Connector 是否可用。

範例回應：

```json
{
  "status": "ok",
  "version": "0.1.0"
}
```

### GET /readers

用途：列出目前可用的讀卡機。

範例回應：

```json
{
  "readers": [
    {
      "name": "ACS ACR1252 1S CL Reader PICC 0"
    }
  ]
}
```

### POST /session/connect

用途：建立讀卡工作階段。

範例請求：

```json
{
  "readerName": "ACS ACR1252 1S CL Reader PICC 0"
}
```

### POST /card/read

用途：執行受控的讀卡命令，不接受任意 APDU。

範例請求：

```json
{
  "sessionId": "1710000000000000000",
  "operation": "summary"
}
```

### POST /card/write

用途：執行受控的寫卡命令，不接受任意 APDU。

範例請求：

```json
{
  "sessionId": "1710000000000000000",
  "operation": "ndef-v1",
  "payload": {
    "version": 1,
    "type": "nfc-tool/demo",
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

受支援 payload 類型：

- `nfc-tool/demo`
- `nfc-tool/ref`

限制：

- media type 固定為 `application/json`
- v1 payload 大小上限為 `256` bytes
- `nfc-tool/demo` 可選擇寫入 `label` 或 `content`
- `content` 必須是單層 JSON 物件，最多 `8` 個欄位，值只允許字串、數字、布林或 `null`
- 不允許任意巢狀物件、陣列或敏感資料欄位
- `nfc-tool/ref` 必須提供 `token`

## WebSocket Events

`WS /events`

事件型別：

- `reader.status`
- `card.present`
- `card.removed`
- `card.read.complete`
- `card.write.complete`
- `error`

範例事件：

```json
{
  "type": "reader.status",
  "reader": {
    "name": "ACS ACR1252 1S CL Reader PICC 0"
  },
  "status": "ready"
}
```

## Security Boundary

- Requirement: Localhost Security Controls
  - Connector 必須驗證來源網域
  - Connector 必須驗證短效 token
  - Connector 不接受任意 APDU 直通
  - 敏感寫卡操作必須由明確使用者觸發
- Decision 4: Connector 對外提供 HTTP + WebSocket 雙通道
  - HTTP 作為控制介面
  - WebSocket 作為事件通道
- Decision 5: 安全邊界在 Connector 層強制執行
  - 所有硬體指令皆需經受控命令層轉譯

## Website Authorization Flow

- Nuxt server route `POST /api/connector-ticket` 簽發 60 秒有效的 HMAC ticket
- Connector 以 `X-Bridge-Token` 或 `WS /events?token=` 驗證 ticket
- scope 分為 `all`、`read`、`write`、`events`

## Safe Write Policy

- v1 寫卡 profile 固定為 `ndef-v1`
- Connector 只接受 NDEF-compatible profile payload，不接受 raw block write
- `nfc-tool/demo` 適合本地驗證與 UI smoke test
- `nfc-tool/ref` 適合 production reference token，不應直接承載完整業務資料
- 驗證失敗時，Connector 會在嘗試寫卡前直接回傳 validation error
- 在已支援的實體 driver 上，Connector 只有在完成卡片寫入並讀回驗證後，才會回傳 `accepted: true`
- 目前已驗證的實體寫入路徑為 macOS `pcsc` driver 搭配 NDEF formatted Type 2 tag

## State and Reconnect

- 狀態：`offline`、`connecting`、`ready`、`busy`、`error`
- Web 前端在首次載入與手動重試時執行 `/health`
- 若 WebSocket 中斷，前端回退到 `/health` 輪詢並提示使用者重新連線
