# Implementation Tasks

## 0. Traceability Checkpoints

- [x] 0.1 將 Requirement: Local Connector Architecture 對應到網站、Connector 與 PC/SC 的實作邊界
- [x] 0.2 將 Requirement: Cross-Platform Support Matrix 對應到平台驗證與支援政策
- [x] 0.3 將 Requirement: Low-Friction User Experience 對應到首次安裝與回訪流程
- [x] 0.4 將 Requirement: Localhost Security Controls 對應到 origin、token 與命令邊界
- [x] 0.5 將 Requirement: Bidirectional Website-Connector Contract 對應到 HTTP API 與 WebSocket 事件
- [x] 0.6 將 Decision 1: 產品架構採用 HTTPS 網站 + localhost Connector + PC/SC 對應到整體系統分層
- [x] 0.7 將 Decision 2: 本機 Connector 使用 Go 實作 對應到 Connector 專案骨架與發行策略
- [x] 0.8 將 Decision 3: Web 前端使用 Nuxt + Nuxt UI 對應到 Web App 骨架與 UI 流程
- [x] 0.9 將 Decision 4: Connector 對外提供 HTTP + WebSocket 雙通道 對應到 localhost 契約
- [x] 0.10 將 Decision 5: 安全邊界在 Connector 層強制執行 對應到安全驗證與高階命令設計
- [x] 0.11 將 Decision 6: Windows on ARM 驅動驗證列為先行風險排除工作 對應到平台驗證優先順序

## 1. Decision 1 / Requirement: Local Connector Architecture

- [x] 1.1 建立 `openspec/project.md`，記錄 HTTPS 網站 + localhost Connector + PC/SC 的正式架構決策
- [x] 1.2 建立 `nfc-web-platform` capability 的主規格來源檔案，作為未來 archive 後的 source of truth
- [x] 1.3 確認 change 命名、capability 命名與未來程式碼目錄命名一致
- [x] 1.4 在技術文件中明確記錄 `Web NFC` 與 `WebUSB` 不作為正式主路徑

## 2. Decision 6 / Requirement: Cross-Platform Support Matrix / Phase 1: 技術驗證

- [x] 2.1 在 macOS 驗證 ACR1252U-M1 是否可透過 PC/SC 正常連線與收發指令
- [ ] 2.2 在 Windows x64 驗證讀卡機驅動、PC/SC 與基本讀卡流程
- [ ] 2.3 在 Windows ARM64 驗證是否可使用標準 CCID / PC/SC 或廠商驅動正常操作
- [ ] 2.4 在 Linux 驗證 pcsc-lite 與讀卡機的基本相容性
- [x] 2.5 建立平台支援矩陣與已知限制清單
- [x] 2.6 把「驅動差異是主要平台風險」寫入支援政策與驗證報告

## 3. Decision 4 / Requirement: Bidirectional Website-Connector Contract / Phase 2: Connector Contract

- [x] 3.1 定義 localhost HTTP API：health、readers、session、read、write
- [x] 3.2 定義 WebSocket 事件格式：reader status、card present、card removed、read complete、write complete、error
- [x] 3.3 定義錯誤碼、狀態機與 reconnect 行為
- [x] 3.4 定義禁止任意 APDU 直通的命令邊界

## 4. Decision 2 / Decision 5 / Requirement: Localhost Security Controls

- [x] 4.1 建立 Go Connector 專案骨架
- [x] 4.2 實作讀卡機列舉、連線與卡片事件監聽
- [x] 4.3 實作受控的讀卡與寫卡命令
- [x] 4.4 實作 localhost API 與 WebSocket 服務
- [x] 4.5 實作 origin 驗證、token 驗證與敏感操作保護
- [x] 4.6 在 Connector 設計中落實「localhost 安全性需要額外設計」這項風險控管

## 5. Decision 3 / Requirement: Low-Friction User Experience / Phase 3: Web App

- [x] 5.1 建立 `Nuxt` + `Nuxt UI` 前端骨架
- [x] 5.2 實作 Connector 偵測與安裝引導頁
- [x] 5.3 實作讀卡機狀態、卡片狀態與操作流程 UI
- [x] 5.4 串接 Connector HTTP / WebSocket 通訊
- [x] 5.5 串接雲端授權與業務流程
- [x] 5.6 在產品流程中明確處理「不是完全零安裝」的首次安裝引導

## 6. Phase 4: 安裝與更新

- [x] 6.1 規劃 Windows x64 / ARM64、macOS、Linux 的安裝包格式
- [x] 6.2 規劃 Connector 開機自啟與自動更新策略
- [x] 6.3 撰寫使用者安裝與故障排除文件
- [x] 6.4 建立最低支援瀏覽器與平台政策
