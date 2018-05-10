[简体中文](https://github.com/go-gitea/gitea/blob/master/README_ZH.md)

# Gitea - Git with a cup of tea

[![Build Status](https://drone.gitea.io/api/badges/go-gitea/gitea/status.svg)](https://drone.gitea.io/go-gitea/gitea)
[![Join the Discord chat at https://discord.gg/NsatcWJ](https://img.shields.io/discord/322538954119184384.svg)](https://discord.gg/NsatcWJ)
[![](https://images.microbadger.com/badges/image/gitea/gitea.svg)](https://microbadger.com/images/gitea/gitea "Get your own image badge on microbadger.com")
[![codecov](https://codecov.io/gh/go-gitea/gitea/branch/master/graph/badge.svg)](https://codecov.io/gh/go-gitea/gitea)
[![Go Report Card](https://goreportcard.com/badge/code.gitea.io/gitea)](https://goreportcard.com/report/code.gitea.io/gitea)
[![GoDoc](https://godoc.org/code.gitea.io/gitea?status.svg)](https://godoc.org/code.gitea.io/gitea)
[![Release](https://github-release-version.herokuapp.com/github/go-gitea/gitea/release.svg?style=flat)](https://github.com/go-gitea/gitea/releases/latest)
[![Help Contribute to Open Source](https://www.codetriage.com/go-gitea/gitea/badges/users.svg)](https://www.codetriage.com/go-gitea/gitea)
[![Become a backer/sponsor of gitea](https://opencollective.com/gitea/tiers/backer/badge.svg?label=backer&color=brightgreen)](https://opencollective.com/gitea)

| | | |
|:---:|:---:|:---:|
|![Dashboard](https://image.ibb.co/dms6DG/1.png)|![Repository](https://image.ibb.co/m6MSLw/2.png)|![Commits History](https://image.ibb.co/cjrSLw/3.png)|
|![Branches](https://image.ibb.co/e6vbDG/4.png)|![Issues](https://image.ibb.co/bJTJSb/5.png)|![Pull Request View](https://image.ibb.co/e02dSb/6.png)|
|![Releases](https://image.ibb.co/cUzgfw/7.png)|![Activity](https://image.ibb.co/eZgGDG/8.png)|![Wiki](https://image.ibb.co/dYV9YG/9.png)|
|![Diff](https://image.ibb.co/ewA9YG/10.png)|![Organization](https://image.ibb.co/ceOwDG/11.png)|![Profile](https://image.ibb.co/c44Q7b/12.png)|

## Purpose

The goal of this project is to make the easiest, fastest, and most
painless way of setting up a self-hosted Git service.
Using Go, this can be done with an independent binary distribution across
**all platforms** which Go supports, including Linux, macOS, and Windows
on x86, amd64, ARM and PowerPC architectures.
Want to try it before doing anything else?
Do it [with the online demo](https://try.gitea.io/)!
This project has been
[forked](https://blog.gitea.io/2016/12/welcome-to-gitea/) from
[Gogs](https://gogs.io) since 2016.11 but changed a lot.

## Building

From the root of the source tree, run:

    make generate all

More info: https://docs.gitea.io/en-us/install-from-source/

## Using

    ./gitea web

NOTE: If you're interested in using our APIs, we have experimental
support with [documentation](https://godoc.org/code.gitea.io/sdk/gitea).

## Contributing

Expected workflow is: Fork -> Patch -> Push -> Pull Request

NOTES:

1. **YOU MUST READ THE [CONTRIBUTORS GUIDE](CONTRIBUTING.md) BEFORE STARTING TO WORK ON A PULL REQUEST.**
2. If you have found a vulnerability in the project, please write privately to **security@gitea.io**. Thanks!

## Further information

For more information and instructions about how to install Gitea, please look
at our [documentation](https://docs.gitea.io/en-us/). If you have questions
that are not covered by the documentation, you can get in contact with us on
our [Discord server](https://discord.gg/NsatcWJ),
[Matrix room](https://matrix.to/#/#gitea:matrix.org),
or [forum](https://discourse.gitea.io/)!

## Authors

* [Maintainers](https://github.com/orgs/go-gitea/people)
* [Contributors](https://github.com/go-gitea/gitea/graphs/contributors)
* [Translators](options/locale/TRANSLATORS)

## License

This project is licensed under the MIT License.
See the [LICENSE](https://github.com/go-gitea/gitea/blob/master/LICENSE) file
for the full license text.
