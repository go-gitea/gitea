---
title: "Setup"
slug: "setup"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "installation"
    name: "Setup"
    weight: 40
    identifier: "setup"
---

# Setup

The first time you access the Gitea web interface, the home page will appear. 
Clicking on "Register" or "Sign In" in the upper right corner will take you to 
the setup page, where you can configure the database type and associated 
settings, general settings such as networking and locations of files, and 
optional settings such as email.

![Screenshot of the setup page](/images/gitea-setup-page-1.png)

When you are done configuring Gitea, click the button labeled "Install Gitea" at
the bottom of the page. This will write your settings to `app.ini`.

After finishing the setup process, you will be directed to a page where you can 
create a user account. The first registered user will automatically become an 
administrator.


## Database Settings

Includes database type and other database-dependent settings. You can choose 
from SQLite3, MySQL, PostgreSQL, and Microsoft SQL Server. (The default database 
system is SQLite3.)

## General Settings

| Option                 | Required? | In `app.ini`                  |
| ---------------------- | --------- | ----------------------------- |
| Site Title             | yes       | `APP_NAME`                    |
| Repository Root Path   | yes       | `repository.ROOT`             |
| Git LFS Root Path      | no        | `server.LFS_CONTENT_PATH`     |
| Run As Username        | yes       | `RUN_USER`                    |
| SSH Server Domain      | yes       | `server.SSH_DOMAIN`           |
| SSH Server Port        | no        | `server.SSH_PORT`             |
| Gitea HTTP Listen Port | yes       | `server.HTTP_PORT`            |
| Gitea Base URL         | yes       | `server.ROOT_URL`             |
| Log Path               | yes       | `log.ROOT_PATH`               |

## Optional Settings

Consists of:

- Email Settings
- Server and Third-Party Service Settings
- Administrator Account Settings: You may create an administrator account here.
  If you do not, the first account created after setting up Gitea will 
  be an admin account.
