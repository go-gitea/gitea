---
date: "2021-12-13:10:10+08:00"
title: "Permissions"
slug: "permissions"
weight: 14
toc: false
draft: false
aliases:
  - /en-us/permissions
menu:
  sidebar:
    parent: "usage"
    name: "Permissions"
    weight: 14
    identifier: "permissions"
---

# Permissions

**Table of Contents**

{{< toc >}}

Gitea supports permissions for repository so that you can give different access for different people. At first, we need to know about `Unit`.

## Unit

In Gitea, we call a sub module of a repository `Unit`. Now we have following units.

| Name            | Description                                          | Permissions |
| --------------- | ---------------------------------------------------- | ----------- |
| Code            | Access source code, files, commits and branches.     | Read Write  |
| Issues          | Organize bug reports, tasks and milestones.          | Read Write  |
| PullRequests    | Enable pull requests and code reviews.               | Read Write  |
| Releases        | Track project versions and downloads.                | Read Write  |
| Wiki            | Write and share documentation with collaborators.    | Read Write  |
| ExternalWiki    | Link to an external wiki                             | Read        |
| ExternalTracker | Link to an external issue tracker                    | Read        |
| Projects        | The URL to the template repository                   | Read Write  |
| Settings        | Manage the repository                                | Admin       |

With different permissions, people could do different things with these units.

| Name            | Read                                               | Write                        | Admin                     |
| --------------- | -------------------------------------------------  | ---------------------------- | ------------------------- |
| Code            | View code trees, files, commits, branches and etc. | Push codes.                  | -                         |
| Issues          | View issues and create new issues.                 | Add labels, assign, close    | -                         |
| PullRequests    | View pull requests and create new pull requests.   | Add labels, assign, close    | -                         |
| Releases        | View releases and download files.                  | Create/Edit releases         | -                         |
| Wiki            | View wiki pages. Clone the wiki repository.        | Create/Edit wiki pages, push | -                         |
| ExternalWiki    | Link to an external wiki                           | -                            | -                         |
| ExternalTracker | Link to an external issue tracker                  | -                            | -                         |
| Projects        | View the boards                                    | Change issues across boards  | -                         |
| Settings        | -                                                  | -                            | Manage the repository     |

And there are some differences for permissions between individual repositories and organization repositories.

## Individual Repository

For individual repositories, the creators are the only owners of repositories and have no limit to change anything of this
repository or delete it. Repositories owners could add collaborators to help maintain the repositories. Collaborators could have `Read`, `Write` and `Admin` permissions.

## Organization Repository

Different from individual repositories, the owner of organization repositories are the owner team of this organization.

### Team

A team in an organization has unit permissions settings. It can have members and repositories scope. A team could access all the repositories in this organization or special repositories changed by the owner team. A team could also be allowed to create new
repositories.

The owner team will be created when the organization is created, and the creator will become the first member of the owner team.
Every member of an organization must be in at least one team. The owner team cannot be deleted and only
members of the owner team can create a new team. An admin team can be created to manage some of the repositories, whose members can do anything with these repositories.
The Generate team can be created by the owner team to do the operations allowed by their permissions.
