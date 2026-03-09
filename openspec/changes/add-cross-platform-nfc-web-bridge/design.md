## Context

產品需求是讓使用者使用外接 USB NFC 讀寫器完成讀卡與寫卡，並盡可能接近「打開網址即可使用」的體驗。硬體為 ACR1252U-M1，網站技術方向為 `Nuxt` + `Nuxt UI`。

研究結論如下：

- `Web NFC` 不足以支援桌面瀏覽器搭配外接 USB 讀卡機的主要場景
- `WebUSB` 僅在部分 Chromium 瀏覽器可用，且不適合作為依賴 PC/SC 驅動的產品主路徑
- 雲端網站無法直接安全、穩定地操作使用者電腦上的 USB NFC 讀卡機
- 真正可交付且跨平台的方案，是由網站負責 UI 與流程，本機 Connector 負責與讀卡機溝通

## Goals / Non-Goals

**Goals:**

- 支援 Windows、Windows ARM64、macOS、Linux
- 把使用者操作摩擦壓到第一次安裝之後只需打開 HTTPS 網站
- 讓前端與本機硬體整合有明確、可版本化的通訊契約
- 將平台差異、權限、安全與更新機制納入正式設計

**Non-Goals:**

- 不追求完全零安裝的純網站方案
- 不以 `WebUSB` 或 `Web NFC` 當正式產品主路徑
- 不在前端直接暴露原始 APDU 直通能力
- 不在本 change 中綁定特定雲端後端供應商

## Decisions

### Decision 1: 產品架構採用 HTTPS 網站 + localhost Connector + PC/SC

前端網站部署在 HTTPS 網域，負責登入、狀態顯示、業務流程與錯誤提示。本機 Connector 在使用者電腦上常駐，透過 PC/SC 與 ACR1252U-M1 通訊，並以 localhost HTTP / WebSocket 對網站提供能力。

這個決策把硬體整合從瀏覽器 API 抽離，避免受限於瀏覽器對 `Web NFC`、`WebUSB` 的支援落差，也讓產品邏輯可以維持單一 Web UI。

### Decision 2: 本機 Connector 使用 Go 實作

Go 適合編譯成小型單一執行檔，便於針對下列平台發行：

- Windows x64
- Windows ARM64
- macOS Apple Silicon
- macOS Intel
- Linux x64

若後續需要，也可再擴充 Linux ARM64。Go 也適合實作常駐程序、localhost API、WebSocket 與自動更新流程。

### Decision 3: Web 前端使用 Nuxt + Nuxt UI

前端網站採用 `Nuxt` + `Nuxt UI`，負責以下職責：

- 使用者登入與工作流程
- Connector 偵測與安裝引導
- 讀卡機狀態與卡片狀態顯示
- 讀卡、寫卡、驗證等操作入口
- 與雲端後端互動以取得授權與業務規則

### Decision 4: Connector 對外提供 HTTP + WebSocket 雙通道

HTTP 適合一次性操作，例如列出讀卡機、建立工作階段、執行讀卡命令。WebSocket 適合事件推播，例如讀卡機接入、卡片放上去、卡片移開、寫卡完成、錯誤通知。

建議最小契約如下：

- `GET /health`
- `GET /readers`
- `POST /session/connect`
- `POST /card/read`
- `POST /card/write`
- `GET /card/poll` 或 `WS /events`

正式實作以 `WS /events` 為主，HTTP 為控制介面。

### Decision 5: 安全邊界在 Connector 層強制執行

Connector 不是通用硬體代理，而是受控產品元件。它必須：

- 驗證請求來源網域
- 驗證網站傳來的短效授權 token
- 拒絕任意 APDU 直通
- 要求敏感寫卡操作由明確使用者動作觸發
- 只暴露受產品流程控制的高階命令

這個決策是為了避免 localhost 服務被其他網站濫用。

### Decision 6: Windows on ARM 驅動驗證列為先行風險排除工作

跨平台目標中，最不確定的是 Windows ARM64 是否能以標準 CCID / PC/SC 或 ACS 提供的驅動穩定支援 ACR1252U-M1。這必須在正式實作前優先驗證，若該平台存在驅動缺口，需在產品支援矩陣中明示。

## Architecture

```text
Browser (Nuxt + Nuxt UI)
        |
        | HTTPS
        v
Cloud API / Auth / Business Rules

Browser (same page)
        |
        | localhost HTTP / WebSocket
        v
Local Connector (Go)
        |
        | PC/SC
        v
ACR1252U-M1 / NFC Card
```

## User Flow

1. 使用者打開 HTTPS 網站。
2. 前端嘗試連線本機 Connector。
3. 若未安裝 Connector，前端顯示平台對應安裝引導。
4. Connector 安裝後可設定開機啟動。
5. 後續使用者只需重新打開網站。
6. 網站顯示讀卡機狀態、卡片狀態與可執行操作。
7. 寫卡等敏感操作需經雲端授權與使用者明確確認。

## Risks / Trade-offs

### 1. 不是完全零安裝

這個方案仍需要第一次安裝本機 Connector。但相較於要求固定瀏覽器、手動授權 USB、處理瀏覽器不相容，這是最低且可控的摩擦。

### 2. 驅動差異是主要平台風險

不同作業系統對 CCID / PC/SC 的支援程度不同，尤其是 Windows ARM64。產品支援承諾必須以實機驗證為準。

### 3. localhost 安全性需要額外設計

本機服務一旦暴露不當，會成為攻擊面。這要求 Connector 必須從第一版起就內建 origin 與 token 驗證，而不是後補。

## Rollout Plan

### Phase 1: 技術驗證

- 驗證 macOS 上透過 PC/SC 存取 ACR1252U-M1
- 驗證 Windows x64 與 Windows ARM64 的讀卡機與驅動可用性
- 驗證 Linux 的基本存取路徑

### Phase 2: Connector Contract

- 定義 localhost API 與事件模型
- 定義錯誤碼與狀態機
- 建立最小可用 Connector 原型

### Phase 3: Web App

- 建立 Nuxt + Nuxt UI 網站骨架
- 實作 Connector 偵測、狀態頁、讀寫流程 UI
- 與雲端授權流程串接

### Phase 4: 安裝與更新

- 建立各平台安裝包
- 加入自動更新策略
- 完成正式支援矩陣與安裝引導文件
