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
[![](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea "Crowdin")

[View this document in English](./README.md)

## 目标

Gitea 的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好的自建 Git 服务。我们采用 Go 作为后端语言，这使我们只要生成一个可执行程序即可。并且他还支持跨平台，支持 Linux, macOS 和 Windows 以及各种架构，除了 x86，amd64，还包括 ARM 和 PowerPC。

如果你想试用在线演示和报告问题，请访问 [demo.gitea.com](https://demo.gitea.com/)。

如果你想使用免费的 Gitea 服务（有仓库数量限制），请访问 [gitea.com](https://gitea.com/user/login)。

如果你想在 Gitea Cloud 上快速部署你自己独享的 Gitea 实例，请访问 [cloud.gitea.com](https://cloud.gitea.com) 开始免费试用。

## 提示

1. **开始贡献代码之前请确保你已经看过了 [贡献者向导（英文）](CONTRIBUTING.md)**.
2. 所有的安全问题，请私下发送邮件给 **security@gitea.io**。谢谢！
3. 如果你要使用API，请参见 [API 文档](https://godoc.org/code.gitea.io/sdk/gitea).

## 文档

关于如何安装请访问我们的 [文档站](https://docs.gitea.com/zh-cn/category/installation)，如果没有找到对应的文档，你也可以通过 [Discord - 英文](https://discord.gg/gitea) 和 QQ群 328432459 来和我们交流。

## 贡献流程

Fork -> Patch -> Push -> Pull Request

## 翻译

多语言翻译是基于Crowdin进行的.
[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea)

## 作者

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](options/locale/TRANSLATORS)

## 授权许可

本项目采用 MIT 开源授权许可证，完整的授权说明已放置在 [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) 文件中。

## 截图

|![Dashboard](https://dl.gitea.com/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.com/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.com/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.com/screenshots/branches.png)|![Web Editor](https://dl.gitea.com/screenshots/web_editor.png)|![Activity](https://dl.gitea.com/screenshots/activity.png)|
|![New Migration](https://dl.gitea.com/screenshots/migration.png)|![Migrating](https://dl.gitea.com/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)|
|![Pull Request Dark](https://dl.gitea.com/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.com/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.com/screenshots/diff_dark.png)|
