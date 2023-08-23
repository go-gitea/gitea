---
date: "2016-11-08T16:00:00+02:00"
title: "Documentation"
slug: /
sidebar_position: 10
toc: false
draft: false
---

# What is Gitea?

Gitea is a painless self-hosted all-in-one software development service, it includes Git hosting, code review, team collaboration, package registry and CI/CD. It is similar to GitHub, Bitbucket and GitLab.
Gitea was forked from [Gogs](http://gogs.io) originally and almost all the code has been changed. See the [Gitea Announcement](https://blog.gitea.com/welcome-to-gitea/)
blog post to read about the justification for a fork.

## Purpose

The goal of this project is to provide the easiest, fastest, and most painless way of setting
up a self-hosted Git service.

With Go, this can be done platform-independently across
**all platforms** which Go supports, including Linux, macOS, and Windows,
on x86, amd64, ARM and PowerPC architectures.
You can try it out using [the online demo](https://try.gitea.io/).

## Features

- Code Hosting: Gitea supports creating and managing repositories, browsing commit history and code files, reviewing and merging code submissions, managing collaborators, handling branches, and more. It also supports many common Git features such as tags, Cherry-pick, hooks, integrated collaboration tools, and more.

- Lightweight and Fast: One of Gitea's design goals is to be lightweight and fast in response. Unlike some large code hosting platforms, it remains lean, performing well in terms of speed, and is suitable for resource-limited server environments. Due to its lightweight design, Gitea has relatively low resource consumption and performs well in resource-constrained environments.

- Easy Deployment and Maintenance: It can be easily deployed on various servers without complex configurations or dependencies. This makes it convenient for individual developers or small teams to set up and manage their own Git services.

- Security: Gitea places a strong emphasis on security, offering features such as user permission management, access control lists, and more to ensure the security of code and data.

- Code Review: Code review supports both the Pull Request workflow and AGit workflow. Reviewers can browse code online and provide review comments or feedback. Submitters can receive review comments and respond or modify code online. Code reviews can help individuals and organizations enhance code quality.

- CI/CD: Gitea Actions supports CI/CD functionality, compatible with GitHub Actions. Users can write workflows in familiar YAML format and reuse a variety of existing Actions plugins. Actions plugins support downloading from any Git website.

- Project Management: Gitea tracks project requirements, features, and bugs through boards and issues. Issues support features like branches, tags, milestones, assignments, time tracking, due dates, dependencies, and more.

- Artifact Repository: Gitea supports over 20 different types of public or private software package management, including Cargo, Chef, Composer, Conan, Conda, Container, Helm, Maven, npm, NuGet, Pub, PyPI, RubyGems, Vagrant, and more.

- Open Source Community Support: Gitea is an open-source project based on the MIT license. It has an active open-source community that continuously develops and improves the platform. The project also actively welcomes community contributions, ensuring updates and innovation.

- Multilingual Support: Gitea provides interfaces in multiple languages, catering to users globally and promoting internationalization and localization.

Additional Features: For more detailed information, please refer to: https://docs.gitea.com/installation/comparison#general-features

## System Requirements

- A Raspberry Pi 3 is powerful enough to run Gitea for small workloads.
- 2 CPU cores and 1GB RAM is typically sufficient for small teams/projects.
- Gitea should be run with a dedicated non-root system account on UNIX-type systems.
  - Note: Gitea manages the `~/.ssh/authorized_keys` file. Running Gitea as a regular user could break that user's ability to log in.
- [Git](https://git-scm.com/) version 2.0.0 or later is required.
  - [Git Large File Storage](https://git-lfs.github.com/) will be available if enabled and if your Git version is >= 2.1.2
  - Git commit-graph rendering will be enabled automatically if your Git version is >= 2.18

## Browser Support

- Last 2 versions of Chrome, Firefox, Safari and Edge
- Firefox ESR

## Components

- Web server framework: [Chi](http://github.com/go-chi/chi)
- ORM: [XORM](https://xorm.io)
- UI frameworks:
  - [jQuery](https://jquery.com)
  - [Fomantic UI](https://fomantic-ui.com)
  - [Vue3](https://vuejs.org)
  - and various components (see package.json)
- Editors:
  - [CodeMirror](https://codemirror.net)
  - [EasyMDE](https://github.com/Ionaru/easy-markdown-editor)
  - [Monaco Editor](https://microsoft.github.io/monaco-editor)
- Database drivers:
  - [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  - [github.com/lib/pq](https://github.com/lib/pq)
  - [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  - [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## Integrated support

 Please visit [Awesome Gitea](https://gitea.com/gitea/awesome-gitea/) to get more third-party integrated support
