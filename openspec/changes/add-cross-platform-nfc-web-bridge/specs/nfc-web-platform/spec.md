# Delta for nfc-web-platform

## ADDED Requirements

### Requirement: Local Connector Architecture

系統 SHALL 以 HTTPS 網站搭配本機 Connector 的方式提供外接 USB NFC 讀寫器能力，而不是依賴瀏覽器直接操作 USB 裝置。

#### Scenario: Opening the product from a supported desktop browser

- **GIVEN** 使用者已安裝本機 Connector
- **AND** 使用者使用支援的桌面瀏覽器打開產品 HTTPS 網站
- **WHEN** 網站需要與 NFC 讀寫器互動
- **THEN** 網站 MUST 透過 localhost 通訊與本機 Connector 交換資料
- **AND** 本機 Connector MUST 透過 PC/SC 與 ACR1252U-M1 溝通

#### Scenario: Avoiding browser-native USB dependency as the primary path

- **GIVEN** 產品需要支援 Windows、Windows ARM64、macOS、Linux
- **WHEN** 定義正式產品架構
- **THEN** 系統 MUST NOT 以 `Web NFC` 或 `WebUSB` 作為正式主路徑

### Requirement: Cross-Platform Support Matrix

系統 SHALL 以跨平台本機 Connector 為核心，支援多個桌面作業系統與架構，並明確定義支援矩陣。

#### Scenario: Defining target platforms

- **WHEN** 專案建立正式支援矩陣
- **THEN** 目標平台 MUST 包含 Windows x64、Windows ARM64、macOS 與 Linux
- **AND** 每個平台 MUST 經過實機驗證後才可標示為正式支援

#### Scenario: Handling uncertain driver support

- **GIVEN** 某平台對 ACR1252U-M1 的驅動或 PC/SC 支援尚未驗證完成
- **WHEN** 產品文件或安裝頁面顯示支援資訊
- **THEN** 系統 MUST 將該平台標示為待驗證或受限支援
- **AND** MUST NOT 對外宣稱完全支援

### Requirement: Low-Friction User Experience

系統 SHALL 把使用者摩擦控制在首次安裝 Connector 後，後續只需打開 HTTPS 網站即可使用。

#### Scenario: First-time setup

- **GIVEN** 使用者尚未安裝本機 Connector
- **WHEN** 使用者打開產品網站
- **THEN** 網站 MUST 能偵測 Connector 尚未可用
- **AND** MUST 顯示對應平台的安裝引導

#### Scenario: Returning user flow

- **GIVEN** 使用者已安裝本機 Connector
- **WHEN** 使用者再次打開產品網站
- **THEN** 網站 MUST 自動偵測 Connector 狀態
- **AND** 若 Connector 與讀卡機皆正常可用，使用者 SHALL 可直接進行讀卡或寫卡流程

### Requirement: Localhost Security Controls

本機 Connector SHALL 只接受受信任網站與受控命令，避免 localhost 服務成為任意網站可濫用的硬體入口。

#### Scenario: Accepting requests from the product website

- **GIVEN** 產品網站已完成登入與授權流程
- **WHEN** 網站向本機 Connector 發出讀卡或寫卡請求
- **THEN** Connector MUST 驗證來源網域與授權 token
- **AND** 只有通過驗證的請求才可執行

#### Scenario: Rejecting raw hardware passthrough

- **WHEN** 前端或其他網站嘗試發送任意 APDU 或未定義命令到本機 Connector
- **THEN** Connector MUST 拒絕該請求
- **AND** MUST 只暴露明確定義的高階操作介面

### Requirement: Bidirectional Website-Connector Contract

系統 SHALL 提供明確的 localhost API 與事件通道，讓網站可取得即時硬體狀態。

#### Scenario: Querying current health and readers

- **WHEN** 網站初始化與本機 Connector 建立連線
- **THEN** Connector MUST 提供健康檢查與讀卡機列表查詢能力

#### Scenario: Receiving realtime card events

- **GIVEN** 使用者已打開網站並與 Connector 建立事件連線
- **WHEN** 卡片被放上讀卡機、移開、讀取完成或寫入完成
- **THEN** Connector MUST 以事件通道主動通知網站
- **AND** 網站 MUST 能根據事件更新使用者介面
