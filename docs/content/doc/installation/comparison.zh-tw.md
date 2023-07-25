---
date: "2018-05-07T13:00:00+02:00"
title: "比較 Gitea 和其它自託管 Git 服務"
slug: "comparison"
weight: 5
toc: false
draft: false
aliases:
  - /zh-tw/comparison
menu:
  sidebar:
    parent: "installation"
    name: "比較"
    weight: 5
    identifier: "comparison"
---

# 比較 Gitea 和其它自託管 Git 服務

**目錄**

{{< toc >}}

為了幫助您判斷 Gitea 是否適合您的需求，這裡列出了它和其它自託管 Git 服務的比較。

請注意我們不會經常檢查其它產品的功能異動，所以這份清單可能過期，如果您在下方表格中找到需要更新的資料，請在 [GitHub 的 Issue](https://github.com/go-gitea/gitea/issues) 回報。

表格中使用的符號：

- ✓ - 支援

- ⁄ - 有限度的支援

- ✘ - 不支援

- _⚙️ - 由第三方服務或外掛程式支援_

## 一般功能

| 功能                     | Gitea                                              | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ------------------------ | -------------------------------------------------- | ---- | --------- | --------- | --------- | --------- | ------------ |
| 免費及開放原始碼         | ✓                                                  | ✓    | ✘         | ✓         | ✘         | ✘         | ✓            |
| 低資源使用 (RAM/CPU)     | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘         | ✘            |
| 支援多種資料庫           | ✓                                                  | ✓    | ✘         | ⁄         | ⁄         | ✓         | ✓            |
| 支援多種作業系統         | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘         | ✓            |
| 簡單的升級程序           | ✓                                                  | ✓    | ✘         | ✓         | ✓         | ✘         | ✓            |
| 支援 Markdown            | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 支援 Orgmode             | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✘         | ?            |
| 支援 CSV                 | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✓         | ?            |
| 支援第三方渲染工具       | ✓                                                  | ✘    | ✘         | ✘         | ✘         | ✓         | ?            |
| Git 驅動的靜態頁面       | [⚙️][gitea-pages-server], [⚙️][gitea-caddy-plugin]   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Git 驅動的整合 wiki      | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✘            |
| 部署 Token               | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 有寫入權限的儲存庫 Token | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✘         | ✓            |
| 內建 Container Registry  | [✘](https://github.com/go-gitea/gitea/issues/2316) | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| 對外部 Git 鏡像          | ✓                                                  | ✓    | ✘         | ✘         | ✓         | ✓         | ✓            |
| FIDO (2FA)               | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| 內建 CI/CD               | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 子群組: 群組中的群組     | ✘                                                  | ✘    | ✘         | ✓         | ✓         | ✘         | ✓            |

## 程式碼管理

| 功能                                      | Gitea                                            | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ----------------------------------------- | ------------------------------------------------ | ---- | --------- | --------- | --------- | --------- | ------------ |
| 儲存庫主題描述                            | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 儲存庫程式碼搜尋                          | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 全域程式碼搜尋                            | ✓                                                | ✘    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Git LFS 2.0                               | ✓                                                | ✘    | ✓         | ✓         | ✓         | ⁄         | ✓            |
| 群組里程碑                                | ✘                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| 精細的使用者權限（程式碼, 問題, Wiki 等） | ✓                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| 驗證提交者                                | ⁄                                                | ✘    | ?         | ✓         | ✓         | ✓         | ✘            |
| GPG 簽署提交                              | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 拒絕未經簽署的提交                        | [✓](https://github.com/go-gitea/gitea/pull/9708) | ✘    | ✓         | ✓         | ✓         | ✘         | ✓            |
| 儲存庫動態頁                              | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 分支管理                                  | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 建立新分支                                | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 網頁程式碼編輯器                          | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 提交線圖                                  | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 儲存庫範本                                | [✓](https://github.com/go-gitea/gitea/pull/8768) | ✘    | ✓         | ✘         | ✓         | ✓         | ✘            |

## 問題追蹤器

| 功能                 | Gitea                                              | Gogs                                          | GitHub EE | GitLab CE                                                               | GitLab EE | BitBucket | RhodeCode CE |
| -------------------- | -------------------------------------------------- | --------------------------------------------- | --------- | ----------------------------------------------------------------------- | --------- | --------- | ------------ |
| 問題追蹤器           | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✓         | ✘            |
| 問題範本             | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 標籤                 | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 時間追蹤             | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 指派問題給多個成員   | ✓                                                  | ✘                                             | ✓         | ✘                                                                       | ✓         | ✘         | ✘            |
| 相關問題             | ✘                                                  | ✘                                             | ⁄         | [✓](https://docs.gitlab.com/ce/user/project/issues/related_issues.html) | ✓         | ✘         | ✘            |
| 機密問題             | [✘](https://github.com/go-gitea/gitea/issues/3217) | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘         | ✘            |
| 對留言的反應         | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 鎖定對話             | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 批次處理問題         | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘         | ✘            |
| 問題看板（看板方法） | [✓](https://github.com/go-gitea/gitea/pull/8346)   | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘         | ✘            |
| 從問題建立新分支     | ✘                                                  | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘         | ✘            |
| 問題搜尋             | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓         | ✘            |
| 全域問題搜尋         | [✘](https://github.com/go-gitea/gitea/issues/2434) | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓         | ✘            |
| 問題相依             | ✓                                                  | ✘                                             | ✘         | ✘                                                                       | ✘         | ✘         | ✘            |
| 從電子郵件建立問題   | [✘](https://github.com/go-gitea/gitea/issues/6226) | [✘](https://github.com/gogs/gogs/issues/2602) | ✘         | ✓                                                                       | ✓         | ✓         | ✘            |
| 服務台               | [✘](https://github.com/go-gitea/gitea/issues/6219) | ✘                                             | ✘         | [✓](https://gitlab.com/groups/gitlab-org/-/epics/3103)                  | ✓         | ✘         | ✘            |

## 拉取/合併請求

| 功能                       | Gitea                                              | Gogs | GitHub EE | GitLab CE                                                                         | GitLab EE | BitBucket                                                                | RhodeCode CE |
| -------------------------- | -------------------------------------------------- | ---- | --------- | --------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------------ | ------------ |
| 拉取/合併請求              | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| Squash 合併                | ✓                                                  | ✘    | ✓         | [✓](https://docs.gitlab.com/ce/user/project/merge_requests/squash_and_merge.html) | ✓         | ✓                                                                        | ✓            |
| Rebase 合併                | ✓                                                  | ✓    | ✓         | ✘                                                                                 | ⁄         | ✘                                                                        | ✓            |
| 拉取/合併請求的行內留言    | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| 拉取/合併請求的核可        | ✓                                                  | ✘    | ⁄         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| 解決合併衝突               | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| 限制某些使用者的推送及合併 | ✓                                                  | ✘    | ✓         | ⁄                                                                                 | ✓         | ✓                                                                        | ✓            |
| 還原指定的提交或合併請求   | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| 拉取/合併請求範本          | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| Cherry-picking 變更        | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✘         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| 下載 Patch                 | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | [/](https://jira.atlassian.com/plugins/servlet/mobile#issue/BCLOUD-8323) | ✘            |

## 第三方整合

| 功能                      | Gitea                                            | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ------------------------- | ------------------------------------------------ | ---- | --------- | --------- | --------- | --------- | ------------ |
| 支援 Webhook              | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 自訂 Git Hook             | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 整合 AD / LDAP            | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| 支援多重 LDAP / AD 伺服器 | ✓                                                | ✓    | ✘         | ✘         | ✓         | ✓         | ✓            |
| 同步 LDAP 使用者          | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| SAML 2.0 service provider                      | [✘](https://github.com/go-gitea/gitea/issues/5512) | [✘](https://github.com/gogs/gogs/issues/1221) | ✓         | ✓         | ✓         | ✓         | ✘            |
| 支援 OpenId Connect       | ✓                                                | ✘    | ✓         | ✓         | ✓         | ?         | ✘            |
| 整合 OAuth 2.0 (外部驗證) | ✓                                                | ✘    | ⁄         | ✓         | ✓         | ?         | ✓            |
| 成為 OAuth 2.0 提供者     | [✓](https://github.com/go-gitea/gitea/pull/5378) | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| 兩步驟驗證 (2FA)          | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✘            |
| 整合 Mattermost/Slack     | ✓                                                | ✓    | ⁄         | ✓         | ✓         | ⁄         | ✓            |
| 整合 Discord              | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✘         | ✘            |
| 整合 Microsoft Teams      | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| 顯示外部 CI/CD 狀態       | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |

[gitea-caddy-plugin]: https://github.com/42wim/caddy-gitea
[gitea-pages-server]: https://codeberg.org/Codeberg/pages-server
