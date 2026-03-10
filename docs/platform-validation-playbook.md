# Platform Validation Playbook

本文件提供目前可直接執行的兩條驗證路徑：

- macOS regression check
- Windows ARM64 風險驗證

目前不涵蓋 Windows x64 與 Linux 的正式驗證執行；兩者待有對應環境後再補。

## Shared Success Criteria

- Connector 可在 `pcsc` driver 下啟動
- `/health` 回 `status=ok`
- `/readers` 可列出實體 reader
- `/session/connect` 可對 `PICC` reader 建立 session
- `/card/read` 可回 UID 與 ATR
- 若平台已進到受控實體寫入階段，`/card/write` 應只在 driver 完成 write + verify 後回 `accepted: true`

## Shared Local Defaults

- Web app URL: `http://localhost:3000`
- Connector base URL: `http://127.0.0.1:42619`
- Shared secret: `development-shared-secret`
- Recommended driver mode during hardware validation: `pcsc`
- 建議開發 allowlist：`http://localhost:*`,`https://localhost:*`,`http://127.0.0.1:*`,`https://127.0.0.1:*`

若本機已經有 mock Connector 佔用 `42619`，可改用例如 `127.0.0.1:42620` 啟動 `pcsc` Connector；此時測試命令中的 base URL 也要一併改成相同 port。

## macOS Regression Check

目的：確認既有 macOS read/write 路徑沒有回歸，並在需要時重新驗證真卡可完成 `ndef-v1` 寫入。

### Prerequisites

- ACR1252U-M1 已接上 macOS
- 可寫的 NDEF formatted Type 2 tag
- 已安裝 Go、Node.js、pnpm

### 1. Build artifacts

```bash
pnpm install
pnpm run build
go build ./connector/cmd/nfc-connector
```

### 2. Start services

Terminal A:

```bash
pnpm dev
```

Terminal B:

```bash
NFC_CONNECTOR_DRIVER=pcsc \
NFC_CONNECTOR_ALLOWED_ORIGINS=http://localhost:3000 \
NFC_CONNECTOR_SHARED_SECRET=development-shared-secret \
go run ./connector/cmd/nfc-connector
```

### 3. Check health and readers

```bash
TOKEN=$(node -e "const crypto=require('node:crypto');const payload=Buffer.from(JSON.stringify({origin:'http://localhost:3000',scope:'all',exp:Math.floor(Date.now()/1000)+120})).toString('base64url');const signature=crypto.createHmac('sha256','development-shared-secret').update(payload).digest('base64url');process.stdout.write(payload + '.' + signature)")

curl -s http://127.0.0.1:42619/health
curl -s -H 'Origin: http://localhost:3000' \
  -H "X-Bridge-Token: $TOKEN" \
  http://127.0.0.1:42619/readers
```

Expected:

- `/health` 回 `status: ok`
- `/readers` 可看到 `ACS ACR1252 Dual Reader PICC`

已驗證範例：2026-03-09 在本機同時存在 mock `42619` 與 `pcsc` `42620` 的情況下，`pcsc` flow 仍可正常完成 health 與 readers 驗證。

### 4. Run controlled read/write flow

