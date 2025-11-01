# Gitea

[![](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml/badge.svg?branch=main)](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml?query=branch%3Amain "Release Nightly")
[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")
[![](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea "Go Report Card")
[![](https://pkg.go.dev/badge/code.gitea.io/gitea?status.svg)](https://pkg.go.dev/code.gitea.io/gitea "GoDoc")
[![](https://img.shields.io/github/release/go-gitea/gitea.svg)](https://github.com/go-gitea/gitea/releases/latest "GitHub 版本發布")
[![](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea "協助貢獻開源專案")
[![](https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen)](https://opencollective.com/gitea "成為 gitea 的支持者/贊助商")
[![](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT "License: MIT")
[![使用 Gitpod 貢獻](https://img.shields.io/badge/Contribute%20with-Gitpod-908a85?logo=gitpod&color=green)](https://gitpod.io/#https://github.com/go-gitea/gitea)
[![](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com "Crowdin")

[English](./README.md) | [繁體中文](./README.zh-tw.md)

## 專案目標
本專案的核心目標，是讓自建 Git 服務的過程，變得最簡單、最高效、最省心。

Gitea 基於 Go 語言開發，凡 Go 語言支持的平台與架構，它皆能適配，涵蓋 Linux、macOS、Windows 系統，以及 x86、amd64、ARM、PowerPC 架構。專案自 2016 年 11 月從[Gogs](https://gogs.io)[分叉](https://blog.gitea.com/welcome-to-gitea/)而來，如今已是煥然一新。

- 線上體驗：造訪[demo.gitea.com](https://demo.gitea.com)
- 免費服務（儲存庫數量有限）：造訪[gitea.com](https://gitea.com/user/login)
- 快速部署專屬實例：前往[cloud.gitea.com](https://cloud.gitea.com)開啟免費試用


## 官方文件
你可在[官方文件網站](https://docs.gitea.com/)取得完整文件，內容涵蓋安裝部署、管理維護、使用指南、開發貢獻等，助你快速上手並充分探索所有功能。

若有建議或想參與文檔編寫，可造訪[文檔倉庫](https://gitea.com/gitea/docs)。


## 建構方法
進入原始碼根目錄，執行以下命令建構：

```
TAGS="bindata" make build
```

若需支持 SQLite 資料庫，執行：

  ```
TAGS="bindata sqlite sqlite_unlock_notify" make build
```

`build`目標分為兩個子目標：

- `make backend`：需依賴[Go Stable](https://go.dev/dl/)，具體版本見[go.mod](/go.mod)
- `make frontend`：需依賴[Node.js LTS](https://nodejs.org/en/download/)（及以上版本）和[pnpm](https://pnpm.io/installation)

構建需聯網以下載 Go 和 npm 依賴套件。若使用包含預構建前端檔案的官方原始碼壓縮包，無需觸發`frontend`目標，無 Node.js 環境也可完成構建。

更多細節：[https://docs.gitea.com/installation/install-from-source](https://docs.gitea.com/installation/install-from-source)


## 使用方法
構建完成後，原始碼根目錄預設會產生 `gitea` 可執行檔案，執行命令：

```
./gitea web
```

> [!NOTE]
> 若需調用 API，我們已提供實驗性支援，文件詳見[此處](https://docs.gitea.com/api)。


## 貢獻指南
標準流程：Fork → Patch → Push → Pull Request

> [!NOTE]
> 1. 提交 Pull Request 前，務必閱讀[《貢獻者指南》](CONTRIBUTING.md)！
> 2. 若發現項目漏洞，請通過郵件**security@gitea.io**私信反饋，感謝你的嚴謹！


## 多語言翻譯
翻譯工作透過 [Crowdin](https://translate.gitea.com) 進行。若需新增翻譯語言，可聯絡 Crowdin 專案管理員新增；也可提交 issue 申請，或在 Discord 的 #translation 頻道諮詢。

若需翻譯上下文或發現翻譯問題，可在對應文本下留言或透過 Discord 溝通。文件設有翻譯相關專區（目前內容待補充），將根據問題逐步完善。

更多資訊：[翻譯貢獻文件](https://docs.gitea.com/contributing/localization)


## 官方及第三方專案

- 官方工具：[go-sdk](https://gitea.com/gitea/go-sdk)、命令列工具[tea](https://gitea.com/gitea/tea)、Gitea Action 專用[執行器](https://gitea.com/gitea/act_runner)
- 第三方專案清單：[gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea)，含 SDK、外掛程式、主題等資源


## 交流頻道

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")


若文件未涵蓋你的問題，可透過[Discord 伺服器](https://discord.gg/Gitea)聯絡我們，或在[論壇](https://forum.gitea.com/)發布貼文。


## 專案成員
- [維護者](https://github.com/orgs/go-gitea/people)
- [貢獻者](https://github.com/go-gitea/gitea/graphs/contributors)
- [譯者](options/locale/TRANSLATORS)


## 支持者
感謝所有支持者的鼎力相助！🙏 [[成為支持者](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## 贊助商
成為贊助商支持專案，你的 logo 將在此展示並連結至官網。[[成為贊助商](https://opencollective.com/gitea#sponsor)]

<a href="https://opencollective.com/gitea/sponsor/0/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/1/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/2/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/3/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/4/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/5/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/6/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/7/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/8/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/gitea/sponsor/9/website" target="_blank"><img src="https://opencollective.com/gitea/sponsor/9/avatar.svg"></a>

## 常見問題
### Q：Gitea 如何發音？
A：發音為[/ɡɪ'ti:/](https://youtu.be/EM71-2uDAoY)，類似「gi-tea」，「g」需發重音。

### Q：為何專案代碼未託管在 Gitea 自身實例上？
A：我們正推進此事，進展可查看[該 issue](https://github.com/go-gitea/gitea/issues/1029)。

### Q：哪裡可找到安全補丁？
A：在[發布日誌](https://github.com/go-gitea/gitea/releases)或[更新日誌](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md)中，搜尋關鍵詞`SECURITY`即可找到。


## 授權條款
本項目採用 MIT 授權條款，完整授權文本詳見 [LICENSE 檔案](https://github.com/go-gitea/gitea/blob/main/LICENSE)。


## 更多資訊
<details>
<summary>尋找介面概述？查看這裡！</summary>

### 登入/註冊頁面

![Login](https://dl.gitea.com/screenshots/login.png)
![Register](https://dl.gitea.com/screenshots/register.png)

### 使用者儀表板

![首頁](https://dl.gitea.com/screenshots/home.png)
![議題](https://dl.gitea.com/screenshots/issues.png)
![拉取請求](https://dl.gitea.com/screenshots/pull_requests.png)
![里程碑](https://dl.gitea.com/screenshots/milestones.png)

### 使用者資料

![Profile](https://dl.gitea.com/screenshots/user_profile.png)

### 探索

![Repos](https://dl.gitea.com/screenshots/explore_repos.png)
![使用者](https://dl.gitea.com/screenshots/explore_users.png)
![組織](https://dl.gitea.com/screenshots/explore_orgs.png)

### 儲存庫

![首頁](https://dl.gitea.com/screenshots/repo_home.png)
![提交](https://dl.gitea.com/screenshots/repo_commits.png)
![分支](https://dl.gitea.com/screenshots/repo_branches.png)
![標籤](https://dl.gitea.com/screenshots/repo_labels.png)
![里程碑](https://dl.gitea.com/screenshots/repo_milestones.png)
![發行版本](https://dl.gitea.com/screenshots/repo_releases.png)
![標籤](https://dl.gitea.com/screenshots/repo_tags.png)

#### 儲存庫議題

![清單](https://dl.gitea.com/screenshots/repo_issues.png)
![議題](https://dl.gitea.com/screenshots/repo_issue.png)

#### 儲存庫提取請求

![清單](https://dl.gitea.com/screenshots/repo_pull_requests.png)
![提取請求](https://dl.gitea.com/screenshots/repo_pull_request.png)
![File](https://dl.gitea.com/screenshots/repo_pull_request_file.png)
![Commits](https://dl.gitea.com/screenshots/repo_pull_request_commits.png)

#### 儲存庫操作

![List](https://dl.gitea.com/screenshots/repo_actions.png)
![詳細資訊](https://dl.gitea.com/screenshots/repo_actions_run.png)

#### 儲存庫活動

![活動](https://dl.gitea.com/screenshots/repo_activity.png)
![貢獻者](https://dl.gitea.com/screenshots/repo_contributors.png)
![程式碼頻率](https://dl.gitea.com/screenshots/repo_code_frequency.png)
![最近的提交](https://dl.gitea.com/screenshots/repo_recent_commits.png)

### 組織

![首頁](https://dl.gitea.com/screenshots/org_home.png)

</details>
