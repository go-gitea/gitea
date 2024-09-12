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

[View this document in Chinese](./README_ZH.md)

## Purpose

The goal of this project is to make the easiest, fastest, and most
painless way of setting up a self-hosted Git service.

As Gitea is written in Go, it works across **all** the platforms and
architectures that are supported by Go, including Linux, macOS, and
Windows on x86, amd64, ARM and PowerPC architectures.
This project has been
[forked](https://blog.gitea.com/welcome-to-gitea/) from
[Gogs](https://gogs.io) since November of 2016, but a lot has changed.

For online demonstrations, you can visit [demo.gitea.com](https://demo.gitea.com).

For accessing free Gitea service (with a limited number of repositories), you can visit [gitea.com](https://gitea.com/user/login).

To quickly deploy your own dedicated Gitea instance on Gitea Cloud, you can start a free trial at [cloud.gitea.com](https://cloud.gitea.com).

## Building

From the root of the source tree, run:

    TAGS="bindata" make build

or if SQLite support is required:

    TAGS="bindata sqlite sqlite_unlock_notify" make build

The `build` target is split into two sub-targets:

- `make backend` which requires [Go Stable](https://go.dev/dl/), the required version is defined in [go.mod](/go.mod).
- `make frontend` which requires [Node.js LTS](https://nodejs.org/en/download/) or greater.

Internet connectivity is required to download the go and npm modules. When building from the official source tarballs which include pre-built frontend files, the `frontend` target will not be triggered, making it possible to build without Node.js.

More info: https://docs.gitea.com/installation/install-from-source

## Using

    ./gitea web

> [!NOTE]
> If you're interested in using our APIs, we have experimental support with [documentation](https://docs.gitea.com/api).

## Contributing

Expected workflow is: Fork -> Patch -> Push -> Pull Request

> [!NOTE]
>
> 1. **YOU MUST READ THE [CONTRIBUTORS GUIDE](CONTRIBUTING.md) BEFORE STARTING TO WORK ON A PULL REQUEST.**
> 2. If you have found a vulnerability in the project, please write privately to **security@gitea.io**. Thanks!

## Translating

Translations are done through Crowdin. If you want to translate to a new language ask one of the managers in the Crowdin project to add a new language there.

You can also just create an issue for adding a language or ask on discord on the #translation channel. If you need context or find some translation issues, you can leave a comment on the string or ask on Discord. For general translation questions there is a section in the docs. Currently a bit empty but we hope to fill it as questions pop up.

https://docs.gitea.com/contributing/localization

[![Crowdin](https://badges.crowdin.net/gitea/localized.svg)](https://crowdin.com/project/gitea)

## Further information

For more information and instructions about how to install Gitea, please look at our [documentation](https://docs.gitea.com/).
If you have questions that are not covered by the documentation, you can get in contact with us on our [Discord server](https://discord.gg/Gitea) or create  a post in the [discourse forum](https://forum.gitea.com/).

We maintain a list of Gitea-related projects at [gitea/awesome-gitea](https://gitea.com/gitea/awesome-gitea).

The official Gitea CLI is developed at [gitea/tea](https://gitea.com/gitea/tea).

## Authors

- [Maintainers](https://github.com/orgs/go-gitea/people)
- [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
- [Translators](options/locale/TRANSLATORS)

## Backers

Thank you to all our backers! 🙏 [[Become a backer](https://opencollective.com/gitea#backer)]

<a href="https://opencollective.com/gitea#backers" target="_blank"><img src="https://opencollective.com/gitea/backers.svg?width=890"></a>

## Sponsors

Support this project by becoming a sponsor. Your logo will show up here with a link to your website. [[Become a sponsor](https://opencollective.com/gitea#sponsor)]

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

## FAQ

**How do you pronounce Gitea?**

Gitea is pronounced [/ɡɪ’ti:/](https://youtu.be/EM71-2uDAoY) as in "gi-tea" with a hard g.

**Why is this not hosted on a Gitea instance?**

We're [working on it](https://github.com/go-gitea/gitea/issues/1029).

## License

This project is licensed under the MIT License.
See the [LICENSE](https://github.com/go-gitea/gitea/blob/main/LICENSE) file
for the full license text.

## Screenshots

Looking for an overview of the interface? Check it out!

|![Dashboard](https://dl.gitea.com/screenshots/home_timeline.png)|![User Profile](https://dl.gitea.com/screenshots/user_profile.png)|![Global Issues](https://dl.gitea.com/screenshots/global_issues.png)|
|:---:|:---:|:---:|
|![Branches](https://dl.gitea.com/screenshots/branches.png)|![Web Editor](https://dl.gitea.com/screenshots/web_editor.png)|![Activity](https://dl.gitea.com/screenshots/activity.png)|
|![New Migration](https://dl.gitea.com/screenshots/migration.png)|![Migrating](https://dl.gitea.com/screenshots/migration.gif)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)|
|![Pull Request Dark](https://dl.gitea.com/screenshots/pull_requests_dark.png)|![Diff Review Dark](https://dl.gitea.com/screenshots/review_dark.png)|![Diff Dark](https://dl.gitea.com/screenshots/diff_dark.png)|
