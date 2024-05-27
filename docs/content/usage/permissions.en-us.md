---
date: "2021-12-13:10:10+08:00"
title: "Permissions"
slug: "permissions"
sidebar_position: 14
toc: false
draft: false
aliases:
  - /en-us/permissions
menu:
  sidebar:
    parent: "usage"
    name: "Permissions"
    sidebar_position: 14
    identifier: "permissions"
---

# Permissions

Gitea supports permissions for repository so that you can give different access for different people. At first, we need to know about `Unit`.

## Unit

In Gitea, we call a sub module of a repository `Unit`. Now we have following possible units.

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
| Packages        | Packages which linked to this repository             | Read Write  |
| Actions         | Review actions logs or restart/cacnel pipelines      | Read Write  |
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
| Projects        | View the columns of projects                       | Change issues across columns | -                         |
| Packages        | View the packages                                  | Upload/Delete packages       | -                         |
| Actions         | View the Actions logs                              | Approve / Cancel / Restart   | -                         |
| Settings        | -                                                  | -                            | Manage the repository     |

And there are some differences for permissions between individual repositories and organization repositories.

## Individual Repository

For individual repositories, the creators are the only owners of repositories and have no limit to change anything of this
repository or delete it. Repositories owners could add collaborators to help maintain the repositories. Collaborators could have `Read`, `Write` and `Admin` permissions.

For a private repository, the experience is similar to visiting an anonymous public repository. You have access to all the available content within the repository, including the ability to clone the code, create issues, respond to issue comments, submit pull requests, and more. If you have 'Write' permission, you can push code to specific branches of the repository, provided it's permitted by the branch protection rules. Additionally, you can make changes to the wiki pages. With 'Admin' permission, you have the ability to modify the repository's settings.

But you cannot delete or transfer this repository if you are not that repository's owner.

## Organization Repository

For individual repositories, the owner is the user who created it. For organization repositories, the owners are the members of the owner team on this organization. All the permissions depends on the team permission settings.

### Owner Team

The owner team will be created when the organization is created, and the creator will become the first member of the owner team. The owner team cannot be deleted and there is at least one member.

### Admin Team

When creating teams, there are two types of teams. One is the admin team, another is the general team. An admin team can be created to manage some of the repositories, whose members can do anything with these repositories. Only members of the owner or admin team can create a new team.

### General Team

A general team in an organization has unit permissions settings. It can have members and repositories scope.

- A team could access all the repositories in this organization or special repositories.
- A team could also be allowed to create new repositories or not.

The General team can be created to do the operations allowed by their permissions. One member could join multiple teams.