```bash
TOKEN=$(node -e "const crypto=require('node:crypto');const payload=Buffer.from(JSON.stringify({origin:'http://localhost:3000',scope:'all',exp:Math.floor(Date.now()/1000)+120})).toString('base64url');const signature=crypto.createHmac('sha256','development-shared-secret').update(payload).digest('base64url');process.stdout.write(payload + '.' + signature)")
READ_TOKEN=$(node -e "const crypto=require('node:crypto');const payload=Buffer.from(JSON.stringify({origin:'http://localhost:3000',scope:'read',exp:Math.floor(Date.now()/1000)+120})).toString('base64url');const signature=crypto.createHmac('sha256','development-shared-secret').update(payload).digest('base64url');process.stdout.write(payload + '.' + signature)")
WRITE_TOKEN=$(node -e "const crypto=require('node:crypto');const payload=Buffer.from(JSON.stringify({origin:'http://localhost:3000',scope:'write',exp:Math.floor(Date.now()/1000)+120})).toString('base64url');const signature=crypto.createHmac('sha256','development-shared-secret').update(payload).digest('base64url');process.stdout.write(payload + '.' + signature)")

SESSION_JSON=$(curl -s -X POST \
  -H 'Origin: http://localhost:3000' \
  -H "X-Bridge-Token: $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"readerName":"ACS ACR1252 Dual Reader PICC"}' \
  http://127.0.0.1:42619/session/connect)

SESSION_ID=$(printf '%s' "$SESSION_JSON" | node -e "const fs=require('fs');const data=JSON.parse(fs.readFileSync(0,'utf8'));process.stdout.write(data.id || '')")

curl -s -X POST \
  -H 'Origin: http://localhost:3000' \
  -H "X-Bridge-Token: $READ_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"sessionId\":\"$SESSION_ID\",\"operation\":\"summary\"}" \
  http://127.0.0.1:42619/card/read

curl -s -X POST \
  -H 'Origin: http://localhost:3000' \
  -H "X-Bridge-Token: $WRITE_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"sessionId\":\"$SESSION_ID\",\"operation\":\"ndef-v1\",\"payload\":{\"version\":1,\"type\":\"web-nfc-bridge/demo\",\"label\":\"Demo card label\",\"updatedAt\":\"2026-03-09T01:35:40Z\"}}" \
  http://127.0.0.1:42619/card/write
```

Expected:

- `session/connect` 成功回 session id
- `card/read` 回 UID 與 ATR
- `card/write` 回 `accepted: true`
- `details.profile` 為 `ndef-write-profile/v1`

已驗證範例：2026-03-09 macOS regression 實測結果為 `uid=0472650DCC2A81`、`accepted=true`、`details.profile=ndef-write-profile/v1`、`pagesWritten=30`。

### 5. Optional raw verification

若要重新確認卡面 bytes，可重跑先前的 page read probe，驗證 page 4 起出現 Type 2 TLV 與 `application/json` MIME record。

## Windows ARM64 Validation

目的：先確認 Windows on ARM 能否以標準 CCID / PC/SC 或 ACS 驅動走通基本讀卡流程；如讀卡穩定，再進一步觀察寫卡路徑。

### Prerequisites

- Windows ARM64 實機
- ACR1252U-M1
- 已安裝 Go、Node.js、pnpm
- 已安裝或可取得 ACS reader driver；若不需額外 driver，至少要確認 Windows 可辨識為 Smart Card / CCID 裝置

### 1. OS and driver check

- 插入 reader 後，先看裝置管理員是否出現 ACR1252U-M1 或對應 Smart Card Reader
- 確認 Windows `Smart Card` 服務已啟動
- 若裝置顯示為未知裝置，先停在 driver 問題，不進入 app 驗證

PowerShell:

```powershell
Get-Service SCardSvr
Get-PnpDevice | Where-Object { $_.FriendlyName -match 'ACS|ACR1252|Smart Card|CCID' }
```

Expected:

- `SCardSvr` 狀態為 `Running` 或至少可啟動
- 可找到對應 reader 裝置

### 2. Build artifacts

```powershell
pnpm install
pnpm run build
go build ./connector/cmd/nfc-connector
```

### 3. Start services

PowerShell Window A:

```powershell
pnpm dev
```

PowerShell Window B:

```powershell
$env:NFC_CONNECTOR_DRIVER = 'pcsc'
$env:NFC_CONNECTOR_ALLOWED_ORIGINS = 'http://localhost:3000'
$env:NFC_CONNECTOR_SHARED_SECRET = 'development-shared-secret'
go run ./connector/cmd/nfc-connector
```

Expected:

- 若 driver 可用，Connector 應正常 listen
- 若 driver 初始化失敗，記錄完整錯誤文字，這就是 2.3 的核心風險結果

