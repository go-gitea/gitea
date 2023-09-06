---
date: "2021-02-02"
title: "Clone filters (partial clone)"
slug: "clone-filters"
sidebar_position: 25
draft: false
toc: false
aliases:
  - /en-us/clone-filters
menu:
  sidebar:
    parent: "usage"
    name: "Clone filters"
    sidebar_position: 25
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

By default, clone filters are enabled, unless `DISABLE_PARTIAL_CLONE` under
`[git]` is set to `true`.

See [GitHub blog post: Get up to speed with partial clone](https://github.blog/2020-12-21-get-up-to-speed-with-partial-clone-and-shallow-clone/)
for common use cases of clone filters (blobless and treeless clones), and
[GitLab docs for partial clone](https://docs.gitlab.com/ee/topics/git/partial_clone.html)
for more advanced use cases (such as filter by file size and remove
filters to turn partial clone into full clone).
