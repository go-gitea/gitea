---
date: "2016-11-08T16:00:00+02:00"
title: "Documentation"
slug: "documentation"
weight: 10
toc: true
draft: false
---

# What is Gitea?

Gitea is a painless self-hosted Git service. It is similar to GitHub, Bitbucket or Gitlab. The initial development have been done on [Gogs](http://gogs.io) but we have forked it and named it Gitea. If you want to read more about the reasons why we have done that please read [this](https://blog.gitea.io/2016/12/welcome-to-gitea/) blog post.

## Purpose

The goal of this project is to make the easiest, fastest, and most painless way of setting up a self-hosted Git service. With Go, this can be done with an independent binary distribution across ALL platforms that Go supports, including Linux, macOS and Windows, even on architectures like ARM or PowerPC.

## Features

- User Dashboard
    - Context switcher (organization or current user)
    - Activity timeline
        - Commits
        - Issues
        - Pull requests
        - Repository creation
    - Searchable repository list
    - List of your organizations
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
        - MySQL
        - PostgreSQL
        - SQLite3
        - MSSQL
        - [TiDB](https://github.com/pingcap/tidb) (experimental)
    - Configuration file
        - See [here](https://github.com/go-gitea/gitea/blob/master/conf/app.ini)
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
            - Permission to create git hooks
            - Permission to create organizations
            - Permission to import repositories
        - Organization management
            - People
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
            - When somthing unexpected happens
        - Monitoring
            - Current processes
            - Cron jobs
                - Update mirrors
                - Repository health check
                - Check repository statstics
                - Clean up old archives
    - Environment variables
    - Command line options
- Multi-language support ([21 languages](https://github.com/go-gitea/gitea/tree/master/options/locale))
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
        - Mutiple email addresses
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
        - Git hooks
        - Deploy keys

## System Requirements

- A cheap Raspberry Pi is powerful enough for basic functionality.
- 2 CPU cores and 1GB RAM would be the baseline for teamwork.
- Gitea is supposed to be run with a dedicated non-root user account on UNIX systems, no other mode of operation is supported. (**NOTE**: in case you run it with your own user account and the built-in SSH server disabled, Gitea modifies the `~/.ssh/authorized_keys` file so you will **not** be able to interactively log in.)

## Browser Support

- Please see [Semantic UI](https://github.com/Semantic-Org/Semantic-UI#browser-support) for specific versions of supported browsers.
- The official support minimal size  is **1024*768**, UI may still looks right in smaller size but no promises and fixes.

## Components

* Web framework: [Macaron](http://go-macaron.com/)
* ORM: [XORM](https://github.com/go-xorm/xorm)
* UI components:
  * [Semantic UI](http://semantic-ui.com/)
  * [GitHub Octicons](https://octicons.github.com/)
  * [Font Awesome](http://fontawesome.io/)
  * [DropzoneJS](http://www.dropzonejs.com/)
  * [Highlight](https://highlightjs.org/)
  * [Clipboard](https://zenorocha.github.io/clipboard.js/)
  * [Emojify](https://github.com/Ranks/emojify.js)
  * [CodeMirror](https://codemirror.net/)
  * [jQuery Date Time Picker](https://github.com/xdan/datetimepicker)
  * [jQuery MiniColors](https://github.com/claviska/jquery-minicolors)
* Database drivers:
  * [github.com/go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
  * [github.com/lib/pq](https://github.com/lib/pq)
  * [github.com/mattn/go-sqlite3](https://github.com/mattn/go-sqlite3)
  * [github.com/pingcap/tidb](https://github.com/pingcap/tidb)
  * [github.com/denisenkom/go-mssqldb](https://github.com/denisenkom/go-mssqldb)

## Software and Service Support

- [Drone](https://github.com/drone/drone) (CI)
