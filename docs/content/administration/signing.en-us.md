---
date: "2019-08-17T10:20:00+01:00"
title: "GPG Commit Signatures"
slug: "signing"
sidebar_position: 50
toc: false
draft: false
aliases:
  - /en-us/signing
menu:
  sidebar:
    parent: "administration"
    name: "GPG Commit Signatures"
    sidebar_position: 50
    identifier: "signing"
---

# GPG Commit Signatures

Gitea will verify GPG commit signatures in the provided tree by
checking if the commits are signed by a key within the Gitea database,
or if the commit matches the default key for Git.

Keys are not checked to determine if they have expired or revoked.
Keys are also not checked with keyservers.

A commit will be marked with a grey unlocked icon if no key can be
found to verify it. If a commit is marked with a red unlocked icon,
it is reported to be signed with a key with an id.

Please note: The signer of a commit does not have to be an author or
committer of a commit.

This functionality requires Git >= 1.7.9 but for full functionality
this requires Git >= 2.0.0.

## Automatic Signing

There are a number of places where Gitea will generate commits itself:

- Repository Initialisation
- Wiki Changes
- CRUD actions using the editor or the API
- Merges from Pull Requests

Depending on configuration and server trust you may want Gitea to
sign these commits.

## Installing and generating a GPG key for Gitea

It is up to a server administrator to determine how best to install
a signing key. Gitea generates all its commits using the server `git`
command at present - and therefore the server `gpg` will be used for
signing (if configured.) Administrators should review best-practices
for GPG - in particular it is probably advisable to only install a
signing secret subkey without the master signing and certifying secret
key.

## General Configuration

Gitea's configuration for signing can be found with the
`[repository.signing]` section of `app.ini`:

```ini
...
[repository.signing]
SIGNING_KEY = default
SIGNING_NAME =
SIGNING_EMAIL =
INITIAL_COMMIT = always
CRUD_ACTIONS = pubkey, twofa, parentsigned
WIKI = never
MERGES = pubkey, twofa, basesigned, commitssigned

...
```

### `SIGNING_KEY`

The first option to discuss is the `SIGNING_KEY`. There are three main
options:

- `none` - this prevents Gitea from signing any commits
- `default` - Gitea will default to the key configured within `git config`
- `KEYID` - Gitea will sign commits with the gpg key with the ID
  `KEYID`. In this case you should provide a `SIGNING_NAME` and
  `SIGNING_EMAIL` to be displayed for this key.

The `default` option will interrogate `git config` for
`commit.gpgsign` option - if this is set, then it will use the results
of the `user.signingkey`, `user.name` and `user.email` as appropriate.

Please note: by adjusting Git's `config` file within Gitea's
repositories, `SIGNING_KEY=default` could be used to provide different
signing keys on a per-repository basis. However, this is clearly not an
ideal UI and therefore subject to change.

**Since 1.17**, Gitea runs git in its own home directory `[git].HOME_PATH` (default to `%(APP_DATA_PATH)/home`)
and uses its own config `{[git].HOME_PATH}/.gitconfig`.
If you have your own customized git config for Gitea, you should set these configs in system git config (aka `/etc/gitconfig`)
or the Gitea internal git config `{[git].HOME_PATH}/.gitconfig`.
Related home files for git command (like `.gnupg`) should also be put in Gitea's git home directory `[git].HOME_PATH`.
If you like to keep the `.gnupg` directory outside of `{[git].HOME_PATH}/`, consider setting the `$GNUPGHOME` environment variable to your preferred location.

### `INITIAL_COMMIT`

This option determines whether Gitea should sign the initial commit
when creating a repository. The possible values are:

- `never`: Never sign
- `pubkey`: Only sign if the user has a public key
- `twofa`: Only sign if the user logs in with two factor authentication
- `always`: Always sign

Options other than `never` and `always` can be combined as a comma
separated list. The commit will be signed if all selected options are true.

### `WIKI`

This options determines if Gitea should sign commits to the Wiki.
The possible values are:

- `never`: Never sign
- `pubkey`: Only sign if the user has a public key
- `twofa`: Only sign if the user logs in with two-factor authentication
- `parentsigned`: Only sign if the parent commit is signed.
- `always`: Always sign

Options other than `never` and `always` can be combined as a comma
separated list. The commit will be signed if all selected options are true.

### `CRUD_ACTIONS`

This option determines if Gitea should sign commits from the web
editor or API CRUD actions. The possible values are:

- `never`: Never sign
- `pubkey`: Only sign if the user has a public key
- `twofa`: Only sign if the user logs in with two-factor authentication
- `parentsigned`: Only sign if the parent commit is signed.
- `always`: Always sign

Options other than `never` and `always` can be combined as a comma
separated list. The change will be signed if all selected options are true.

### `MERGES`

This option determines if Gitea should sign merge commits from PRs.
The possible options are:

- `never`: Never sign
- `pubkey`: Only sign if the user has a public key
- `twofa`: Only sign if the user logs in with two-factor authentication
- `basesigned`: Only sign if the parent commit in the base repo is signed.
- `headsigned`: Only sign if the head commit in the head branch is signed.
- `commitssigned`: Only sign if all the commits in the head branch to the merge point are signed.
- `approved`: Only sign approved merges to a protected branch.
- `always`: Always sign

Options other than `never` and `always` can be combined as a comma
separated list. The merge will be signed if all selected options are true.

## Obtaining the Public Key of the Signing Key

The public key used to sign Gitea's commits can be obtained from the API at:

```sh
/api/v1/signing-key.gpg
```

In cases where there is a repository specific key this can be obtained from:

```sh
/api/v1/repos/:username/:reponame/signing-key.gpg
```
