---
date: "2016-11-08T16:00:00+02:00"
title: "Documentation"
slug: "documentation"
url: "/en-us/"
weight: 10
toc: false
draft: false
---

# What is Gitea?

Gitea is a painless self-hosted all-in-one software development service, it includes Git hosting, code review, team collaboration, package registry and CI/CD. It is similar to GitHub, Bitbucket and GitLab.
Gitea was forked from [Gogs](http://gogs.io) originally and almost all the code has been changed. See the [Gitea Announcement](https://blog.gitea.io/2016/12/welcome-to-gitea/)
blog post to read about the justification for a fork.

## Purpose

The goal of this project is to provide the easiest, fastest, and most painless way of setting
up a self-hosted Git service.

With Go, this can be done platform-independently across
**all platforms** which Go supports, including Linux, macOS, and Windows,
on x86, amd64, ARM and PowerPC architectures.
You can try it out using [the online demo](https://try.gitea.io/).

## Features

- User Dashboard
  - Context switcher (organization or current user)
  - Activity timeline
    - Commits
    - Issues
    - Pull requests
    - Repository creation
  - Searchable repository list
  - List of organizations
  - A list of mirror repositories
- Issues dashboard
  - Context switcher (organization or current user)
  - Filter by
    - Open
    - Closed
    - Your repositories
    - Assigned issues
    - Your issues
    - Repository
  - Sort by
    - Oldest
    - Last updated
    - Number of comments
- Pull request dashboard
  - Same as issue dashboard
- Repository types
  - Mirror
  - Normal
  - Migrated
- Notifications (email and web)
  - Read
  - Unread
  - Pin
- Explore page
  - Users
  - Repos
  - Organizations
  - Search
- Custom templates
- Override public files (logo, css, etc)
- CSRF and XSS protection
- HTTPS support
- Set allowed upload sizes and types
- Logging
- Configuration
  - Databases
    - MySQL (>=5.7)
    - PostgreSQL (>=10)
    - SQLite3
    - MSSQL (>=2008R2 SP3)
    - TiDB (MySQL protocol)
  - Configuration file
    - [app.ini](https://github.com/go-gitea/gitea/blob/main/custom/conf/app.example.ini)
  - Admin panel
    - Statistics
    - Actions
      - Delete inactive accounts
      - Delete cached repository archives
      - Delete repositories records which are missing their files
      - Run garbage collection on repositories
      - Rewrite SSH keys
      - Resync hooks
      - Recreate repositories which are missing
    - Server status
      - Uptime
      - Memory
      - Current # of goroutines
      - And more
    - User management
      - Search
      - Sort
      - Last login
      - Authentication source
      - Maximum repositories
      - Disable account
      - Admin permissions
      - Permission to create Git Hooks
      - Permission to create organizations
      - Permission to import repositories
    - Organization management
      - Members
      - Teams
      - Avatar
      - Hooks
    - Repository management
      - See all repository information and manage repositories
    - Authentication sources
      - OAuth
      - PAM
      - LDAP
      - SMTP
    - Configuration viewer
      - Everything in config file
    - System notices
      - When something unexpected happens
    - Monitoring
      - Current processes
      - Cron jobs
        - Update mirrors
        - Repository health check
        - Check repository statistics
        - Clean up old archives
  - Environment variables
  - Command line options
- Multi-language support ([21 languages](https://github.com/go-gitea/gitea/tree/main/options/locale))
- [Mermaid](https://mermaidjs.github.io/) diagrams in Markdown
- Math syntax in Markdown
- Mail service
  - Notifications
  - Registration confirmation
  - Password reset
- Reverse proxy support
  - Includes subpaths
- Users
  - Profile
    - Name
    - Username
    - Email
    - Website
    - Join date
    - Followers and following
    - Organizations
    - Repositories
    - Activity
    - Starred repositories
  - Settings
    - Same as profile and more below
    - Keep email private
    - Avatar
      - Gravatar
      - Libravatar
      - Custom
    - Password
    - Multiple email addresses
    - SSH Keys
    - Connected applications
    - Two factor authentication
    - Linked OAuth2 sources
    - Delete account
- Repositories
  - Clone with SSH/HTTP/HTTPS
  - Git LFS
  - Watch, Star, Fork
  - View watchers, stars, and forks
  - Code
    - Branch browser
    - Web based file upload and creation
    - Clone urls
    - Download
      - ZIP
      - TAR.GZ
    - Web based editor
      - Markdown editor
      - Plain text editor
        - Syntax highlighting
      - Diff preview
      - Preview
      - Choose where to commit to
    - View file history
    - Delete file
    - View raw
  - Issues
    - Issue templates
    - Milestones
    - Labels
    - Assign issues
    - Track time
    - Reactions
    - Filter
      - Open
      - Closed
      - Assigned person
      - Created by you
      - Mentioning you
    - Sort
      - Oldest
      - Last updated
      - Number of comments
    - Search
    - Comments
    - Attachments
  - Pull requests
    - Same features as issues
  - Commits
    - Commit graph
    - Commits by branch
    - Search
    - Search in all branches
    - View diff
    - View SHA
    - View author
    - Browse files in commit
  - Releases
    - Attachments
    - Title
    - Content
    - Delete
    - Mark as pre-release
    - Choose branch
  - Wiki
    - Import
    - Markdown editor
  - Settings
    - Options
      - Name
      - Description
      - Private/Public
      - Website
      - Wiki
        - Enabled/disabled
        - Internal/external
      - Issues
        - Enabled/disabled
        - Internal/external
        - External supports url rewriting for better integration
      - Enable/disable pull requests
      - Transfer repository
      - Delete wiki
      - Delete repository
    - Collaboration
      - Read/write/admin
    - Branches
      - Default branch
      - Branch protection
    - Webhooks
    - Git Hooks
    - Deploy keys
- Package Registries
  - Composer
  - Conan
  - Container
  - Generic
  - Helm
  - Maven
  - NPM
  - Nuget
  - PyPI
  - RubyGems

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

## Software and Service Support

- [Drone](https://github.com/drone/drone) (CI)
