---
date: "2016-11-08T16:00:00+02:00"
title: "文档"
slug: "documentation"
url: "/zh-cn/"
weight: 10
toc: false
draft: false
---

# 关于Gitea

Gitea 是一个自己托管的Git服务程序。他和GitHub, Bitbucket or Gitlab等比较类似。他是从 [Gogs](http://gogs.io) 发展而来，不过我们已经Fork并且命名为Gitea。对于我们Fork的原因可以看 [这里](https://blog.gitea.io/2016/12/welcome-to-gitea/)。

## 目标

Gitea的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好的自建 Git 服务。我们采用Go作为后端语言，这使我们只要生成一个可执行程序即可。并且他还支持跨平台，支持 Linux, macOS 和 Windows 以及各种架构，除了x86，amd64，还包括 ARM 和 PowerPC。

## 功能特性

- 支持活动时间线
- 支持 SSH 以及 HTTP/HTTPS 协议
- 支持 SMTP、LDAP 和反向代理的用户认证
- 支持反向代理子路径
- 支持用户、组织和仓库管理系统
- 支持添加和删除仓库协作者
- 支持仓库和组织级别 Web 钩子（包括 Slack 集成）
- 支持仓库 Git 钩子和部署密钥
- 支持仓库工单（Issue）、合并请求（Pull Request）以及 Wiki
- 支持迁移和镜像仓库以及它的 Wiki
- 支持在线编辑仓库文件和 Wiki
- 支持自定义源的 Gravatar 和 Federated Avatar
- 支持邮件服务
- 支持后台管理面板
- 支持 MySQL、PostgreSQL、SQLite3、MSSQL 和 TiDB(MySQL) 数据库
- 支持多语言本地化（21 种语言）
- 支持软件包注册中心（Composer/Conan/Container/Generic/Helm/Maven/NPM/Nuget/PyPI/RubyGems）

## 系统要求

- 最低的系统硬件要求为一个廉价的树莓派
- 如果用于团队项目，建议使用 2 核 CPU 及 1GB 内存

## 浏览器支持

- Chrome, Firefox, Safari, Edge

## 组件

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

## 软件及服务支持

- [Drone](https://github.com/drone/drone) (CI)

## 需要帮助?

如果从本页中没有找到你需要的内容，请访问 [帮助页面]({{< relref "support.zh-cn.md" >}})
