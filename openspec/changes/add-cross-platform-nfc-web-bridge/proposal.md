## Why

專案目標是讓使用者透過 ACR1252U-M1 這類 USB NFC 讀寫器進行卡片讀寫，同時把操作摩擦降到最低，理想體驗是使用者未來只要打開 HTTPS 網站就能完成工作。

研究結果顯示，桌面瀏覽器無法可靠地以純網頁方式直接控制這類 USB 讀卡機。`Web NFC` 主要適用於 Android 裝置的內建 NFC，桌面瀏覽器支援不足；`WebUSB` 雖然在部分 Chromium 瀏覽器可用，但 Safari、Firefox 不支援，而且 ACR1252U-M1 這類裝置通常依賴 PC/SC 或 CCID 驅動模型，不適合把瀏覽器直連 USB 當成產品主路徑。

如果專案要同時支援 Windows、Windows on ARM、macOS、Linux，並維持最低使用者摩擦，必須在「純網站」與「可實際交付」之間做出清楚的架構決策。

## What Changes

- 定義本產品的正式架構為「HTTPS 網站 + 本機 Connector + PC/SC」
- 明確排除以 `Web NFC` 或 `WebUSB` 作為正式產品主路徑
- 為前端選定 `Nuxt` + `Nuxt UI`，承載使用者操作流程、狀態顯示與授權互動
- 為本機 Connector 選定 Go 作為主要實作語言，以便覆蓋 Windows x64、Windows ARM64、macOS、Linux
- 定義網站與本機 Connector 之間的 localhost 通訊契約與安全邊界
- 把跨平台驅動與相容性驗證，特別是 Windows on ARM，列為正式交付前的必要驗證項目

## Capabilities

### New Capabilities

- `nfc-web-platform`: 定義跨平台 NFC Web 產品的架構、通訊模式、安全要求與最低摩擦使用流程

### Modified Capabilities

## Impact

- Affected specs: `nfc-web-platform`
- Affected code: 未來將新增 Nuxt Web 前端、本機 Go Connector、安裝與更新流程、平台驗證腳本
- Affected systems:
  - HTTPS Web 應用程式
  - 本機常駐 Connector
  - USB NFC 讀寫器與 PC/SC 堆疊
  - 使用者裝置上的安裝、啟動、更新流程
- Key risks:
  - Windows on ARM 對 ACR1252U-M1 的驅動支援需要優先驗證
  - 本機 Connector 若缺乏 origin 驗證與授權設計，會造成 localhost 安全風險
