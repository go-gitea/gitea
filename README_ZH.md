[English](https://github.com/go-gitea/gitea/blob/master/README.md)

# Gitea - Git with a cup of tea

[![Build Status](http://drone.gitea.io/api/badges/go-gitea/gitea/status.svg)](http://drone.gitea.io/go-gitea/gitea)
[![Join the chat at https://gitter.im/go-gitea/gitea](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-gitea/gitea?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](http://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![Coverage Status](https://coverage.gitea.io/badges/go-gitea/gitea/coverage.svg)](https://coverage.gitea.io/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea)
[![GoDoc](https://godoc.org/code.gitea.io/gitea?status.svg)](https://godoc.org/code.gitea.io/gitea)

[![](public/img/gitea-large-resize.png)](https://github.com/go-gitea/gitea)

##### 状态

**最新稳定版**: (查看 [Releases](https://github.com/go-gitea/gitea/releases))

| Web | UI  | Preview  |
|:-------------:|:-------:|:-------:|
|![Dashboard](https://gogs.io/img/screenshots/1.png)|![Repository](https://gogs.io/img/screenshots/2.png)|![Commits History](https://gogs.io/img/screenshots/3.png)|
|![Profile](https://gogs.io/img/screenshots/4.png)|![Admin Dashboard](https://gogs.io/img/screenshots/5.png)|![Diff](https://gogs.io/img/screenshots/6.png)|
|![Issues](https://gogs.io/img/screenshots/7.png)|![Releases](https://gogs.io/img/screenshots/8.png)|![Organization](https://gogs.io/img/screenshots/9.png)|

### 重要提示

1. **开始贡献代码之前请确保你已经看过了 [贡献者向导（英文）](CONTRIBUTING.md)**.
2. 所有的安全问题，请私下发送邮件给 **security@gitea.io**。谢谢！
3. 如果你要使用API，请参见 [API 文档](https://godoc.org/github.com/go-gitea/go-sdk).

## 目标

Gitea的首要目标是创建一个极易安装，运行非常快速，安装和使用体验良好的自建 Git 服务。我们采用Go作为后端语言，这使我们只要生成一个可执行程序即可。并且他还支持跨平台，支持 Linux, macOS 和 Windows 以及各种架构，除了x86，amd64，还包括 ARM 和 PowerPC。

如果您想试用一下，请访问 [在线Demo](https://try.gitea.io/)！

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
- 支持 MySQL、PostgreSQL、SQLite3 和 TiDB（实验性支持） 数据库
- 支持多语言本地化（21 种语言）

## 系统要求

- 最低的系统硬件要求为一个廉价的树莓派
- 如果用于团队项目，建议使用 2 核 CPU 及 1GB 内存

## 浏览器支持

- 请根据 [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) 查看具体支持的浏览器版本。
- 官方支持的最小 UI 尺寸为 **1024*768**，UI 不一定会在更小尺寸的设备上被破坏，但我们无法保证且不会修复。

## 安装

**提示: 因为 Gitea 是从 [Gogs](https://github.com/gogits/gogs) fork 过来的，因此教程和文档可以直接应用在 Gitea 上**

安装 Gitea:

- go get code.gitea.io/gitea
- [使用Docker安装](https://docs.gitea.io/zh-cn/%E4%BB%8Edocker%E5%AE%89%E8%A3%85/)
- [使用Vagrant安装](https://github.com/go-gitea/examples/tree/master/vagrant)

**提示: 二进制版本即将发布**

### 教程

- [使用 Gogs 搭建自己的 Git 服务器](https://mynook.info/blog/post/host-your-own-git-server-using-gogs)
- [阿里云上 Ubuntu 14.04 64 位安装 Gogs](http://my.oschina.net/luyao/blog/375654)
- [How To Set Up Gogs on Ubuntu 14.04](https://www.digitalocean.com/community/tutorials/how-to-set-up-gogs-on-ubuntu-14-04) (英文)
- [Run your own GitHub-like service with the help of Docker](http://blog.hypriot.com/post/run-your-own-github-like-service-with-docker/) (英文)
- [Dockerized Gogs git server and alpine postgres in 20 minutes or less](http://garthwaite.org/docker-gogs.html) (英文)
- [Host Your Own Private GitHub with Gogs.io](https://eladnava.com/host-your-own-private-github-with-gogs-io/) (英文)
- [Installing Gogs on FreeBSD](https://www.codejam.info/2015/03/installing-gogs-on-freebsd.html) (英文)
- [Gogs on Raspberry Pi](http://blog.meinside.pe.kr/Gogs-on-Raspberry-Pi/) (英文)
- [Cloudflare Full SSL with GOGS (Go Git Service) using NGINX](http://www.listekconsulting.com/articles/cloudflare-full-ssl-with-gogs-go-git-service-using-nginx/) (英文)

- [How to install Gogs on a Linux Server (DigitalOcean)](https://www.youtube.com/watch?v=deSfX0gqefE) (英文 视频)
- [Instalando Gogs no Ubuntu](https://www.youtube.com/watch?v=4UkHAR1F7ZA) (Português) (英文 视频)

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

- [Drone](https://github.com/drone/drone) (CI)
- [Fabric8](http://fabric8.io/) (DevOps)
- [Taiga](https://taiga.io/) (Project Management)
- [Puppet](https://forge.puppetlabs.com/Siteminds/gogs) (IT)
- [Kanboard](http://kanboard.net/plugin/gogs-webhook) (Project Management)
- [BearyChat](https://bearychat.com/) (Team Communication)
- [HiWork](http://www.hiwork.cc/) (Team Communication)

### 产品支持

- [Synology](https://www.synology.com) (Docker)
- [One Space](http://www.onespace.cc) (App Store)

## 特别鸣谢

- Router and middleware mechanism of [Macaron](https://github.com/go-macaron/macaron).
- System Monitor Status is inspired by [GoBlog](https://github.com/fuxiaohei/goblog).
- Thanks [Rocker](http://weibo.com/rocker1989) for designing Logo.
- Thanks [Crowdin](https://crowdin.com/project/gogs) for providing open source translation plan.
- Thanks [DigitalOcean](https://www.digitalocean.com) for hosting home and demo sites.
- Thanks [KeyCDN](https://www.keycdn.com/) and [QiNiu](http://www.qiniu.com/) for providing CDN service.

## 贡献流程

Fork -> Patch -> Push -> Pull Request

## 作者

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](conf/locale/TRANSLATORS)

## 授权许可

本项目采用 MIT 开源授权许可证，完整的授权说明已放置在 [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) 文件中。
