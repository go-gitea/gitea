---
date: "2016-11-08T16:00:00+02:00"
title: "Troubleshooting"
slug: "troubleshooting"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "help"
    name: "Troubleshooting"
    weight: 20
    identifier: "troubleshooting"
---

# Troubleshooting

This page contains some common seen issues and their solutions.

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

