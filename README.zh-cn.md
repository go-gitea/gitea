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

## 项目目标

本项目的核心目标，是让自建Git服务的过程，变得最简单、最高效、最省心。

Gitea基于Go语言开发，凡Go语言支持的平台与架构，它皆能适配，涵盖Linux、macOS、Windows系统，以及x86、amd64、ARM、PowerPC架构。项目自2016年11月从[Gogs](https://gogs.io) [分叉](https://blog.gitea.com/welcome-to-gitea/)而来，如今已是焕然一新。

在线体验：访问[demo.gitea.com](https://demo.gitea.com)。

免费服务（仓库数量有限）：访问[gitea.com](https://gitea.com/user/login)。

快速部署专属实例：前往[cloud.gitea.com](https://cloud.gitea.com)开启免费试用。

## 官方文档

你可在[官方文档网站](https://docs.gitea.com/)获取完整文档，内容涵盖安装部署、管理维护、使用指南、开发贡献等，助你快速上手并充分探索所有功能。

若有建议或想参与文档编写，可访问[文档仓库](https://gitea.com/gitea/docs)。

## 构建方法

进入源码根目录，执行以下命令构建：

    TAGS="bindata" make build

若需支持SQLite数据库，执行：

    TAGS="bindata sqlite sqlite_unlock_notify" make build

`build`目标分为两个子目标：

- `make backend`：需依赖[Go Stable](https://go.dev/dl/)，具体版本见[go.mod](/go.mod)
- `make frontend`：需依赖[Node.js LTS](https://nodejs.org/en/download/)（及以上版本）和[pnpm](https://pnpm.io/installation)

构建需联网以下载Go和npm依赖包。若使用包含预构建前端文件的官方源码压缩包，无需触发`frontend`目标，无Node.js环境也可完成构建。

更多细节：https://docs.gitea.com/installation/install-from-source

## 使用方法

构建完成后，源码根目录默认生成`gitea`可执行文件，运行命令：

    ./gitea web

> [!NOTE]
> 若需调用API，我们已提供实验性支持，文档详见[此处](https://docs.gitea.com/api)。

## 贡献指南

标准流程：Fork → Patch → Push → Pull Request

> [!NOTE]
>
> 1. 提交Pull Request前，务必阅读[《贡献者指南》](CONTRIBUTING.md)！
> 2. 若发现项目漏洞，请通过邮件**security@gitea.io**私信反馈，感谢你的严谨！

## 多语言翻译

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com)

翻译工作通过[Crowdin](https://translate.gitea.com)进行。若需新增翻译语言，可联系Crowdin项目管理员添加；也可提交issue申请，或在Discord的#translation频道咨询。

若需翻译上下文或发现翻译问题，可在对应文本下留言或通过Discord沟通。文档设有翻译相关专区（目前内容待补充），将根据问题逐步完善。

更多信息：[翻译贡献文档](https://docs.gitea.com/contributing/localization)

## 官方及第三方项目

我们提供官方[go-sdk](https://gitea.com/gitea/go-sdk)、命令行工具[tea](https://gitea.com/gitea/tea)及Gitea Action专用[运行器](https://gitea.com/gitea/act_runner)。

我们在[gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea)维护Gitea相关项目清单，你可在此发现更多第三方项目，包括SDK、插件、主题等。

## 交流渠道

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](https://discord.gg/Gitea "Join the Discord chat at https://discord.gg/Gitea")

若[文档](https://docs.gitea.com/)未覆盖你的问题，可通过[Discord服务器](https://discord.gg/Gitea)联系我们，或在[论坛](https://forum.gitea.com/)发布帖子。

## 项目成员

- [维护者](https://github.com/orgs/go-gitea/people)
- [贡献者](https://github.com/go-gitea/gitea/graphs/contributors)
- [译者](options/locale/TRANSLATORS)

## 支持者

感谢所有支持者的鼎力相助！🙏 [[成为支持者](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## 赞助商

成为赞助商支持项目，你的logo将在此展示并链接至官网。[[成为赞助商](https://opencollective.com/gitea#sponsor)]

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

**Gitea如何发音？**

发音为[/ɡɪ'ti:/](https://youtu.be/EM71-2uDAoY)，类似"gi-tea"，"g"需发重音。

**为何项目代码未托管在Gitea自身实例上？**

我们正[推进此事](https://github.com/go-gitea/gitea/issues/1029)。

**哪里可找到安全补丁？**

在[发布日志](https://github.com/go-gitea/gitea/releases)或[更新日志](https://github.com/go-gitea/gitea/blob/main/CHANGELOG.md)中，搜索关键词`SECURITY`即可找到。

## 许可证

本项目采用MIT许可证。
完整许可文本详见[LICENSE文件](https://github.com/go-gitea/gitea/blob/main/LICENSE)。

## 更多信息

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
