# Project Context

## Project Goal

建立一套可透過 HTTPS 網站操作的 NFC 讀寫產品，硬體為 ACR1252U-M1。使用者體驗目標是在首次安裝本機 Connector 後，未來只需打開網站即可完成讀卡與寫卡。

## Constraints

- 需要支援 Windows
- 需要支援 Windows on ARM
- 需要支援 macOS
- 需要支援 Linux
- 純瀏覽器直接操作 USB 讀卡機不可作為正式主路徑

## Preferred Stack

- Web frontend: `Nuxt` + `Nuxt UI`
- Local connector: Go
- Reader communication: PC/SC
- Website-to-connector transport: localhost HTTP + WebSocket

## Architecture Decision

- 正式產品路徑採用 HTTPS 網站 + localhost Connector + PC/SC
- `Web NFC` 與 `WebUSB` 僅保留為研究結論，不作為正式主路徑
- Web 前端先在 repo root 建立 `Nuxt` + `Nuxt UI` 應用程式
- localhost Connector 契約先以 `http://127.0.0.1:42619` 為暫定 base URL

## Product Principles

- 以最低摩擦為優先，但不以不穩定的瀏覽器硬體 API 換取表面上的零安裝
- 寫卡與敏感操作必須有清楚的授權與安全邊界
- 平台支援承諾必須建立在實機驗證之上，特別是 Windows ARM64