### 4. Generate bridge tokens in PowerShell

```powershell
function Convert-ToBase64Url([byte[]] $bytes) {
  [Convert]::ToBase64String($bytes).TrimEnd('=') -replace '\+', '-' -replace '/', '_'
}

function New-BridgeToken([string] $scope) {
  $payloadObject = @{
    origin = 'http://localhost:3000'
    scope = $scope
    exp = [int][DateTimeOffset]::UtcNow.ToUnixTimeSeconds() + 120
  }
  $payloadJson = $payloadObject | ConvertTo-Json -Compress
  $payloadBytes = [System.Text.Encoding]::UTF8.GetBytes($payloadJson)
  $payload = Convert-ToBase64Url $payloadBytes
  $hmac = [System.Security.Cryptography.HMACSHA256]::new([System.Text.Encoding]::UTF8.GetBytes('development-shared-secret'))
  try {
    $signatureBytes = $hmac.ComputeHash([System.Text.Encoding]::UTF8.GetBytes($payload))
  }
  finally {
    $hmac.Dispose()
  }
  $signature = Convert-ToBase64Url $signatureBytes
  return "$payload.$signature"
}

$AllToken = New-BridgeToken 'all'
$ReadToken = New-BridgeToken 'read'
$WriteToken = New-BridgeToken 'write'
```

### 5. Run health, reader, and read validation

```powershell
Invoke-RestMethod http://127.0.0.1:42619/health

$headers = @{
  Origin = 'http://localhost:3000'
  'X-Bridge-Token' = $AllToken
}

$readers = Invoke-RestMethod -Headers $headers http://127.0.0.1:42619/readers
$readers

$session = Invoke-RestMethod -Method Post -Headers $headers -ContentType 'application/json' -Body '{"readerName":"ACS ACR1252 Dual Reader PICC"}' http://127.0.0.1:42619/session/connect
$session

$readHeaders = @{
  Origin = 'http://localhost:3000'
  'X-Bridge-Token' = $ReadToken
}

Invoke-RestMethod -Method Post -Headers $readHeaders -ContentType 'application/json' -Body (@{
  sessionId = $session.id
  operation = 'summary'
} | ConvertTo-Json -Compress) http://127.0.0.1:42619/card/read
```

Expected:

- `/health` 回 `status=ok`
- `/readers` 列得出 reader
- `/session/connect` 可建立 session
- `/card/read` 回 UID 與 ATR

### 6. Optional write probe after read success

只有在前一步全部穩定時才做：

```powershell
$writeHeaders = @{
  Origin = 'http://localhost:3000'
  'X-Bridge-Token' = $WriteToken
}

Invoke-RestMethod -Method Post -Headers $writeHeaders -ContentType 'application/json' -Body (@{
  sessionId = $session.id
  operation = 'ndef-v1'
  payload = @{
    version = 1
    type = 'web-nfc-bridge/demo'
    label = 'Windows ARM64 probe'
    updatedAt = '2026-03-09T01:35:40Z'
  }
} | ConvertTo-Json -Depth 5 -Compress) http://127.0.0.1:42619/card/write
```

Interpretation:

- 若 read 成功但 write 失敗，先記錄失敗訊息，不代表 2.3 失敗；2.3 的最低門檻是 driver / PCSC 基本可操作
- 若連 `/readers` 都拿不到，優先回頭檢查 driver / Smart Card service

## Result Recording Template

每次驗證請至少記錄：

- Platform / architecture
- Reader driver 狀態
- `/health` 結果
- `/readers` 結果
- `/session/connect` 結果
- `/card/read` 結果
- `/card/write` 結果（若有做）
- 任何錯誤原文

## Current Execution Plan

- 先做 macOS regression check，確認目前已完成能力仍穩定
- 再做 Windows ARM64 driver risk validation
- Windows x64 與 Linux 先保留在 pending validation
