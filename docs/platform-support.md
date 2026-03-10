# Platform Support Matrix

## Validation Priority

1. macOS
2. Windows ARM64
3. Windows x64
4. Linux

Decision 6: Windows on ARM 驅動驗證列為先行風險排除工作。

## Current Validation Window

- 目前已可立即執行：macOS regression check、Windows ARM64 風險驗證
- 延後執行：Windows x64、Linux
- 在 Windows ARM64 完成前，不調整其 high-risk 標記

## Support Matrix

| Platform | Architecture          | Status                 | Notes                                                                                                   |
| -------- | --------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------- |
| Windows  | x64                   | Pending validation     | 需驗證 PC/SC 與基本讀卡流程                                                                             |
| Windows  | ARM64                 | High-risk validation   | 需先確認 CCID / PCSC 或 ACS 驅動可用性                                                                  |
| macOS    | Apple Silicon / Intel | Validated (read/write) | 已驗證列 reader、建立 session、讀 UID/ATR，並可對 NDEF formatted Type 2 tag 執行受控 `ndef-v1` 實體寫入 |
| Linux    | x64                   | Pending validation     | 需驗證 pcsc-lite 與裝置相容性                                                                           |

## Known Risks

- Requirement: Cross-Platform Support Matrix
  - 所有平台支援聲明都必須建立在實機驗證之上
- 驅動差異是主要平台風險
  - Windows ARM64 為最優先排雷目標
- 未驗證完成前，不可對外宣稱完整支援

## Latest Validation Snapshot

- macOS 2026-03-09：`ACS ACR1252 Dual Reader PICC` 可成功建立 session 並讀得 UID `0472650DCC2A81` 與 ATR `3B8F8001804F0CA0000003060300030000000068`
- macOS 2026-03-09：`/card/write` 以 `ndef-v1` demo payload 對真卡回傳 `accepted: true`，Connector details 包含 `driver=pcsc`、`profile=ndef-write-profile/v1`、`payloadType=web-nfc-bridge/demo`、`pagesWritten=30`
- macOS 2026-03-09：實體讀回 page 4 起的資料可見 Type 2 TLV 與 NDEF MIME record 前綴：`03 74 D2 10 61 61 70 70 6C 69 63 61 74 69 6F 6E ...`，代表卡片已寫入 `application/json` payload

## Browser Policy

- 正式支援的前提是瀏覽器可穩定連線到 localhost Connector
- 不以 `Web NFC` 或 `WebUSB` 作為支援基礎

## Planned Installer Targets

- Local build always emits four downloadable artifacts
- Default version comes from git tag or `package.json` version when available
- macOS: PKG
- Windows x64: MSI on Windows hosts with `wix`
- Windows ARM64: MSI on Windows hosts with `wix`
- Linux x64: Debian package for Ubuntu / `dpkg`
- Ubuntu `.deb` installs a system-level `systemd` service and starts it automatically
