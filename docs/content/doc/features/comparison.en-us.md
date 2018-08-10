---
date: "2018-05-07T13:00:00+02:00"
title: "Gitea compared to other Git hosting options"
slug: "comparison"
weight: 5
toc: true
draft: false
menu:
  sidebar:
    parent: "features"
    name: "Comparison"
    weight: 5
    identifier: "comparison"
---

# Gitea compared to other Git hosting options

To help decide if Gitea is suited for your needs here is how it compares to other Git self hosted options.

Be warned that we don't regularly check for feature changes in other products so this list can be outdated. If you find anything that needs to be updated in table below please report [issue on Github](https://github.com/go-gitea/gitea/issues).

_Symbols used in table:_

* _✓ - supported_

* _⁄ - supported with limited functionality_

* _✘ - unsupported_

#### General Features

| Feature | Gitea | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
|---------|-------|------|-----------|-----------|-----------|-----------|--------------|
| Open source and free | ✓ | ✓ | ✘| ✓ | ✘ | ✘ | ✓ |
| Low resource usage (RAM/CPU) | ✓ | ✓ | ✘ | ✘ | ✘ | ✘ | ✘ |
| Multiple database support | ✓ | ✓ | ✘ | ⁄ | ⁄ | ✓ | ✓ |
| Multiple OS support | ✓ | ✓ | ✘ | ✘ | ✘ | ✘ | ✓ |
| Easy upgrade process | ✓ | ✓ | ✘ | ✓ | ✓ | ✘ | ✓ |
| Markdown support | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Static Git-powered pages | ✘ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Integrated Git-powered wiki | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Deploy Tokens | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Repository Tokens with write rights | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✓ |
| Built-in Container Registry | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| External git mirroring | ✓ | ✓ | ✘ | ✘ | ✓ | ✓ | ✓ |
| FIDO U2F (2FA) | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Built-in CI/CD | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Subgroups: groups within groups | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✓ |

#### Code management

| Feature | Gitea | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
|---------|-------|------|-----------|-----------|-----------|-----------|--------------|
| Repository topics | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Repository code search | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Global code search | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Git LFS 2.0 | ✓ | ✘ | ✓ | ✓ | ✓ | ⁄ | ✓ |
| Group Milestones | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Granular user roles (Code, Issues, Wiki etc) | ✓ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Verified Committer | ✘ | ✘ | ? | ✓ | ✓ | ✓ | ✘ |
| GPG Signed Commits | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Reject unsigned commits | ✘ | ✘ | ✓ | ✓ | ✓ | ✘ | ✓ |
| Repository Activity page | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Branch manager | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Create new branches | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Web code editor | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Commit graph | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |

#### Issue Tracker

| Feature | Gitea | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
|---------|-------|------|-----------|-----------|-----------|-----------|--------------|
| Issue tracker | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Issue templates | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Labels | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Time tracking | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Multiple assignees for issues | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Related issues | ✘ | ✘ | ⁄ | ✘ | ✓ | ✘ | ✘ |
| Confidential issues | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Comment reactions | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Lock Discussion | ✘ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Batch issue handling | ✓ | ✘ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Issue Boards | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Create new branches from issues | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |
| Issue search | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Global issue search | ✘ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Issue dependency | ✓ | ✘ | ✘ | ✘ | ✘ | ✘ | ✘ |

#### Pull/Merge requests

| Feature | Gitea | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
|---------|-------|------|-----------|-----------|-----------|-----------|--------------|
| Pull/Merge requests | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Squash merging | ✓ | ✘ | ✓ | ✘ | ✓ | ✓ | ✓ |
| Rebase merging | ✓ | ✓ | ✓ | ✘ | ⁄ | ✘ | ✓ |
| Pull/Merge request inline comments | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Pull/Merge request approval | ✓ | ✘ | ⁄ | ✓ | ✓ | ✓ | ✓ |
| Merge conflict resolution | ✘ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Restrict push and merge access to certain users | ✓ | ✘ | ✓ | ⁄ | ✓ | ✓ | ✓ |
| Revert specific commits or a merge request | ✘ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Pull/Merge requests templates | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ | ✘ |
| Cherry-picking changes | ✘ | ✘ | ✘ | ✓ | ✓ | ✘ | ✘ |


#### 3rd-party integrations

| Feature | Gitea | Gogs | GitHub EE | GitLab CE | GitLab EE | BitBucket | RhodeCode CE |
|---------|-------|------|-----------|-----------|-----------|-----------|--------------|
| Webhook support | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Custom Git Hooks | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| AD / LDAP integration | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| Multiple LDAP / AD server support | ✓ | ✓ | ✘ | ✘ | ✓ | ✓ | ✓ |
| LDAP user synchronization | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
| OpenId Connect support | ✓ | ✘ | ✓ | ✓ | ✓ | ? | ✘ |
| OAuth 2.0 integration (external authorization) | ✓ | ✘ | ⁄ | ✓ | ✓ | ? | ✓ |
| Act as OAuth 2.0 provider | ✘ | ✘ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Two factor authentication (2FA) | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✘ |
| Mattermost/Slack integration | ✓ | ✓ | ⁄ | ✓ | ✓ | ⁄ | ✓ |
| Discord integration | ✓ | ✓ | ✓ | ✘ | ✘ | ✘ | ✘ |
| External CI/CD status display | ✓ | ✘ | ✓ | ✓ | ✓ | ✓ | ✓ |
