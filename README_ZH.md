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

[View this document in English](./README.md)

## 目标

Gitea 的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好的自建 Git 服务。我们采用 Go 作为后端语言，这使我们只要生成一个可执行程序即可。并且他还支持跨平台，支持 Linux、macOS 和 Windows 以及各种架构，除了 x86 和 amd64，还包括 ARM 和 PowerPC。

如果你想试用在线演示和报告问题，请访问 [demo.gitea.com](https://demo.gitea.com/)。

如果你想使用免费的 Gitea 服务（有仓库数量限制），请访问 [gitea.com](https://gitea.com/user/login)。

如果你想在 Gitea Cloud 上快速部署你自己独享的 Gitea 实例，请访问 [cloud.gitea.com](https://cloud.gitea.com) 开始免费试用。

## 文档

关于如何安装请访问我们的 [文档站](https://docs.gitea.com/zh-cn/category/installation)，如果没有找到对应的文档，你也可以通过 [Discord - 英文](https://discord.gg/gitea) 和 QQ群 328432459 来和我们交流。

## 编译

在源代码的根目录下执行：

    TAGS="bindata" make build

或者如果需要SQLite支持：

    TAGS="bindata sqlite sqlite_unlock_notify" make build

编译过程会分成2个子任务：

- `make backend`，需要 [Go Stable](https://go.dev/dl/)，最低版本需求可查看 [go.mod](/go.mod)。
- `make frontend`，需要 [Node.js LTS](https://nodejs.org/en/download/) 或更高版本。

你需要连接网络来下载 go 和 npm modules。当从 tar 格式的源文件编译时，其中包含了预编译的前端文件，因此 `make frontend` 将不会被执行。这允许编译时不需要 Node.js。

更多信息: https://docs.gitea.com/installation/install-from-source

## 使用

编译之后，默认会在根目录下生成一个名为 `gitea` 的文件。你可以这样执行它：

    ./gitea web

> [!注意]
> 如果你要使用API，请参见 [API 文档](https://godoc.org/code.gitea.io/sdk/gitea)。

## 贡献

贡献流程：Fork -> Patch -> Push -> Pull Request

> [!注意]
>
> 1. **开始贡献代码之前请确保你已经看过了 [贡献者向导（英文）](CONTRIBUTING.md)**。
> 2. 所有的安全问题，请私下发送邮件给 **security@gitea.io**。 谢谢！

## 翻译

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://translate.gitea.com)

多语言翻译是基于Crowdin进行的。

从 [文档](https://docs.gitea.com/contributing/localization) 中获取更多信息。

## 官方和第三方项目

Gitea 提供官方的 [go-sdk](https://gitea.com/gitea/go-sdk)，以及名为 [tea](https://gitea.com/gitea/tea) 的 CLI 工具 和 用于 Gitea Action 的 [action runner](https://gitea.com/gitea/act_runner)。

[gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea) 是一个 Gitea 相关项目的列表，你可以在这里找到更多的第三方项目，包括 SDK、插件、主题等等。

## 作者

- [Maintainers](https://github.com/orgs/go-gitea/people)
- [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
- [Translators](options/locale/TRANSLATORS)

## 授权许可

本项目采用 MIT 开源授权许可证，完整的授权说明已放置在 [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) 文件中。

## 更多信息

<details>
<summary>截图</summary>

|![Dashboard](https://dl.gitea.com/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.com/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.com/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.com/screenshots/branches.png)|![Web Editor](https://dl.gitea.com/screenshots/web_editor.png)|![Activity](https://dl.gitea.com/screenshots/activity.png)|
|![New Migration](https://dl.gitea.com/screenshots/migration.png)|![Migrating](https://dl.gitea.com/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)|
|![Pull Request Dark](https://dl.gitea.com/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.com/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.com/screenshots/diff_dark.png)|

</details>
