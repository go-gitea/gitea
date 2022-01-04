---
date: "2021-02-02"
title: "Clone filters (partial clone)"
slug: "clone-filters"
weight: 25
draft: false
toc: false
menu:
  sidebar:
    parent: "advanced"
    name: "Clone filters"
    weight: 25
    identifier: "clone-filters"
---

# Clone filters (partial clone)

Git introduces `--filter` option to `git clone` command, which filters out
large files and objects (such as blobs) to create partial clone of a repo.
Clone filters are especially useful for large repo and/or metered connection,
where full clone (without `--filter`) can be expensive (as all history data
must be downloaded).

This requires Git version 2.22 or later, both on the Gitea server and on the
client. For clone filters to work properly, make sure that Git version
on the client is at least the same as on the server (or later). Login to
Gitea server as admin and head to Site Administration -> Configuration to
see Git version of the server.

By default, clone filters are disabled, which cause the server to ignore
`--filter` option.

To enable clone filters on per-repo basis, edit the repo's `config` on
repository location. Consult `ROOT` option on `repository` section of
Gitea configuration (`app.ini`) for the exact location. For example, to
enable clone filters for `some-repo`, edit
`/var/gitea/data/gitea-repositories/some-user/some-repo.git/config` and add:

```ini
[uploadpack]
	allowfilter = true
```

To enable clone filters globally, add that config above to `~/.gitconfig`
of user that run Gitea (for example `git`).

Alternatively, you can use `git config` to set the option.

To enable for a specific repo:

```bash
cd /var/gitea/data/gitea-repositories/some-user/some-repo.git
git config --local uploadpack.allowfilter true
```
To enable globally, login as user that run Gitea and:

```bash
git config --global uploadpack.allowfilter true
```

See [GitHub blog post: Get up to speed with partial clone](https://github.blog/2020-12-21-get-up-to-speed-with-partial-clone-and-shallow-clone/)
for common use cases of clone filters (blobless and treeless clones), and
[GitLab docs for partial clone](https://docs.gitlab.com/ee/topics/git/partial_clone.html)
for more advanced use cases (such as filter by file size and remove
filters to turn partial clone into full clone).
