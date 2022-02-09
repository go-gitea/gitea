---
date: "2018-05-07T13:00:00+02:00"
title: "Gitea compared to other Git hosting options"
slug: "comparison"
weight: 5
toc: false
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Comparison"
    weight: 5
    identifier: "comparison"
---

# Gitea compared to other Git hosting options

**Table of Contents**

{{< toc >}}

To help decide if Gitea is suited for your needs, here is how it compares to other Git self hosted options.

Be warned that we don't regularly check for feature changes in other products, so this list may be outdated. If you find anything that needs to be updated in the table below, please report it in an [issue on GitHub](https://github.com/go-gitea/gitea/issues).

_Symbols used in table:_

- _✓ - supported_

- _⁄ - supported with limited functionality_

- _✘ - unsupported_

## General Features

| Feature                             | Gitea                                              | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket      | RhodeCode CE |
| ----------------------------------- | -------------------------------------------------- | ---- | --------- | --------- | --------- | -------------- | ------------ |
| Open source and free                | ✓                                                  | ✓    | ✘         | ✓         | ✘         | ✘              | ✓            |
| Low resource usage (RAM/CPU)        | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘              | ✘            |
| Multiple database support           | ✓                                                  | ✓    | ✘         | ⁄         | ⁄         | ✓              | ✓            |
| Multiple OS support                 | ✓                                                  | ✓    | ✘         | ✘         | ✘         | ✘              | ✓            |
| Easy upgrade process                | ✓                                                  | ✓    | ✘         | ✓         | ✓         | ✘              | ✓            |
| Markdown support                    | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓              | ✓            |
| Orgmode support                     | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✘              | ?            |
| CSV support                         | ✓                                                  | ✘    | ✓         | ✘         | ✘         | ✓              | ?            |
| Third-party render tool support     | ✓                                                  | ✘    | ✘         | ✘         | ✘         | ✓              | ?            |
| Static Git-powered pages            | [✘](https://github.com/go-gitea/gitea/issues/302)  | ✘    | ✓         | ✓         | ✓         | ✘              | ✘            |
| Integrated Git-powered wiki         | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓ (cloud only) | ✘            |
| Deploy Tokens                       | ✓                                                  | ✓    | ✓         | ✓         | ✓         | ✓              | ✓            |
| Repository Tokens with write rights | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓              | ✓            |
| Built-in Container Registry         | [✘](https://github.com/go-gitea/gitea/issues/2316) | ✘    | ✘         | ✓         | ✓         | ✘              | ✘            |
| External git mirroring              | ✓                                                  | ✓    | ✘         | ✘         | ✓         | ✓              | ✓            |
| FIDO U2F (2FA)                      | ✓                                                  | ✘    | ✓         | ✓         | ✓         | ✓              | ✘            |
| Built-in CI/CD                      | ✘                                                  | ✘    | ✓         | ✓         | ✓         | ✘              | ✘            |
| Subgroups: groups within groups     | ✘                                                  | ✘    | ✘         | ✓         | ✓         | ✘              | ✓            |

## Code management

| Feature                                      | Gitea                                            | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| -------------------------------------------- | ------------------------------------------------ | ---- | --------- | --------- | --------- | --------- | ------------ |
| Repository topics                            | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Repository code search                       | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Global code search                           | ✓                                                | ✘    | ✓         | ✘         | ✓         | ✓         | ✓            |
| Git LFS 2.0                                  | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Group Milestones                             | ✘                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Granular user roles (Code, Issues, Wiki etc) | ✓                                                | ✘    | ✘         | ✓         | ✓         | ✘         | ✘            |
| Verified Committer                           | ⁄                                                | ✘    | ?         | ✓         | ✓         | ✓         | ✘            |
| GPG Signed Commits                           | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Reject unsigned commits                      | [✓](https://github.com/go-gitea/gitea/pull/9708) | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Repository Activity page                     | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Branch manager                               | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Create new branches                          | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Web code editor                              | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Commit graph                                 | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Template Repositories                        | [✓](https://github.com/go-gitea/gitea/pull/8768) | ✘    | ✓         | ✘         | ✓         | ✓         | ✘            |

## Issue Tracker

| Feature                         | Gitea                                              | Gogs                                          | GitHub EE | GitLab CE                                                               | GitLab EE | BitBucket | RhodeCode CE |
| ------------------------------- | -------------------------------------------------- | --------------------------------------------- | --------- | ----------------------------------------------------------------------- | --------- | -------------- | ------------ |
| Issue tracker                   | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✓ (cloud only) | ✘            |
| Issue templates                 | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Labels                          | ✓                                                  | ✓                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Time tracking                   | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Multiple assignees for issues   | ✓                                                  | ✘                                             | ✓         | ✘                                                                       | ✓         | ✘              | ✘            |
| Related issues                  | ✘                                                  | ✘                                             | ⁄         | [✓](https://docs.gitlab.com/ce/user/project/issues/related_issues.html) | ✓         | ✘              | ✘            |
| Confidential issues             | [✘](https://github.com/go-gitea/gitea/issues/3217) | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| Comment reactions               | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Lock Discussion                 | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Batch issue handling            | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✘              | ✘            |
| Issue Boards (Kanban)           | [✓](https://github.com/go-gitea/gitea/pull/8346)   | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| Create new branches from issues | ✘                                                  | ✘                                             | ✘         | ✓                                                                       | ✓         | ✘              | ✘            |
| Issue search                    | ✓                                                  | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓              | ✘            |
| Global issue search             | [✘](https://github.com/go-gitea/gitea/issues/2434) | ✘                                             | ✓         | ✓                                                                       | ✓         | ✓              | ✘            |
| Issue dependency                | ✓                                                  | ✘                                             | ✘         | ✘                                                                       | ✘         | ✘              | ✘            |
| Create issue via email          | [✘](https://github.com/go-gitea/gitea/issues/6226) | [✘](https://github.com/gogs/gogs/issues/2602) | ✘         | ✘                                                                       | ✓         | ✓              | ✘            |
| Service Desk                    | [✘](https://github.com/go-gitea/gitea/issues/6219) | ✘                                             | ✘         | [✓](https://gitlab.com/groups/gitlab-org/-/epics/3103)                  | ✓         | ✘              | ✘            |

## Pull/Merge requests

| Feature                                         | Gitea                                              | Gogs | GitHub EE | GitLab CE                                                                         | GitLab EE | BitBucket                                                                | RhodeCode CE |
| ----------------------------------------------- | -------------------------------------------------- | ---- | --------- | --------------------------------------------------------------------------------- | --------- | ------------------------------------------------------------------------ | ------------ |
| Pull/Merge requests                             | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| Squash merging                                  | ✓                                                  | ✘    | ✓         | [✓](https://docs.gitlab.com/ce/user/project/merge_requests/squash_and_merge.html) | ✓         | ✓                                                                        | ✓            |
| Rebase merging                                  | ✓                                                  | ✓    | ✓         | ✘                                                                                 | ⁄         | ✘                                                                        | ✓            |
| Pull/Merge request inline comments              | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| Pull/Merge request approval                     | ✓                                                  | ✘    | ⁄         | ✓                                                                                 | ✓         | ✓                                                                        | ✓            |
| Merge conflict resolution                       | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| Restrict push and merge access to certain users | ✓                                                  | ✘    | ✓         | ⁄                                                                                 | ✓         | ✓                                                                        | ✓            |
| Revert specific commits or a merge request      | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✓         | ✓                                                                                 | ✓         | ✓                                                                        | ✘            |
| Pull/Merge requests templates                   | ✓                                                  | ✓    | ✓         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| Cherry-picking changes                          | [✘](https://github.com/go-gitea/gitea/issues/5158) | ✘    | ✘         | ✓                                                                                 | ✓         | ✘                                                                        | ✘            |
| Download Patch                                  | ✓                                                  | ✘    | ✓         | ✓                                                                                 | ✓         | [/](https://jira.atlassian.com/plugins/servlet/mobile#issue/BCLOUD-8323) | ✘            |

## 3rd-party integrations

| Feature                                        | Gitea                                            | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
| ---------------------------------------------- | ------------------------------------------------ | ---- | --------- | --------- | --------- | --------- | ------------ |
| Webhook support                                | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Custom Git Hooks                               | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| AD / LDAP integration                          | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✓            |
| Multiple LDAP / AD server support              | ✓                                                | ✓    | ✘         | ✘         | ✓         | ✓         | ✓            |
| LDAP user synchronization                      | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
| OpenId Connect support                         | ✓                                                | ✘    | ✓         | ✓         | ✓         | ?         | ✘            |
| OAuth 2.0 integration (external authorization) | ✓                                                | ✘    | ⁄         | ✓         | ✓         | ?         | ✓            |
| Act as OAuth 2.0 provider                      | [✓](https://github.com/go-gitea/gitea/pull/5378) | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Two factor authentication (2FA)                | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✓         | ✘            |
| Mattermost/Slack integration                   | ✓                                                | ✓    | ⁄         | ✓         | ✓         | ⁄         | ✓            |
| Discord integration                            | ✓                                                | ✓    | ✓         | ✓         | ✓         | ✘         | ✘            |
| Microsoft Teams integration                    | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✘            |
| External CI/CD status display                  | ✓                                                | ✘    | ✓         | ✓         | ✓         | ✓         | ✓            |
