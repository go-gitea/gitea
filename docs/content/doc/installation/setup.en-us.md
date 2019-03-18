---
title: "Setup"
slug: "setup"
weight: 10
toc: true
draft: true
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
the setup page, which looks like this:

![Screenshot of the setup page](images/gitea-setup-page-1.png)

On the setup page, you can configure the following settings:

## Database Settings

Includes database type and other database-dependent settings. You can choose 
from SQLite3, MySQL, PostgreSQL, and Microsoft SQL Server. (The default database 
system is SQLite3.)

## General Settings

| Option                 | Required? | In `app.ini`                  |
| ---------------------- | --------- | ----------------------------- |
| Site Title             | required  | `APP_NAME`                    |
| Repository Root Path   | required  | `repository.ROOT`             |
| Git LFS Root Path      | optional  | `server.LFS_CONTENT_PATH`     |
| Run As Username        | required  | `RUN_USER`                    |
| SSH Server Domain      | required  | `server.SSH_DOMAIN`           |
| SSH Server Port        | optional  | `server.SSH_PORT`             |
| Gitea HTTP Listen Port | required  | `server.HTTP_PORT`            |
| Gitea Base URL         | required  | `server.ROOT_URL`             |
| Log Path               | required  | `log.ROOT_PATH`               |

## Optional Settings

Consists of:

- Email Settings
- Server and Third-Party Service Settings
- Administrator Account Settings: You may create an administrator account here.
  If you do not, the first account created after setting up Gitea will 
  be an admin account.

<!-- TODO Describe setup options -->
<!-- TODO Add screenshots -->

When you are done configuring Gitea, click the button labeled "Install Gitea" at
the bottom of the page.

After finishing the setup process, you will be directed to a page where you can 
create a user account. The first registered user will automatically become an 
administrator.
