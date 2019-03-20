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

[![Screenshot of the setup page](/images/gitea-setup-page-1.png)](/images/gitea-setup-page-1.png)

When you are done configuring Gitea, click the button labeled "Install Gitea" at
the bottom of the page. This will write your settings to `app.ini`.

After finishing the setup process, you will be directed to a page where you can 
create a user account. The first registered user will automatically become an 
administrator.


## Database Settings

Includes database type and other database-dependent settings. You can choose 
from SQLite3, MySQL, PostgreSQL, and Microsoft SQL Server. (The default database 
system is SQLite3.)

To set up Gitea with SQLite3, you just have to specify the path to the database 
file. If you're running Gitea as a service, this must be an absolute path.

For MySQL, PostgreSQL, and Microsoft SQL Server, you need to specify the 
database server's domain and port, the database name, and the username and 
password that Gitea will use to access the database.

## General Settings

| Option                 | In app.ini                  | Required? |
| ---------------------- | --------------------------- | --------- |
| Site Title             | `APP_NAME`                  | yes       |
| Repository Root Path   | `repository.ROOT`           | yes       |
| Git LFS Root Path      | `server.LFS_CONTENT_PATH`   | no        |
| Run As Username        | `RUN_USER`                  | yes       |
| SSH Server Domain      | `server.SSH_DOMAIN`         | yes       |
| SSH Server Port        | `server.SSH_PORT`           | no        |
| Gitea HTTP Listen Port | `server.HTTP_PORT`          | yes       |
| Gitea Base URL         | `server.ROOT_URL`           | yes       |
| Log Path               | `log.ROOT_PATH`             | yes       |

## Optional Settings

Consists of:

- Email Settings
- Server and Third-Party Service Settings
- Administrator Account Settings: You may create an administrator account here.
  If you do not, the first account created after setting up Gitea will 
  be an admin account.
