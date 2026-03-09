# Release Process

本專案使用 GitHub Actions 產出可安裝的 release 資產，目標平台如下：

- Windows x64：`.msi`
- Windows ARM64：`.msi`
- macOS Apple Silicon：`.pkg`
- Ubuntu x64：`.deb`

## 發版方式

使用 Git tag 作為正式發版來源。

1. 確認 `package.json` 的版本已更新。
2. 建立 tag，例如 `v0.1.10`。
3. 推送 tag 到 GitHub。
4. GitHub Actions 執行 `release-installers` workflow。
5. 安裝包會附加到對應的 GitHub Release。

## Workflows

- `ci`：在 pull request 與 `main` push 時執行一般驗證。
- `release-installers`：在 `v*` tag push 時建置安裝包並發佈 release。

## 本機指令

- `pnpm run build:app`：只建置 Nuxt 應用程式。
- `pnpm run connector:test`：執行 connector 測試。
- `pnpm run ci`：執行本機 CI 等級驗證。
- `pnpm run build`：建置應用程式並產出本機可用安裝包。

## 簽章狀態

目前 release 流程會產出可安裝的安裝包，但尚未包含正式簽章流程。

- Windows：未做 Authenticode 簽章時，可能出現 SmartScreen 警告。
- macOS：未做 Developer ID 簽章與 notarization 時，可能出現 Gatekeeper 警告。
- Ubuntu：`.deb` 可安裝，但若未搭配額外套件來源簽署，不等同於 repository trust chain。

若後續要對外正式發佈，建議新增：

1. Windows code signing。
2. macOS Developer ID signing。
3. macOS notarization。
