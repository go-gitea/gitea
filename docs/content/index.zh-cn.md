---
date: "2016-11-08T16:00:00+02:00"
title: "文档"
slug: /
sidebar_position: 10
toc: false
draft: false
---

# 关于Gitea

Gitea 是一个轻量级的 DevOps 平台软件。从开发计划到产品成型的整个软件生命周期，他都能够高效而轻松的帮助团队和开发者。包括 Git 托管、代码审查、团队协作、软件包注册和 CI/CD。它与 GitHub、Bitbucket 和 GitLab 等比较类似。
Gitea 最初是从 [Gogs](http://gogs.io) 分支而来，几乎所有代码都已更改。对于我们Fork的原因可以看
[这里](https://blog.gitea.com/welcome-to-gitea/)。

## 目标

Gitea的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好
的自建 Git 服务。

采用Go作为后端语言，只需生成一个可执行程序即可。
支持 Linux, macOS 和 Windows等多平台，
 支持主流的x86，amd64、
 ARM 和 PowerPC等架构。

## 功能特性

- 代码托管：Gitea⽀持创建和管理仓库、浏览提交历史和代码⽂件、审查和合并代码提交、管理协作者、管理分⽀等。它还⽀持许多常见的Git特性，⽐如标签、Cherry-pick、hook、集成协作⼯具等。
- 轻量级和快速: Gitea 的设计目标之一就是轻量级和快速响应。它不像一些大型的代码托管平台那样臃肿，因此在性能方面表现出色，适用于资源有限的服务器环境。由于其轻量级设计，Gitea 在资源消耗方面相对较低，可以在资源有限的环境下运行良好。
- 易于部署和维护: 轻松地部署在各种服务器上，不需要复杂的配置和依赖。这使得个人开发者或小团队可以方便地设置和管理自己的 Git 服务。
- 安全性: Gitea 注重安全性，提供了用户权限管理、访问控制列表等功能，可以确保代码和数据的安全性。
- 代码评审：代码评审同时支持 Pull Request workflow 和 AGit workflow。评审⼈可以在线浏览代码，并提交评审意见或问题。 提交者可以接收到评审意见，并在线回 复或修改代码。代码评审可以帮助用户和企业提⾼代码质量。
- CI/CD: Gitea Actions⽀持 CI/CD 功能，该功能兼容 GitHub Actions，⽤⼾可以采用熟悉的YAML格式编写workflows，也可以重⽤⼤量的已有的 Actions 插件。Actions 插件支持从任意的 Git 网站中下载。
- 项目管理：Gitea 通过看板和⼯单来跟踪⼀个项⽬的需求，功能和bug。⼯单⽀持分支，标签、⾥程碑、 指派、时间跟踪、到期时间、依赖关系等功能。
- 制品库: Gitea支持超过 20 种不同种类的公有或私有软件包管理，包括：Cargo, Chef, Composer, Conan, Conda, Container, Helm, Maven, npm, NuGet, Pub, PyPI, RubyGems, Vagrant等
- 开源社区支持: Gitea 是一个基于 MIT 许可证的开源项目,Gitea 拥有一个活跃的开源社区，能够持续地进行开发和改进，同时也积极接受社区贡献，保持了平台的更新和创新。
- 多语言支持： Gitea 提供多种语言界面，适应全球范围内的用户，促进了国际化和本地化。

更多功能特性：详见：https://docs.gitea.com/installation/comparison#general-features

## 系统要求

- 树莓派Pi3功能强大，足以运行 Gitea 来处理小型工作负载。
- 对于小型团队/项目而言，2 个 CPU 内核和 1GB 内存通常就足够了。
- 在 UNIX 系统上，Gitea 应使用专用的非 root 系统账户运行。
  - 注意：Gitea 管理 `~/.ssh/authorized_keys` 文件。以普通用户身份运行 Gitea 可能会破坏该用户的登录能力。
- [Git](https://git-scm.com/) 需要 2.0.0 或更高版本。
  - [Git Large File Storage](https://git-lfs.github.com/) 如果启用，且 Git 版本大于等于 2.1.2，则该选项可用
  - 如果 Git 版本大于等于 2.18，将自动启用 Git 提交历史图形化展示功能

## 浏览器支持

- Last 2 versions of Chrome, Firefox, Safari and Edge
- Firefox ESR

## 技术栈

- Web框架： [Chi](http://github.com/go-chi/chi)
- ORM: [XORM](https://xorm.io)
- UI 框架：
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - 更多组件参见 package.json
- 编辑器：
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
- 数据库驱动：
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## 集成支持

请访问 [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/) 获得更多的第三方集成支持
