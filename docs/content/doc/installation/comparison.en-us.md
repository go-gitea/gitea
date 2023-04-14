---
date: "2018-05-07T13:00:00+02:00"
title: "Compared to other Git hosting"
slug: "comparison"
weight: 5
toc: false
draft: false
menu:
  sidebar:
    name: "Comparison"
    weight: 5
    parent: installation
    identifier: "comparison"
---

# Gitea compared to other Git hosting options

**Table of Contents**

{{< toc >}}

To help decide if Gitea is suited for your needs, here is how it compares to other Git self hosted options.

Be warned that we don't regularly check for feature changes in other products, so this list may be outdated. If you find anything that needs to be updated in the table below, please [open an issue](https://github.com/go-gitea/gitea/issues/new/choose).

_Symbols used in table:_

- _✓ - supported_

- _⁄ - supported with limited functionality_

- _✘ - unsupported_

- _⚙️ - supported through third-party software_

## General Features

| Feature                                          | Gitea                                               | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ------------------------------------------------ | --------------------------------------------------- | ---- | --------- | --------- | --------- | --------- | ------------ |
| Open source and free                             | ✓                                                   | ✓    | ✘         | ✓         | ✘         | ✘         | ✓            |
| Low RAM/ CPU usage                               | ✓                                                   | ✓    | ✘         | ✘         | ✘         | ✘         | ✘            |
| Multiple database support                        | ✓                                                   | ✓    | ✘         | ⁄         | ⁄         | ✓         | ✓            |
| Multiple OS support                              | ✓                                                   | ✓    | ✘         | ✘         | ✘         | ✘         | ✓            |
| Easy upgrades                                    | ✓                                                   | ✓    | ✘         | ✓         | ✓         | ✘         | ✓            |
| Telemetry                                        | **✘**                                               | ✘    | ✓         | ✓         | ✓         | ✓         | ?            |
| Third-party render tool support                  | ✓                                                   | ✘    | ✘         | ✘         | ✘         | ✓         | ?            |
| WebAuthn (2FA)                                   | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ?            |
| Extensive API                                    | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Built-in Package/Container Registry              | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Sync commits to an external repo (push mirror)   | ✓                                                   | ✓    | ✘         | ✓         | ✓         | ✘         | ✓            |
| Sync commits from an external repo (pull mirror) | ✓                                                   | ✘    | ✘         | ✓         | ✓         | ✘         | ?            |
| Light and Dark Theme                             | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ?            |
| Custom Theme Support                             | ✓                                                   | ✓    | ✘         | ✘         | ✘         | ✓         | ✘            |
| Markdown support                                 | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| CSV support                                      | ✓                                                   | ✘    | ✓         | ✘         | ✘         | ✓         | ?            |
| 'GitHub / GitLab pages'                          | [⚙️][gitea-pages-server], [⚙️][gitea-caddy-plugin]    | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Repo-specific wiki (as a repo itself)            | ✓                                                   | ✓    | ✓         | ✓         | ✓         | /         | ✘            |
| Deploy Tokens                                    | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Repository Tokens with write rights              | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| RSS Feeds                                        | ✓                                                   | ✘    | ✓         | ✘         | ✘         | ✘         | ✘            |
| Built-in CI/CD                                   | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Subgroups: groups within groups                  | [✘](https://github.com/go-gitea/gitea/issues/1872)  | ✘    | ✘         | ✓         | ✓         | ✘         | ✓            |
| Interaction with other instances                 | [/](https://github.com/go-gitea/gitea/issues/18240) | ✘    | ✘         | ✘         | ✘         | ✘         | ✘            |
| Mermaid diagrams in Markdown                     | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Math syntax in Markdown                          | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |

## Code management

| Feature                                     | Gitea                                               | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ------------------------------------------- | --------------------------------------------------- | ---- | --------- | --------- | --------- | --------- | ------------ |
| Repository topics                           | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Repository code search                      | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Global code search                          | ✓                                                   | ✘    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Git LFS 2.0                                 | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Group Milestones                            | [✘](https://github.com/go-gitea/gitea/issues/14622) | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Granular user roles (Code, Issues, Wiki, …) | ✓                                                   | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Verified Committer                          | ⁄                                                   | ✘    | ?         | ✓         | ✓         | ✓         | ✘            |
| GPG Signed Commits                          | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| SSH Signed Commits                          | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ?         | ?            |
| Reject unsigned commits                     | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Migrating repos from other services         | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Repository Activity page                    | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Branch manager                              | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Create new branches                         | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Web code editor                             | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Commit graph                                | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Template Repositories                       | ✓                                                   | ✘    | ✓         | ✘         | ✓         | ✓         | ✘            |
| Git Blame                                   | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Visual comparison of image changes          | ✓                                                   | ✘    | ✓         | ?         | ?         | ?         | ?            |

## Issue Tracker

| Feature                       | Gitea                                               | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ----------------------------- | --------------------------------------------------- | ---- | --------- | --------- | --------- | --------- | ------------ |
| Issue tracker                 | ✓                                                   | ✓    | ✓         | ✓         | ✓         | /         | ✘            |
| Issue templates               | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Labels                        | ✓                                                   | ✓    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Time tracking                 | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Multiple assignees for issues | ✓                                                   | ✘    | ✓         | ✘         | ✓         | ✘         | ✘            |
| Related issues                | ✘                                                   | ✘    | ⁄         | ✓         | ✓         | ✘         | ✘            |
| Confidential issues           | [✘](https://github.com/go-gitea/gitea/issues/3217)  | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Comment reactions             | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Lock Discussion               | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Batch issue handling          | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Issue Boards (Kanban)         | [/](https://github.com/go-gitea/gitea/issues/14710) | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Create branch from issue      | [✘](https://github.com/go-gitea/gitea/issues/20226) | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Convert comment to new issue  | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Issue search                  | ✓                                                   | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Global issue search           | [/](https://github.com/go-gitea/gitea/issues/2434)  | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Issue dependency              | ✓                                                   | ✘    | ✘         | ✘         | ✘         | ✘         | ✘            |
| Create issue via email        | [✘](https://github.com/go-gitea/gitea/issues/6226)  | ✘    | ✘         | ✓         | ✓         | ✓         | ✘            |
| Service Desk                  | [✘](https://github.com/go-gitea/gitea/issues/6219)  | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |

## Pull/Merge requests

| Feature                                         | Gitea                                              | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ----------------------------------------------- | -------------------------------------------------- | ---- | --------- | --------- | --------- | --------- | ------------ |
| Pull/Merge requests                             | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Squash merging                                  | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Rebase merging                                  | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Pull/Merge request inline comments              | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Pull/Merge request approval                     | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Pull/Merge require approval                     | ✓                                                  | ✘    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Pull/Merge multiple reviewers                   | ✓                                                  | ✓    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Merge conflict resolution                       | [✘](https://github.com/go-gitea/gitea/issues/9014) | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Restrict push and merge access to certain users | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Revert specific commits                         | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Pull/Merge requests templates                   | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Cherry-picking changes                          | ✓                                                  | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Download Patch                                  | ✓                                                  | ✘    | ✓         | ✓         | ✓         | /         | ✘            |
| Merge queues                                    | ✘                                                  | ✘    | ✓         | ✘         | ✓         | ✘         | ✘            |

## 3rd-party integrations

| Feature                                        | Gitea                                              | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ---------------------------------------------- | ------------------------------------------------   | ---- | --------- | --------- | --------- | --------- | ------------ |
| Webhooks                                       | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Git Hooks                                      | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| AD / LDAP integration                          | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Multiple LDAP / AD server support              | ✓                                                  | ✓    | ✘         | ✘         | ✓         | ✓         | ✓            |
| LDAP user synchronization                      | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| SAML 2.0 service provider                      | [✘](https://github.com/go-gitea/gitea/issues/5512) | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| OpenID Connect support                         | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ?         | ✘            |
| OAuth 2.0 integration (external authorization) | ✓                                                  | ✘    | ⁄         | ✓         | ✓         | ?         | ✓            |
| Act as OAuth 2.0 provider                      | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Two factor authentication (2FA)                | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Integration with the most common services      | ✓                                                  | /    | ⁄         | ✓         | ✓         | ⁄         | ✓            |
| Incorporate external CI/CD                     | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |

[gitea-caddy-plugin]: https://github.com/42wim/caddy-gitea
[gitea-pages-server]: https://codeberg.org/Codeberg/pages-server
