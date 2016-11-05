# Gitea - Git with a cup of tea

[![Build Status](https://travis-ci.org/go-gitea/gitea.svg?branch=master)](https://travis-ci.org/go-gitea/gitea)
[![codecov](https://codecov.io/gh/go-gitea/gitea/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-gitea/gitea)](https://goreportcard.com/report/github.com/go-gitea/gitea)
[![GoDoc](https://godoc.org/github.com/go-gitea/gitea?status.svg)](https://godoc.org/github.com/go-gitea/gitea)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](http://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![Join the chat at https://gitter.im/go-gitea/gitea](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-gitea/gitea?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

![](https://github.com/go-gitea/gitea/blob/master/public/img/gogs-large-resize.png?raw=true)

##### Current tip version: 0.9.99 (see [Releases](https://github.com/go-gitea/gitea/releases) for binary versions or submit a task on [alpha stage automated binary building system](https://build.gogs.io/))

| Web | UI  | Preview  |
|:-------------:|:-------:|:-------:|
|![Dashboard](https://gogs.io/img/screenshots/1.png)|![Repository](https://gogs.io/img/screenshots/2.png)|![Commits History](https://gogs.io/img/screenshots/3.png)|
|![Profile](https://gogs.io/img/screenshots/4.png)|![Admin Dashboard](https://gogs.io/img/screenshots/5.png)|![Diff](https://gogs.io/img/screenshots/6.png)|
|![Issues](https://gogs.io/img/screenshots/7.png)|![Releases](https://gogs.io/img/screenshots/8.png)|![Organization](https://gogs.io/img/screenshots/9.png)|

### Important Notes

1. **YOU MUST READ [Contributing Code](https://github.com/go-gitea/gitea/wiki/Contributing-Code) BEFORE STARTING TO WORK ON A PULL REQUEST**.
2. Due to testing purpose, data of [try.gogs.io](https://try.gogs.io) was reset in **Jan 28, 2015** and will reset multiple times after. Please do **NOT** put your important data on the site.
3. The demo site [try.gogs.io](https://try.gogs.io) is running under `develop` branch.
4. If you think there are vulnerabilities in the project, please talk privately to **u@gogs.io**. Thanks!
5. If you're interested in using APIs, we have experimental support with [documentation](https://github.com/gogits/go-gogs-client/wiki).
6. If your team/company is using Gogs and would like to put your logo on [our website](https://gogs.io), contact us by any means.

## Purpose

The goal of this project is to make the easiest, fastest, and most painless way of setting up a self-hosted Git service. With Go, this can be done with an independent binary distribution across **ALL platforms** that Go supports, including Linux, Mac OS X, Windows and ARM.

## Overview

- Please see the [Documentation](https://gogs.io/docs/intro) for common usages and change log.
- See the [Trello Board](https://trello.com/b/uxAoeLUl/gogs-go-git-service) to follow the develop team.
- Want to try it before doing anything else? Do it [online](https://try.gogs.io/gogs/gogs)!
- Having trouble? Get help with [Troubleshooting](https://gogs.io/docs/intro/troubleshooting.html) or [User Forum](https://discuss.gogs.io/).
- Want to help with localization? Check out the [guide](https://gogs.io/docs/features/i18n.html)!

## Features

- Activity timeline
- SSH and HTTP/HTTPS protocols
- SMTP/LDAP/Reverse proxy authentication
- Reverse proxy with sub-path
- Account/Organization/Repository management
- Add/Remove repository collaborators
- Repository/Organization webhooks (including Slack)
- Repository Git hooks/deploy keys
- Repository issues, pull requests and wiki
- Migrate and mirror repository and its wiki
- Web editor for repository files and wiki
- Gravatar and Federated avatar with custom source
- Mail service
- Administration panel
- Supports MySQL, PostgreSQL, SQLite3 and [TiDB](https://github.com/pingcap/tidb) (experimental)
- Multi-language support ([19 languages](https://crowdin.com/project/gogs))

## System Requirements

- A cheap Raspberry Pi is powerful enough for basic functionality.
- 2 CPU cores and 1GB RAM would be the baseline for teamwork.

## Browser Support

- Please see [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) for specific versions of supported browsers.
- The official support minimal size  is **1024*768**, UI may still looks right in smaller size but no promises and fixes.

## Installation

Make sure you install the [prerequisites](https://gogs.io/docs/installation) first.

There are 5 ways to install Gogs:

- [Install from binary](https://gogs.io/docs/installation/install_from_binary.html)
- [Install from source](https://gogs.io/docs/installation/install_from_source.html)
- [Install from packages](https://gogs.io/docs/installation/install_from_packages.html)
- [Ship with Docker](https://github.com/go-gitea/gitea/tree/master/docker)
- [Install with Vagrant](https://github.com/geerlingguy/ansible-vagrant-examples/tree/master/gogs)

### Tutorials

- [How To Set Up Gogs on Ubuntu 14.04](https://www.digitalocean.com/community/tutorials/how-to-set-up-gogs-on-ubuntu-14-04)
- [Run your own GitHub-like service with the help of Docker](http://blog.hypriot.com/post/run-your-own-github-like-service-with-docker/)
- [Dockerized Gogs git server and alpine postgres in 20 minutes or less](http://garthwaite.org/docker-gogs.html)
- [Host Your Own Private GitHub with Gogs.io](https://eladnava.com/host-your-own-private-github-with-gogs-io/)
- [使用 Gogs 搭建自己的 Git 服务器](https://mynook.info/blog/post/host-your-own-git-server-using-gogs) (Chinese)
- [阿里云上 Ubuntu 14.04 64 位安装 Gogs](http://my.oschina.net/luyao/blog/375654) (Chinese)
- [Installing Gogs on FreeBSD](https://www.codejam.info/2015/03/installing-gogs-on-freebsd.html)
- [Gogs on Raspberry Pi](http://blog.meinside.pe.kr/Gogs-on-Raspberry-Pi/)
- [Cloudflare Full SSL with GOGS (Go Git Service) using NGINX](http://www.listekconsulting.com/articles/cloudflare-full-ssl-with-gogs-go-git-service-using-nginx/)

### Screencasts

- [How to install Gogs on a Linux Server (DigitalOcean)](https://www.youtube.com/watch?v=deSfX0gqefE)
- [Instalando Gogs no Ubuntu](https://www.youtube.com/watch?v=4UkHAR1F7ZA) (Português)

### Deploy to Cloud

- [OpenShift](https://github.com/tkisme/gogs-openshift)
- [Cloudron](https://cloudron.io/appstore.html#io.gogs.cloudronapp)
- [Scaleway](https://www.scaleway.com/imagehub/gogs/)
- [Portal](https://portaldemo.xyz/cloud/)
- [Sandstorm](https://github.com/cem/gogs-sandstorm)
- [sloppy.io](https://github.com/sloppyio/quickstarters/tree/master/gogs)
- [YunoHost](https://github.com/mbugeia/gogs_ynh)
- [DPlatform](https://github.com/j8r/DPlatform)

## Software and Service Support

- [Drone](https://github.com/drone/drone) (CI)
- [Fabric8](http://fabric8.io/) (DevOps)
- [Taiga](https://taiga.io/) (Project Management)
- [Puppet](https://forge.puppetlabs.com/Siteminds/gogs) (IT)
- [Kanboard](http://kanboard.net/plugin/gogs-webhook) (Project Management)
- [BearyChat](https://bearychat.com/) (Team Communication)
- [HiWork](http://www.hiwork.cc/) (Team Communication)

### Product Support

- [Synology](https://www.synology.com) (Docker)
- [One Space](http://www.onespace.cc) (App Store)

## Acknowledgments

- Router and middleware mechanism of [Macaron](https://github.com/go-macaron/macaron).
- System Monitor Status is inspired by [GoBlog](https://github.com/fuxiaohei/goblog).
- Thanks [Rocker](http://weibo.com/rocker1989) for designing Logo.
- Thanks [Crowdin](https://crowdin.com/project/gogs) for providing open source translation plan.
- Thanks [DigitalOcean](https://www.digitalocean.com) for hosting home and demo sites.
- Thanks [KeyCDN](https://www.keycdn.com/) and [QiNiu](http://www.qiniu.com/) for providing CDN service.

## Contributors

- Ex-team members [@lunny](https://github.com/lunny), [@fuxiaohei](https://github.com/fuxiaohei) and [@slene](https://github.com/slene).
- See [contributors page](https://github.com/go-gitea/gitea/graphs/contributors) for full list of contributors.
- See [TRANSLATORS](conf/locale/TRANSLATORS) for public list of translators.

## License

This project is under the MIT License. See the [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) file for the full license text.
