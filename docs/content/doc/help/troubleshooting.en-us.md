---
date: "2016-11-08T16:00:00+02:00"
title: "Troubleshooting"
slug: "troubleshooting"
weight: 10
toc: true
draft: false
menu:
  sidebar:
    parent: "Help"
    name: "Troubleshooting"
    weight: 20
    identifier: "troubleshooting"
---

# Troubleshooting

This page contains some common issues you can run into and their solutions.

## SSH issues

If you are having issues with reaching your repositories over `ssh` while the
Gitea web front-end and `https` based git operations work fine, consider
looking at the following items.

```
Permission denied (publickey).
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

This error signifies that the server rejected your log in attempt, check the
following things:

* On the client:
  * Ensure the public and private ssh keys are added to the correct Gitea user.
  * Make sure there are no issues in your remote url, ensure the name of the
    git user (before the `@`) is spelled correctly.
  * Ensure the public and private ssh keys are available and reachable on the
    client machine.
  * Try to `ssh git@myremote.example` to ensure that everything is set up
    properly.
* On the server:
  * Check the permissions of the `.ssh` directory in the home directory of your
    `git` user.
  * Verify that the correct public keys are added to `.ssh/authorized_keys`.
    Try to run `Rewrite '.ssh/authorized_keys' file (for Gitea SSH keys)` on the
    Gitea admin panel.

If you get a similar error without the public key part (shown below) then
authentication succeeded, but some other setting is preventing ssh from
reaching the correct repository.

```
fatal: Could not read from remote repository.

Please make sure you have the correct access rights
and the repository exists.
```

In this case, look into the following settings:

* On the server:
  * Make sure that your `git` user has a usable shell set. You can verify this
    with `getent passwd git | cut -d: -f7`, `chsh` can be used to modify this.
  * Ensure that the `gitea serv` command in `.ssh/authorized_keys` uses the
    proper configuration file.
