---
date: "2016-11-08T16:00:00+02:00"
title: "文件"
slug: /
sidebar_position: 10
toc: false
draft: false
---

# 關於 Gitea

Gitea 是一個可自行託管的 Git 服務。你可以拿 GitHub、Bitbucket 或 Gitlab 來比較看看。
Gitea 是從 [Gogs](http://gogs.io) Fork 出來的，請閱讀部落格文章 [Gitea 公告](https://blog.gitea.com/welcome-to-gitea/)以了解我們 Fork 的理由。

## 目標

本專案的首要目標是建立一個容易安裝，執行快速，安装和使用體驗良好的自建 Git 服務。我們採用 GO 為後端語言，Go 可以產生各平台使用的執行檔。它支援 Linux、macOS 和 Windows 外，處理器架構包含 amd64、i386、ARM 和 PowerPC 等。

## 功能

- 代碼託管：Gitea 支援建立和管理存儲庫、瀏覽提交歷史和程式碼檔案、審查和合併程式碼提交、管理協作者、處理分支等。它還支援許多常見的 Git 功能，如標籤、Cherry-pick、鉤子、集成協作工具等。

- 輕量級和快速：Gitea 的設計目標之一就是輕量級和快速響應。與某些大型代碼託管平台不同，它保持了精簡，在速度方面表現出色，適用於資源有限的伺服器環境。由於其輕量級設計，Gitea 的資源消耗相對較低，在資源受限的環境中表現出色。

- 易於部署和維護：它可以輕鬆地部署在各種伺服器上，無需複雜的配置或依賴。這使得個人開發者或小團隊可以方便地設置和管理自己的 Git 服務。

- 安全性：Gitea 強調安全性，提供用戶權限管理、訪問控制列表等功能，確保程式碼和數據的安全性。

- 代碼審查：代碼審查同時支援拉取請求工作流和 AGit 工作流。審查者可以在線瀏覽程式碼並提供審查意見或反饋。提交者可以接收審查意見並在線回覆或修改程式碼。代碼審查可以幫助個人和組織提升程式碼質量。

- CI/CD：Gitea Actions 支援 CI/CD 功能，與 GitHub Actions 相容。用戶可以使用熟悉的 YAML 格式編寫工作流程，並重複使用各種現有的 Actions 插件。Actions 插件支援從任何 Git 網站下載。

- 專案管理：Gitea 通過看板和工單來追蹤一個專案的需求、功能和錯誤。工單支援分支、標籤、里程碑、指派、時間追蹤、到期日期、依賴關係等功能。

- 制品庫：Gitea 支援超過 20 種不同類型的公有或私有軟體包管理，包括：Cargo、Chef、Composer、Conan、Conda、Container、Helm、Maven、npm、NuGet、Pub、PyPI、RubyGems、Vagrant 等。

- 開源社區支援：Gitea 是一個基於 MIT 許可證的開源專案，擁有活躍的開源社區，能夠持續進行開發和改進，同時也積極接受社區貢獻，保持了平台的更新和創新。

- 多語言支援：Gitea 提供多種語言界面，適應全球範圍內的用戶，促進了國際化和本地化。

更多功能特性：詳見：https://docs.gitea.com/installation/comparison#general-features

## 系統需求

- Raspberry Pi 3 的效能足夠讓 Gitea 承擔小型工作負載。
- 雙核心 CPU 和 1GB 記憶體通常足以應付小型團隊/專案。
- 在類 UNIX 系統上， 應該以專用的非 root 系統帳號來執行 Gitea。
  - 備註：Gitea 管理著 `~/.ssh/authorized_keys` 檔案。以一般身份使用者執行 Gitea 可能會破壞該使用者的登入能力。

- [Git](https://git-scm.com/) 的最低需求為 2.0 或更新版本。
  - 當 git 版本 >= 2.1.2 時，可啟用 Git [large file storage](https://git-lfs.github.com/)。
  - 當 git 版本 >= 2.18 時，將自動啟用 Git 提交線圖渲染。

## 瀏覽器支援

- 最近 2 個版本的 Chrome, Firefox, Safari, Edge
- Firefox ESR

## 元件

- Web 框架： [Chi](http://github.com/go-chi/chi)
- ORM： [XORM](https://xorm.io)
- UI 元件：
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
  - ... (package.json)
- 資料庫驅動程式：
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## 集成支持

請訪問 [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/) 獲得更多的第三方集成支持
