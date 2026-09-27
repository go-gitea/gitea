# Teabag


[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](# "Join the Discord chat at #")


[![](#/badges/users.svg)](# "Help Contribute to Open Source")

[![](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT "License: MIT")


[繁體中文](./README.zh-tw.md) | [简体中文](./README.zh-cn.md)

## Purpose

Teabag is a lightweight, non-enterprise Git forge focused on simple self-hosting for individuals, homelabs, small teams, and communities. It provides Git hosting, issues, pull requests, releases, and lightweight CI integration without enterprise collaboration suite overhead.

As Teabag is written in Go, it works across **all** the platforms and
architectures that are supported by Go, including Linux, macOS, FreeBSD/OpenBSD and Windows
on x86, amd64, ARM, RISC-V 64 and PowerPC architectures.

For online demonstrations, you can visit [demo.gitea.com](#).

For accessing free Teabag service (with a limited number of repositories), you can visit [gitea.com](#).

To quickly deploy your own dedicated Teabag instance on Teabag Cloud, you can start a free trial at [cloud.gitea.com](#),
or use container (docker/podman/etc) to deploy on your own server with the [official image](#).

## Documentation

You can find comprehensive documentation on our official [documentation website](#).

It includes installation, administration, usage, development, contributing guides, and more to help you get started and explore all features effectively.

If you have any suggestions or would like to contribute to it, you can visit the [documentation repository](#)

## Building

See [docs/build-setup.md](docs/build-setup.md) for prerequisites
and [docs/development.md](docs/development.md) for setting up a local development environment, linting, and testing.

If you'd like to build from source or make a distribution package, see [docs/build-source.md](docs/build-source.md) for more information.

After building, you can run `./teabag web` to start the server, or `./teabag help` to see all available commands.

## Contributing

Expected workflow is: Fork -> Patch -> Push -> Pull Request

> [!NOTE]
>
> 1. **YOU MUST READ THE [CONTRIBUTORS GUIDE](CONTRIBUTING.md) BEFORE STARTING TO WORK ON A PULL REQUEST.**
> 2. New to the codebase? The [development guide](docs/development.md) walks through setting up a local environment and building from source.
> 3. If you have found a vulnerability in the project, please write privately to **security@teabag.local**. Thanks!

## Translating



Translations are done through [Crowdin](#). If you want to translate to a new language, ask one of the managers in the Crowdin project to add a new language there.

You can also just create an issue for adding a language or ask on Discord on the #translation channel. If you need context or find some translation issues, you can leave a comment on the string or ask on Discord.

Get more information from [the translation section of our contributing guide](CONTRIBUTING.md#translation).

## Official and Third-Party Projects

We provide an official [go-sdk](#), a CLI tool called [tea](#) and an [action runner](#) for Teabag Action.

We maintain a list of Teabag-related projects at [gitea/awesome-gitea](#), where you can discover more third-party projects, including SDKs, plugins, themes, and more.

## Communication

[![](https://img.shields.io/discord/322538954119184384.svg?logo=discord&logoColor=white&label=Discord&color=5865F2)](# "Join the Discord chat at #")

If you have questions that are not covered by the [documentation](#), you can get in contact with us on our [Discord server](#) or create a post in the [discourse forum](#).

## Authors

- [Maintainers](#)

- [Translators](options/locale/TRANSLATORS)

## Backers





## Sponsors














## FAQ

**How do you pronounce Teabag?**

Teabag is pronounced [/ɡɪ’ti:/](https://youtu.be/EM71-2uDAoY) as in "gi-tea" with a hard g.

**How do I configure Teabag?**

For dynamic config options, you can change it on your admin panel's configuration section.

For static config options, you can edit your `app.ini` file and restart the instance.


**Where can I find the security patches?**



(more FAQs are listed in [FAQ documentation](#help/faq))

## License

This project is licensed under the MIT License.

for the full license text.

## Further information

<details>
<summary>Looking for an overview of the interface? Check it out the screenshots!</summary>

### Login/Register Page

![Login](#login.png)
![Register](#register.png)

### User Dashboard

![Home](#home.png)
![Issues](#issues.png)
![Pull Requests](#pull_requests.png)
![Milestones](#milestones.png)

### User Profile

![Profile](#user_profile.png)

### Explore

![Repos](#explore_repos.png)
![Users](#explore_users.png)
![Orgs](#explore_orgs.png)

### Repository

![Home](#repo_home.png)
![Commits](#repo_commits.png)
![Branches](#repo_branches.png)
![Labels](#repo_labels.png)
![Milestones](#repo_milestones.png)
![Releases](#repo_releases.png)
![Tags](#repo_tags.png)

#### Repository Issue

![List](#repo_issues.png)
![Issue](#repo_issue.png)

#### Repository Pull Requests

![List](#repo_pull_requests.png)
![Pull Request](#repo_pull_request.png)
![File](#repo_pull_request_file.png)
![Commits](#repo_pull_request_commits.png)

#### Repository Actions

![List](#repo_actions.png)
![Details](#repo_actions_run.png)

#### Repository Activity

![Activity](#repo_activity.png)
![Contributors](#repo_contributors.png)
![Code Frequency](#repo_code_frequency.png)
![Recent Commits](#repo_recent_commits.png)

### Organization

![Home](#org_home.png)

</details>
