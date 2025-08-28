# Gitea

[![](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml/badge.svg?branch=main)](https://github.com/go-gitea/gitea/actions/workflows/release-nightly.yml?query=branch%3Amain "Release Nightly")
[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")
[![](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea "Go Report Card")
[![](https://pkg.go.dev/badge/code.gitea.io/gitea?status.svg)](https://pkg.go.dev/code.gitea.io/gitea "GoDoc")
[![](https://img.shields.io/github/release/go-gitea/gitea.svg)](https://github.com/go-gitea/gitea/releases/latest "GitHub release")
[![](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea "Help Contribute to Open Source")
[![](https://opencollective.com/gitea/tiers/backers/badge.svg?label=backers&color=brightgreen)](https://opencollective.com/gitea "Become a backer/sponsor of gitea")
[![](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT "License: MIT")
[![Contribute with Gitpod](https://img.shields.io/badge/Contribute%20with-Gitpod-908a85?logo=gitpod&color=green)](https://gitpod.io/#https://github.com/go-gitea/gitea)
[![](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com "Crowdin")

[English](./README.md) | [繁體中文](./README.zh-tw.md)

## 目的

这个项目的目标是提供最简单、最快速、最无痛的方式来设置自托管的 Git 服务。

由于 Gitea 是用 Go 语言编写的，它可以在 Go 支持的所有平台和架构上运行，包括 Linux、macOS 和 Windows 的 x86、amd64、ARM 和 PowerPC 架构。这个项目自 2016 年 11 月从 [Gogs](https://gogs.io) [分叉](https://blog.gitea.com/welcome-to-gitea/) 而来，但已经有了很多变化。

在线演示可以访问 [demo.gitea.com](https://demo.gitea.com)。

要访问免费的 Gitea 服务（有一定数量的仓库限制），可以访问 [gitea.com](https://gitea.com/user/login)。

要快速部署您自己的专用 Gitea 实例，可以在 [cloud.gitea.com](https://cloud.gitea.com) 开始免费试用。

## 文件

您可以在我们的官方 [文件网站](https://docs.gitea.com/) 上找到全面的文件。

它包括安装、管理、使用、开发、贡献指南等，帮助您快速入门并有效地探索所有功能。

如果您有任何建议或想要贡献，可以访问 [文件仓库](https://gitea.com/gitea/docs)

## 构建

从源代码树的根目录运行：

    TAGS="bindata" make build

如果需要 SQLite 支持：

    TAGS="bindata sqlite sqlite_unlock_notify" make build

`build` 目标分为两个子目标：

- `make backend` 需要 [Go Stable](https://go.dev/dl/)，所需版本在 [go.mod](/go.mod) 中定义。
- `make frontend` 需要 [Node.js LTS](https://nodejs.org/en/download/) 或更高版本。

需要互联网连接来下载 go 和 npm 模块。从包含预构建前端文件的官方源代码压缩包构建时，不会触发 `frontend` 目标，因此可以在没有 Node.js 的情况下构建。

更多信息：https://docs.gitea.com/installation/install-from-source

## 使用

构建后，默认情况下会在源代码树的根目录生成一个名为 `gitea` 的二进制文件。要运行它，请使用：

    ./gitea web

> [!注意]
> 如果您对使用我们的 API 感兴趣，我们提供了实验性支持，并附有 [文件](https://docs.gitea.com/api)。

## 贡献

预期的工作流程是：Fork -> Patch -> Push -> Pull Request

> [!注意]
>
> 1. **在开始进行 Pull Request 之前，您必须阅读 [贡献者指南](CONTRIBUTING.md)。**
> 2. 如果您在项目中发现了漏洞，请私下写信给 **security@gitea.io**。谢谢！

## 翻译

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com)

翻译通过 [Crowdin](https://translate.gitea.com) 进行。如果您想翻译成新的语言，请在 Crowdin 项目中请求管理员添加新语言。

您也可以创建一个 issue 来添加语言，或者在 discord 的 #translation 频道上询问。如果您需要上下文或发现一些翻译问题，可以在字符串上留言或在 Discord 上询问。对于一般的翻译问题，文档中有一个部分。目前有点空，但我们希望随着问题的出现而填充它。

更多信息请参阅 [文件](https://docs.gitea.com/contributing/localization)。

## 官方和第三方项目

我们提供了一个官方的 [go-sdk](https://gitea.com/gitea/go-sdk)，一个名为 [tea](https://gitea.com/gitea/tea) 的 CLI 工具和一个 Gitea Action 的 [action runner](https://gitea.com/gitea/act_runner)。

我们在 [gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea) 维护了一个 Gitea 相关项目的列表，您可以在那里发现更多的第三方项目，包括 SDK、插件、主题等。

## 通讯

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")

如果您有任何文件未涵盖的问题，可以在我们的 [Discord 服务器](https://discord.gg/Gitea) 上与我们联系，或者在 [discourse 论坛](https://forum.gitea.com/) 上创建帖子。

## 作者

- [维护者](https://github.com/orgs/go-gitea/people)
- [贡献者](https://github.com/go-gitea/gitea/graphs/contributors)
- [翻译者](options/locale/TRANSLATORS)

## 支持者

感谢所有支持者！ 🙏 [[成为支持者](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## 赞助商

通过成为赞助商来支持这个项目。您的标志将显示在这里，并带有链接到您的网站。 [[成为赞助商](https://opencollective.com/gitea#sponsor)]

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

## 常见问题

**Gitea 怎么发音？**

Gitea 的发音是 [/ɡɪ’ti:/](https://youtu.be/EM71-2uDAoY)，就像 "gi-tea" 一样，g 是硬音。

**为什么这个项目没有托管在 Gitea 实例上？**

我们正在 [努力](https://github.com/go-gitea/gitea/issues/1029)。

**在哪里可以找到安全补丁？**

在 [发布日志](https://github.com/go-gitea/gitea/releases) 或 [变更日志](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md) 中，搜索关键词 `SECURITY` 以找到安全补丁。

## 许可证

这个项目是根据 MIT 许可证授权的。
请参阅 [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) 文件以获取完整的许可证文本。

## 进一步信息

<details>
<summary>寻找界面概述？查看这里！</summary>

### 登录/注册页面

![Login](https://dl.gitea.com/screenshots/login.png)
![Register](https://dl.gitea.com/screenshots/register.png)

### 用户仪表板

![Home](https://dl.gitea.com/screenshots/home.png)
![Issues](https://dl.gitea.com/screenshots/issues.png)
![Pull Requests](https://dl.gitea.com/screenshots/pull_requests.png)
![Milestones](https://dl.gitea.com/screenshots/milestones.png)

### 用户资料

![Profile](https://dl.gitea.com/screenshots/user_profile.png)

### 探索

![Repos](https://dl.gitea.com/screenshots/explore_repos.png)
![Users](https://dl.gitea.com/screenshots/explore_users.png)
![Orgs](https://dl.gitea.com/screenshots/explore_orgs.png)

### 仓库

![Home](https://dl.gitea.com/screenshots/repo_home.png)
![Commits](https://dl.gitea.com/screenshots/repo_commits.png)
![Branches](https://dl.gitea.com/screenshots/repo_branches.png)
![Labels](https://dl.gitea.com/screenshots/repo_labels.png)
![Milestones](https://dl.gitea.com/screenshots/repo_milestones.png)
![Releases](https://dl.gitea.com/screenshots/repo_releases.png)
![Tags](https://dl.gitea.com/screenshots/repo_tags.png)

#### 仓库问题

![List](https://dl.gitea.com/screenshots/repo_issues.png)
![Issue](https://dl.gitea.com/screenshots/repo_issue.png)

#### 仓库拉取请求

![List](https://dl.gitea.com/screenshots/repo_pull_requests.png)
![Pull Request](https://dl.gitea.com/screenshots/repo_pull_request.png)
![File](https://dl.gitea.com/screenshots/repo_pull_request_file.png)
![Commits](https://dl.gitea.com/screenshots/repo_pull_request_commits.png)

#### 仓库操作

![List](https://dl.gitea.com/screenshots/repo_actions.png)
![Details](https://dl.gitea.com/screenshots/repo_actions_run.png)

#### 仓库活动

![Activity](https://dl.gitea.com/screenshots/repo_activity.png)
![Contributors](https://dl.gitea.com/screenshots/repo_contributors.png)
![Code Frequency](https://dl.gitea.com/screenshots/repo_code_frequency.png)
![Recent Commits](https://dl.gitea.com/screenshots/repo_recent_commits.png)

### 组织

![Home](https://dl.gitea.com/screenshots/org_home.png)

</details>
