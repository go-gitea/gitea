---
date: "2019-04-05T16:00:00+02:00"
title: "FAQ"
slug: "faq"
sidebar_position: 5
toc: false
draft: false
aliases:
  - /en-us/faq
menu:
  sidebar:
    parent: "help"
    name: "FAQ"
    sidebar_position: 5
    identifier: "faq"
---

# Frequently Asked Questions

This page contains some common questions and answers.

For more help resources, check all [Support Options](help/support.md).

## Difference between 1.x and 1.x.x downloads, how can I get latest stable release with bug fixes?

Version 1.20.x will be used for this example.

On our [downloads page](https://dl.gitea.com/gitea/) you will see a 1.20 directory, as well as directories for 1.20.0, 1.20.1.

The 1.20 directory is the nightly build, which is built on each merged commit to the [`release/v1.20`](https://github.com/go-gitea/gitea/tree/release/v1.20) branch.

The 1.20.0 directory is a release build that was created when the [`v1.20.0`](https://github.com/go-gitea/gitea/releases/tag/v1.20.0) tag was created.

The nightly builds (1.x) downloads will change as commits are merged to their respective branch, they contain the latest changes/fixes before a tag release is built.

If a bug fix is targeted on 1.20.1 but 1.20.1 is not released yet, you can get the "1.20-nightly" build to get the bug fix.

## How to migrate from Gogs/GitHub/etc. to Gitea

To migrate from Gogs to Gitea:

- [Gogs version 0.11.46.0418](https://github.com/go-gitea/gitea/issues/4286)

To migrate from GitHub to Gitea, you can use Gitea's built-in migration form.

In order to migrate items such as issues, pull requests, etc. you will need to input at least your username.

[Example (requires login)](https://demo.gitea.com/repo/migrate)

To migrate from GitLab to Gitea, you can use this non-affiliated tool:

https://github.com/loganinak/MigrateGitlabToGogs

## Where does Gitea store what file

- _`AppWorkPath`_
  - The `WORK_PATH` option in `app.ini`
  - Else the `--work-path` flag
  - Else Environment variable `GITEA_WORK_DIR`
  - Else a built-in value set at build time
  - Else the directory that contains the Gitea binary
- `AppDataPath` (default for database, indexers, etc.)
  - `APP_DATA_PATH` from `app.ini`
  - Else _`AppWorkPath`_`/data`
- _`CustomPath`_ (custom templates)
  - The `--custom-path` flag
  - Else Environment variable `GITEA_CUSTOM`
  - Else a built-in value set at build time
  - Else _`AppWorkPath`_`/custom`
- HomeDir
  - Unix: Environment variable `HOME`
  - Windows: Environment variable `USERPROFILE`, else environment variables `HOMEDRIVE`+`HOMEPATH`
- RepoRootPath
  - `ROOT` in the \[repository] section of `app.ini` if absolute
  - Else _`AppWorkPath`_`/ROOT` if `ROOT` in the \[repository] section of `app.ini` if relative
  - Default `%(APP_DATA_PATH)/gitea-repositories`
- INI (config file)
  - `--config` flag
  - A possible built-in value set a build time
  - Else _`CustomPath`_`/conf/app.ini`
- SQLite Database
  - `PATH` in `database` section of `app.ini`
  - Else `%(APP_DATA_PATH)/gitea.db`

## Not seeing a clone URL or the clone URL being incorrect

There are a few places that could make this show incorrectly.

1. If using a reverse proxy, make sure you have followed the correction directions in the [reverse proxy guide](administration/reverse-proxies.md)
2. Make sure you have correctly set `ROOT_URL` in the `server` section of your `app.ini`

If certain clone options aren't showing up (HTTP/S or SSH), the following options can be checked in your `app.ini`

- `DISABLE_HTTP_GIT`: if set to true, there will be no HTTP/HTTPS link
- `DISABLE_SSH`: if set to true, there will be no SSH link
- `SSH_EXPOSE_ANONYMOUS`: if set to false, SSH links will be hidden for anonymous users

## File upload fails with: 413 Request Entity Too Large

This error occurs when the reverse proxy limits the file upload size.

See the [reverse proxy guide](administration/reverse-proxies.md) for a solution with nginx.

## Custom Templates not loading or working incorrectly

Gitea's custom templates must be added to the correct location or Gitea will not find and use them.

The correct path for the template(s) will be relative to the `CustomPath`

1. To find `CustomPath`, look for Custom File Root Path in Site Administration -> Configuration
2. If you are still unable to find a path, the default can be [calculated above](#where-does-gitea-store-what-file)
3. Once you have figured out the correct custom path, you can refer to the [customizing Gitea](administration/customizing-gitea.md) page to add your template to the correct location.

## Does Gitea have a "GitHub/GitLab pages" feature?

Gitea doesn't provide a built-in Pages server. You need a dedicated domain to serve static pages to avoid CSRF security risks.

For simple usage, you can use a reverse proxy to rewrite & serve static contents from Gitea's raw file URLs.

And there are already available third-party services, like a standalone [pages server](https://codeberg.org/Codeberg/pages-server) or a [caddy plugin](https://github.com/42wim/caddy-gitea), that can provide the required functionality.

## Active user vs login prohibited user

In Gitea, an "active" user refers to a user that has activated their account via email.

A "login prohibited" user is a user that is not allowed to log in to Gitea anymore

## Setting up logging

- [Official Docs](administration/logging-config.md)

## What is Swagger?

[Swagger](https://swagger.io/) is what Gitea uses for its API documentation.

All Gitea instances have the built-in API and there is no way to disable it completely.
You can, however, disable showing its documentation by setting `ENABLE_SWAGGER` to `false` in the `api` section of your `app.ini`.
For more information, refer to Gitea's [API docs](development/api-usage.md).

You can see the latest API (for example) on https://gitea.com/api/swagger

You can also see an example of the `swagger.json` file at https://gitea.com/swagger.v1.json

## Adjusting your server for public/private use

### Preventing spammers

There are multiple things you can combine to prevent spammers.

1. By whitelisting or blocklisting certain email domains
2. By only whitelisting certain domains with OpenID (see below)
3. Setting `ENABLE_CAPTCHA` to `true` in your `app.ini` and properly configuring `RECAPTCHA_SECRET` and `RECAPTCHA_SITEKEY`
4. Settings `DISABLE_REGISTRATION` to `true` and creating new users via the [CLI](administration/command-line.md), [API](development/api-usage.md), or Gitea's Admin UI

### Only allow/block certain email domains

You can configure `EMAIL_DOMAIN_WHITELIST` or `EMAIL_DOMAIN_BLOCKLIST` in your app.ini under `[service]`

### Only allow/block certain OpenID providers

You can configure `WHITELISTED_URIS` or `BLACKLISTED_URIS` under `[openid]` in your `app.ini`

**NOTE:** whitelisted takes precedence, so if it is non-blank then blacklisted is ignored

### Issue only users

The current way to achieve this is to create/modify a user with a max repo creation limit of 0.

### Restricted users

Restricted users are limited to a subset of the content based on their organization/team memberships and collaborations, ignoring the public flag on organizations/repos etc.\_\_

Example use case: A company runs a Gitea instance that requires login. Most repos are public (accessible/browsable by all co-workers).

At some point, a customer or third party needs access to a specific repo and only that repo. Making such a customer account restricted and granting any needed access using team membership(s) and/or collaboration(s) is a simple way to achieve that without the need to make everything private.

### Enable Fail2ban

Use [Fail2Ban](administration/fail2ban-setup.md) to monitor and stop automated login attempts or other malicious behavior based on log patterns

## SSHD vs built-in SSH

SSHD is the built-in SSH server on most Unix systems.

Gitea also provides its own SSH server, for usage when SSHD is not available.

## Gitea is running slow

The most common culprit for this is loading federated avatars.

This can be turned off by setting `ENABLE_FEDERATED_AVATAR` to `false` in your `app.ini`

Another option that may need to be changed is setting `DISABLE_GRAVATAR` to `true` in your `app.ini`

## Can't create repositories/files

Make sure that Gitea has sufficient permissions to write to its home directory and data directory.

See [AppDataPath and RepoRootPath](#where-does-gitea-store-what-file)

**Note for Arch users:** At the time of writing this, there is an issue with the Arch package's systemd file including this line:

`ReadWritePaths=/etc/gitea/app.ini`

Which makes all other paths non-writeable to Gitea.

## Translation is incorrect/how to add more translations

Our translations are currently crowd-sourced on our [Crowdin project](https://crowdin.com/project/gitea)

Whether you want to change a translation or add a new one, it will need to be there as all translations are overwritten in our CI via the Crowdin integration.

## Push Hook / Webhook / Actions aren't running

If you can push but can't see push activities on the home dashboard, or the push doesn't trigger webhook and Actions workflows, it's likely that the git hooks are not working.

There are a few possibilities:

1. The git hooks are out of sync: run "Resynchronize pre-receive, update and post-receive hooks of all repositories" on the site admin panel
2. The git repositories (and hooks) are stored on some filesystems (ex: mounted by NAS) which don't support script execution, make sure the filesystem supports `chmod a+x any-script`
3. If you are using docker, make sure Docker Server (not the client) >= 20.10.6

## SSH issues

If you cannot reach repositories over `ssh`, but `https` works fine, consider looking into the following.

First, make sure you can access Gitea via SSH.

`ssh git@myremote.example`

If the connection is successful, you should receive an error message like the following:

```
Hi there, You've successfully authenticated, but Gitea does not provide shell access.
If this is unexpected, please log in with password and setup Gitea under another user.
```

If you do not get the above message but still connect, it means your SSH key is **not** being managed by Gitea. This means hooks won't run, among other potential problems.

If you cannot connect at all, your SSH key may not be configured correctly locally.
This is specific to SSH and not Gitea, so will not be covered here.

### SSH Common Errors

```
Permission denied (publickey).
fatal: Could not read from remote repository.
```

This error signifies that the server rejected a log in attempt, check the
following things:

- On the client:
  - Ensure the public and private ssh keys are added to the correct Gitea user.
  - Make sure there are no issues in the remote url. In particular, ensure the name of the
    Git user (before the `@`) is spelled correctly.
  - Ensure public and private ssh keys are correct on client machine.
- On the server:
  - Make sure the repository exists and is correctly named.
  - Check the permissions of the `.ssh` directory in the system user's home directory.
  - Verify that the correct public keys are added to `.ssh/authorized_keys`.

    Try to run `Rewrite '.ssh/authorized_keys' file (for Gitea SSH keys)` on the
    Gitea admin panel.
  - Read Gitea logs.
  - Read /var/log/auth (or similar).
  - Check permissions of repositories.

The following is an example of a missing public SSH key where authentication
succeeded, but some other setting is preventing SSH from reaching the correct
repository.

```
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

In this case, look into the following settings:

- On the server:
  - Make sure that the `git` system user has a usable shell set
    - Verify this with `getent passwd git | cut -d: -f7`
    - `usermod` or `chsh` can be used to modify this.
  - Ensure that the `gitea serv` command in `.ssh/authorized_keys` uses the
    correct configuration file.

## Missing releases after migrating repository with tags

To migrate an repository _with_ all tags, you need to do two things:

- Push tags to the repository:

```
 git push --tags
```

- (Re-)sync tags of all repositories within Gitea:

```
gitea admin repo-sync-releases
```

## LFS Issues

For issues concerning LFS data upload

```
batch response: Authentication required: Authorization error: <GITEA_LFS_URL>/info/lfs/objects/batch
Check that you have proper access to the repository
error: failed to push some refs to '<GIT_REPO_URL>'
```

Check the value of `LFS_HTTP_AUTH_EXPIRY` in your `app.ini` file.

By default, your LFS token will expire after 20 minutes. If you have a slow connection or a large file (or both), it may not finish uploading within the time limit.

You may want to set this value to `60m` or `120m`.

## How can I create users before starting Gitea

Gitea provides a sub-command `gitea migrate` to initialize the database, after which you can use the [admin CLI commands](administration/command-line.md#admin) to add users like normal.

## How can I enable password reset

There is no setting for password resets. It is enabled when a [mail service](administration/email-setup.md) is configured, and disabled otherwise.

## How can a user's password be changed

- As an **admin**, you can change any user's password (and optionally force them to change it on next login)...
  - By navigating to your `Site Administration -> User Accounts` page and editing a user.
  - By using the [admin CLI commands](administration/command-line.md#admin).

    Keep in mind most commands will also need a [global flag](administration/command-line.md#global-options) to point the CLI at the correct configuration.
- As a **user** you can change it...
  - In your account `Settings -> Account` page (this method **requires** you to know your current password).
  - By using the `Forgot Password` link.

    If the `Forgot Password/Account Recovery` page is disabled, please contact your administrator to configure a [mail service](administration/email-setup.md).

## Why is my markdown broken

In Gitea version `1.11` we moved to [goldmark](https://github.com/yuin/goldmark) for markdown rendering, which is [CommonMark](https://commonmark.org/) compliant.

If you have markdown that worked as you expected prior to version `1.11` and after upgrading it's not working anymore, please look through the CommonMark spec to see whether the problem is due to a bug or non-compliant syntax.

If it is the latter, _usually_ there is a compliant alternative listed in the spec.

## Upgrade errors with MySQL

If you are receiving errors on upgrade of Gitea using MySQL that read:

> `ORM engine initialization failed: migrate: do migrate: Error: 1118: Row size too large...`

Please run `gitea doctor convert` or run `ALTER TABLE table_name ROW_FORMAT=dynamic;` for each table in the database.

The underlying problem is that the space allocated for indices by the default row format
is too small. Gitea requires that the `ROWFORMAT` for its tables is `DYNAMIC`.

If you are receiving an error line containing `Error 1071: Specified key was too long; max key length is 1000 bytes...`
then you are attempting to run Gitea on tables which use the ISAM engine. While this may have worked by chance in previous versions of Gitea, it has never been officially supported and
you must use InnoDB. You should run `ALTER TABLE table_name ENGINE=InnoDB;` for each table in the database.

## Why are Emoji displaying only as placeholders or in monochrome

Gitea requires the system or browser to have one of the supported Emoji fonts installed, which are Apple Color Emoji, Segoe UI Emoji, Segoe UI Symbol, Noto Color Emoji and Twemoji Mozilla. Generally, the operating system should already provide one of these fonts, but especially on Linux, it may be necessary to install them manually.

## Initial logging

Before Gitea has read the configuration file and set-up its logging it will log a number of things to stdout in order to help debug things if logging does not work.

You can stop this logging by setting the `--quiet` or `-q` option. Please note this will only stop logging until Gitea has set-up its own logging.

If you report a bug or issue you MUST give us logs with this information restored.

You should only set this option once you have completely configured everything.

## Warnings about struct defaults during database startup

Sometimes when there are migrations the old columns and default values may be left
unchanged in the database schema. This may lead to warning such as:

```
2020/08/02 11:32:29 ...rm/session_schema.go:360:Sync() [W] Table user Column keep_activity_private db default is , struct default is 0
```

These can safely be ignored, but you are able to stop these warnings by getting Gitea to recreate these tables using:

```
gitea doctor recreate-table user
```

This will cause Gitea to recreate the user table and copy the old data into the new table
with the defaults set appropriately.

You can ask Gitea to recreate multiple tables using:

```
gitea doctor recreate-table table1 table2 ...
```

And if you would like Gitea to recreate all tables simply call:

```
gitea doctor recreate-table
```

It is highly recommended to back-up your database before running these commands.

## How to adopt repositories from disk

- Add your (bare) repositories to the correct spot for your configuration (`repository.ROOT`), ensuring they are in the correct layout `<REPO_ROOT>/[user]/[repo].git`.
  - **Note:** the directory names must be lowercase.
  - You can also check `<ROOT_URL>/admin/config` for the repository root path.
- Ensure that the user/org exists that you want to adopt repositories for.
- As an admin, go to `<ROOT_URL>/admin/repos/unadopted` and search.
  - Users can also be given similar permissions via config [`ALLOW_ADOPTION_OF_UNADOPTED_REPOSITORIES`](administration/config-cheat-sheet.md#repository).
- If the above steps are done correctly, you should be able to select repositories to adopt.
  - If no repositories are found, enable [debug logging](administration/config-cheat-sheet.md#repository) to check for any specific errors.

## Gitea can't start on NFS

In most cases, it's caused by broken NFS lock system. You can try to stop Gitea process and
run `flock -n /data-nfs/gitea/queues/LOCK echo 'lock acquired'` to see whether the lock can be acquired immediately.
If the lock can't be acquired, NFS might report some errors like `lockd: cannot monitor node-3, statd: server rpc.statd not responding, timed out` in its server logs.

Then the NFS lock could be reset by:

```bash
# /etc/init.d/nfs stop
# rm -rf /var/lib/nfs/sm/*
# /etc/init.d/nfs start
```
