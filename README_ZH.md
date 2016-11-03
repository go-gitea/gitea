# Gitea - Git with a cup of tea

[![Build Status](https://travis-ci.org/go-gitea/gitea.svg?branch=master)](https://travis-ci.org/go-gitea/gitea)
[![codecov](https://codecov.io/gh/go-gitea/gitea/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-gitea/gitea)](https://goreportcard.com/report/github.com/go-gitea/gitea)
[![GoDoc](https://godoc.org/github.com/go-gitea/gitea?status.svg)](https://godoc.org/github.com/go-gitea/gitea)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](http://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![Join the chat at https://gitter.im/go-gitea/gitea](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-gitea/gitea?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Gogs (Go Git Service) 是一款极易搭建的自助 Git 服务。

## 开发目的

Gogs 的目标是打造一个最简单、最快速和最轻松的方式搭建自助 Git 服务。使用 Go 语言开发使得 Gogs 能够通过独立的二进制分发，并且支持 Go 语言支持的 **所有平台**，包括 Linux、Mac OS X、Windows 以及 ARM 平台。

## 项目概览

- 有关基本用法和变更日志，请通过 [使用手册](https://gogs.io/docs/intro/) 查看。
- 您可以到 [Trello Board](https://trello.com/b/uxAoeLUl/gogs-go-git-service) 跟随开发团队的脚步。
- 想要先睹为快？直接去 [在线体验](https://try.gogs.io/gogs/gogs) 。
- 使用过程中遇到问题？尝试从 [故障排查](https://gogs.io/docs/intro/troubleshooting.html) 页面或 [用户论坛](https://discuss.gogs.io/) 获取帮助。
- 希望帮助多国语言界面的翻译吗？请立即访问 [详情页面](https://gogs.io/docs/features/i18n.html)！

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
- 支持 MySQL、PostgreSQL、SQLite3 和 [TiDB](https://github.com/pingcap/tidb)（实验性支持） 数据库
- 支持多语言本地化（[19 种语言]([more](https://crowdin.com/project/gogs))）

## 系统要求

- 最低的系统硬件要求为一个廉价的树莓派
- 如果用于团队项目，建议使用 2 核 CPU 及 1GB 内存

## 浏览器支持

- 请根据 [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) 查看具体支持的浏览器版本。
- 官方支持的最小 UI 尺寸为 **1024*768**，UI 不一定会在更小尺寸的设备上被破坏，但我们无法保证且不会修复。

## 安装部署

在安装 Gogs 之前，您需要先安装 [基本环境](https://gogs.io/docs/installation)。

然后，您可以通过以下 5 种方式来安装 Gogs：

- [二进制安装](https://gogs.io/docs/installation/install_from_binary.html)
- [源码安装](https://gogs.io/docs/installation/install_from_source.html)
- [包管理安装](https://gogs.io/docs/installation/install_from_packages.html)
- [采用 Docker 部署](https://github.com/go-gitea/gitea/tree/master/docker)
- [通过 Vagrant 安装](https://github.com/geerlingguy/ansible-vagrant-examples/tree/master/gogs)

### 使用教程

- [使用 Gogs 搭建自己的 Git 服务器](https://mynook.info/blog/post/host-your-own-git-server-using-gogs)
- [阿里云上 Ubuntu 14.04 64 位安装 Gogs](http://my.oschina.net/luyao/blog/375654)

### 云端部署

- [OpenShift](https://github.com/tkisme/gogs-openshift)
- [Cloudron](https://cloudron.io/appstore.html#io.gogs.cloudronapp)
- [Scaleway](https://www.scaleway.com/imagehub/gogs/)
- [Portal](https://portaldemo.xyz/cloud/)
- [Sandstorm](https://github.com/cem/gogs-sandstorm)
- [sloppy.io](https://github.com/sloppyio/quickstarters/tree/master/gogs)
- [YunoHost](https://github.com/mbugeia/gogs_ynh)
- [DPlatform](https://github.com/j8r/DPlatform)

## 软件及服务支持

- [Drone](https://github.com/drone/drone)（CI）
- [Fabric8](http://fabric8.io/)（DevOps）
- [Taiga](https://taiga.io/)（项目管理）
- [Puppet](https://forge.puppetlabs.com/Siteminds/gogs)（IT）
- [Kanboard](http://kanboard.net/plugin/gogs-webhook)（项目管理）
- [BearyChat](https://bearychat.com/)（团队交流）
- [HiWork](http://www.hiwork.cc/)（团队交流）

### 产品支持

- [Synology](https://www.synology.com)（Docker）
- [One Space](http://www.onespace.cc)（应用商店）

## 特别鸣谢

- 基于 [Macaron](https://github.com/go-macaron/macaron) 的路由与中间件机制。
- 基于 [GoBlog](https://github.com/fuxiaohei/goblog) 修改的系统监视状态。
- 感谢 [Rocker](http://weibo.com/rocker1989) 设计的 Logo。
- 感谢 [Crowdin](https://crowdin.com/project/gogs) 提供免费的开源项目本地化支持。
- 感谢 [DigitalOcean](https://www.digitalocean.com) 提供主站和体验站点的服务器赞助。
- 感谢 [KeyCDN](https://www.keycdn.com/) 和 [七牛云存储](http://www.qiniu.com/) 提供 CDN 服务赞助。

## 贡献成员

- 前团队成员 [@lunny](https://github.com/lunny)、[@fuxiaohei](https://github.com/fuxiaohei) 和 [@slene](https://github.com/slene)。
- 您可以通过查看 [贡献者页面](https://github.com/go-gitea/gitea/graphs/contributors) 获取完整的贡献者列表。
- 您可以通过查看 [TRANSLATORS](conf/locale/TRANSLATORS) 文件获取公开的翻译人员列表。

## 授权许可

本项目采用 MIT 开源授权许可证，完整的授权说明已放置在 [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) 文件中。
