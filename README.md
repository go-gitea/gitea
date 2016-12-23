[简体中文](https://github.com/go-gitea/gitea/blob/master/README_ZH.md)

# Gitea - Git with a cup of tea

[![Build Status](http://drone.gitea.io/api/badges/go-gitea/gitea/status.svg)](http://drone.gitea.io/go-gitea/gitea)
[![Join the chat at https://gitter.im/go-gitea/gitea](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/go-gitea/gitea?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](http://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![Coverage Status](https://coverage.gitea.io/badges/go-gitea/gitea/coverage.svg)](https://coverage.gitea.io/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea)
[![GoDoc](https://godoc.org/code.gitea.io/gitea?status.svg)](https://godoc.org/code.gitea.io/gitea)

[![](public/img/gitea-large-resize.png)](https://github.com/go-gitea/gitea)

## Version

[![Release](http://github-release-version.herokuapp.com/github/go-gitea/gitea/release.svg?style=flat)](https://github.com/go-gitea/gitea/releases/latest)

||||
|:-------------:|:-------:|:-------:|
|![Dashboard](https://i.imgur.com/3iEQsux.jpg)|![Repository](https://i.imgur.com/glqFnj8.jpg)|![Commits History](https://i.imgur.com/ad1FEpi.jpg)|
|![Profile](https://i.imgur.com/q81EcGa.jpg)|![Admin Dashboard](https://i.imgur.com/L2CQeN0.jpg)|![Diff](https://i.imgur.com/cNuvMum.jpg)|
|![Issues](https://i.imgur.com/xCYRqaF.jpg)|![Releases](https://i.imgur.com/ILpRBCe.jpg)|![Organization](https://i.imgur.com/0BHnrcL.jpg)|
||||

## Notes

1. **YOU MUST READ THE [Contributors Guide](CONTRIBUTING.md) BEFORE STARTING TO WORK ON A PULL REQUEST**.
2. If you think there are vulnerabilities in the project, please talk privately to **security@gitea.io**. Thanks!
3. If you're interested in using APIs, we have experimental support with [documentation](https://godoc.org/github.com/go-gitea/go-sdk).

## Purpose

The goal of this project is to make the easiest, fastest, and most painless way of setting up a self-hosted Git service. With Go, this can be done with an independent binary distribution across **ALL platforms** that Go supports, including Linux, Mac OS X, Windows and ARM. Want to try it before doing anything else? Do it [online](https://try.gitea.io/)!

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
- Multi-language support ([21 languages](https://crowdin.com/project/gogs))

## System Requirements

- A cheap Raspberry Pi is powerful enough for basic functionality.
- 2 CPU cores and 1GB RAM would be the baseline for teamwork.

## Browser Support

- Please see [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) for specific versions of supported browsers.
- The official support minimal size  is **1024*768**, UI may still looks right in smaller size but no promises and fixes.

## Installation

* [From binary](https://docs.gitea.io/en-us/install-from-binary/)
* [From package](https://docs.gitea.io/en-us/install-from-package/)
* [From source](https://docs.gitea.io/en-us/install-from-source/)
* [With Docker](https://docs.gitea.io/en-us/install-with-docker/)

## Contributing

Fork -> Patch -> Push -> Pull Request

## Authors

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](conf/locale/TRANSLATORS)

## License

This project is under the MIT License. See the [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) file for the full license text.
