---
date: "2019-04-05T16:00:00+02:00"
title: "FAQ"
slug: "faq"
weight: 5
toc: true
draft: false
menu:
  sidebar:
    parent: "help"
    name: "FAQ"
    weight: 5
    identifier: "faq"
---

# Frequently Asked Questions

This page contains some common questions and answers.  
Also see [Support Options]({{< relref "doc/help/seek-help.en-us.md" >}})

* [How to migrate from Gogs/GitHub/etc. to Gitea](#how-to-migrate-from-gogs-github-etc-to-gitea)
* [Where does Gitea store "x" file](#where-does-gitea-store-x-file)
* [Not seeing a clone URL or the clone URL being incorrect](#not-seeing-a-clone-url-or-the-clone-url-being-incorrect)
* [Custom Templates not loading or working incorrectly](#custom-templates-not-loading-or-working-incorrectly)
* [Active user vs login prohibited user](#active-user-vs-login-prohibited-user)
* [Setting up logging](#setting-up-logging)
* [Adjusting your server for public/private use](#adjusting-your-server-for-public-private-use)
  * [Preventing spammers](#preventing-spammers)
  * [Only allow/block certain email domains](#only-allow-block-certain-email-domains)
  * [Issue only users](#issue-only-users)
* [Adding custom themes](#how-to-add-use-custom-themes)
* [SSHD vs built-in SSH](#sshd-vs-built-in-ssh)
* [Gitea is running slow](#why-is-gitea-running-slow)
* [Translation is incorrect/how to add more translations](#translation-is-incorrect-how-to-add-more-translations)
* [SSH Issues](#ssh-issues)
* [Missing releases after migration repository with tags](#missing-releases-after-migrating-repository-with-tags)
* [LFS Issues](#lfs-issues)


## How to migrate from Gogs/GitHub/etc. to Gitea
To migrate from Gogs to Gitea:

* [Gogs version 0.9.146 or less]({{< relref "doc/upgrade/from-gogs.en-us.md" >}})
* [Gogs version 0.11.46.0418](https://github.com/go-gitea/gitea/issues/4286)

To migrate from GitHub to Gitea, you can use Gitea's [Migrator tool](https://gitea.com/gitea/migrator)

To migrate from Gitlab to Gitea, you can use this non-affiliated tool:  
https://github.com/loganinak/MigrateGitlabToGogs

## Where does Gitea store "x" file
* WorkPath
  * Environment variable `GITEA_WORK_DIR`
  * Else binary location
* AppDataPath (default for database, indexers, etc.)
  * `APP_DATA_PATH` from `app.ini`
  * Else `%(WorkPath)/data`
* CustomPath (custom templates)
  * Environment variable `GITEA_CUSTOM`
  * Else `%(WorkPath)/custom`
* INI (config file)
  * `-c` flag
  * Else `%(CustomPath)/conf/app.ini`
* SQLite Database 
  * `PATH` in `database` section of `app.ini`
  * Else `%(AppDataPath)/gitea.db`

## Not seeing a clone URL or the clone URL being incorrect
There are a few places that could make this show incorrectly.

1. If using a reverse proxy, make sure you have followed the correction directions in the [reverse proxy guide]({{< relref "doc/usage/reverse-proxies.en-us.md" >}})
2. Make sure you have correctly set `ROOT_URL` in the `server` section of your `app.ini`

## Custom Templates not loading or working incorrectly
Gitea's custom templates must be added to the correct location or Gitea will not find and use them.  
To find the correct path, follow these steps:

1. Check if the environment variable is set:  
```
echo $GITEA_CUSTOM
```

2. If not, the default can be [found above](#where-does-gitea-store-x-file)
3. Once you have figured out the correct custom path, you can refer to the [customizing Gitea]({{< relref "doc/advanced/customizing-gitea.en-us.md" >}}) page to add your template to the correct location.

## Active user vs login prohibited user
In Gitea, an "active" user refers to a user that has activated their account via email.  
A "login prohibited" user is a user that is not allowed to log in to Gitea anymore

## Setting up logging 
* [Official Docs]({{< relref "doc/advanced/logging-documentation.en-us.md" >}})

## Adjusting your server for public/private use

### Preventing spammers
There are multiple things you can combine to prevent spammers.  

1. By only whitelisting certain domains with OpenID (see below)
2. Setting `ENABLE_CAPTCHA` to `true` in your `app.ini` and properly configuring `RECAPTCHA_SECRET` and `RECAPTCHA_SITEKEY`
3. Settings `DISABLE_REGISTRATION` to `true` and creating new users via the [CLI]({{< relref "doc/usage/command-line.en-us.md" >}}), [API]({{< relref "doc/advanced/api-usage.en-us.md" >}}), or Gitea's Admin UI  

[API Example](https://try.gitea.io/api/swagger)

### Only allow/block certain email domains
If using OpenID, you can configure `WHITELISTED_URIS` or `BLACKLISTED_URIS` in your `app.ini`  
**NOTE:** whitelisted takes precedence, so if it is non-blank then blacklisted is ignored

### Issue only users
The current way to achieve this is to create/modify a user with a max repo creation limit of 0.

## How to add/use custom themes
Gitea supports two official themes right now, `gitea` and `arc-green` (`light` and `dark` respectively)  
To add your own theme, currently the only way is to provide a complete theme (not just color overrides)  
  
As an example, let's say our theme is `arc-blue` (this is a real theme, and can be found [in this issue](https://github.com/go-gitea/gitea/issues/6011))  
Name the `.css` file `theme-arc-blue.css` and add it to your custom folder in `custom/pulic/css`  
Allow users to use it by adding `arc-blue` to the list of `THEMES` in your `app.ini`

## SSHD vs built-in SSH
SSHD is the built-in SSH server on most Unix systems.  
Gitea also provides its own SSH server, for usage when SSHD is not available.

## Why is Gitea running slow?
The most common culprit for this is loading federated avatars.  
This can be turned off by setting `ENABLE_FEDERATED_AVATAR` to `false` in your `app.ini`  
Another option that may need to be changed is setting `DISABLE_GRAVATAR` to `true` in your `app.ini`

## Translation is incorrect/how to add more translations
Our translations are currently crowd-sourced on our [Crowding project](https://crowdin.com/project/gitea)  
Whether you want to change a translation or add a new one, it will need to be there as all translations are overwritten in our CI via the Crowdin integration.

## SSH issues

For issues reaching repositories over `ssh` while the Gitea web front-end, but
`https` based git repository access works fine, consider looking into the following.

```
Permission denied (publickey).
fatal: Could not read from remote repository.
```

This error signifies that the server rejected a log in attempt, check the
following things:

* On the client:
  * Ensure the public and private ssh keys are added to the correct Gitea user.
  * Make sure there are no issues in the remote url. In particular, ensure the name of the
    git user (before the `@`) is spelled correctly.
  * Ensure public and private ssh keys are correct on client machine.
  * Try to connect using ssh (ssh git@myremote.example) to ensure a connection
    can be made.
* On the server:
  * Make sure the repository exists and is correctly named.
  * Check the permissions of the `.ssh` directory in the system user's home directory.
  * Verify that the correct public keys are added to `.ssh/authorized_keys`.
    Try to run `Rewrite '.ssh/authorized_keys' file (for Gitea SSH keys)` on the
    Gitea admin panel.
  * Read Gitea logs.
  * Read /var/log/auth (or similar).
  * Check permissions of repositories.

The following is an example of a missing public SSH key where authentication
succeeded, but some other setting is preventing SSH from reaching the correct
repository.

```
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

In this case, look into the following settings:

* On the server:
  * Make sure that the `git` system user has a usable shell set
    * Verify this with `getent passwd git | cut -d: -f7`
    * `usermod` or `chsh` can be used to modify this.
  * Ensure that the `gitea serv` command in `.ssh/authorized_keys` uses the
    correct configuration file.

## Missing releases after migrating repository with tags

To migrate an repository *with* all tags, you need to do two things:

* Push tags to the repository:
```
 git push --tags
 ```
 
 * (Re-)sync tags of all repositories within Gitea:
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
Have you checked the value of `LFS_HTTP_AUTH_EXPIRY` in your `app.ini` file? By default, your LFS token will expire after 20 minutes. If you have a slow connection or a large file (or both), it may not finish uploading within the time limit. 

You may want to set this value to `60m` or `120m`.
