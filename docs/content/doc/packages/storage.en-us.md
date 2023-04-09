---
date: "2022-11-01T00:00:00+00:00"
title: "Storage"
slug: "usage/packages/storage"
weight: 5
draft: false
toc: false
menu:
  sidebar:
    parent: "packages"
    name: "Storage"
    weight: 5
    identifier: "storage"
---

# Storage

This document describes the storage of the package registry and how it can be managed.

**Table of Contents**

{{< toc >}}

## Deduplication

The package registry has a build-in deduplication of uploaded blobs.
If two identical files are uploaded only one blob is saved on the filesystem.
This ensures no space is wasted for duplicated files.

If two packages are uploaded with identical files, both packages will display the same size but on the filesystem they require only half of the size.
Whenever a package gets deleted only the references to the underlaying blobs are removed.
The blobs get not removed at this moment, so they still require space on the filesystem.
When a new package gets uploaded the existing blobs may get referenced again.

These unreferenced blobs get deleted by a [clean up job]({{< relref "doc/administration/config-cheat-sheet.en-us.md#cron---cleanup-expired-packages-croncleanup_packages" >}}).
The config setting `OLDER_THAN` configures how long unreferenced blobs are kept before they get deleted.

## Cleanup Rules

Package registries can become large over time without cleanup.
It's recommended to delete unnecessary packages and set up cleanup rules to automatically manage the package registry usage.
Every package owner (user or organization) manages the cleanup rules which are applied to their packages.

|Setting|Description|
|-|-|
|Enabled|Turn the cleanup rule on or off.|
|Type|Every rule manages a specific package type.|
|Apply pattern to full package name|If enabled, the patterns below are applied to the full package name (`package/version`). Otherwise only the version (`version`) is used.|
|Keep the most recent|How many versions to *always* keep for each package.|
|Keep versions matching|The regex pattern that determines which versions to keep. An empty pattern keeps no version while `.+` keeps all versions. The container registry will always keep the `latest` version even if not configured.|
|Remove versions older than|Remove only versions older than the selected days.|
|Remove versions matching|The regex pattern that determines which versions to remove. An empty pattern or `.+` leads to the removal of every package if no other setting tells otherwise.|

Every cleanup rule can show a preview of the affected packages.
This can be used to check if the cleanup rules is proper configured.

### Regex examples

Regex patterns are automatically surrounded with `\A` and `\z` anchors.
Do not include any `\A`, `\z`, `^` or `$` token in the regex patterns as they are not necessary.
The patterns are case-insensitive which matches the behaviour of the package registry in Gitea.

|Pattern|Description|
|-|-|
|`.*`|Match every possible version.|
|`v.+`|Match versions that start with `v`.|
|`release`|Match only the version `release`.|
|`release.*`|Match versions that are either named or start with `release`.|
|`.+-temp-.+`|Match versions that contain `-temp-`.|
|`v.+\|release`|Match versions that either start with `v` or are named `release`.|
|`package/v.+\|other/release`|Match versions of the package `package` that start with `v` or the version `release` of the package `other`. This needs the setting *Apply pattern to full package name* enabled.|

### How the cleanup rules work

The cleanup rules are part of the [clean up job]({{< relref "doc/administration/config-cheat-sheet.en-us.md#cron---cleanup-expired-packages-croncleanup_packages" >}}) and run periodically.

The cleanup rule:

1. Collects all packages of the package type for the owners registry.
1. For every package it collects all versions.
1. Excludes from the list the # versions based on the *Keep the most recent* value.
1. Excludes from the list any versions matching the *Keep versions matching* value.
1. Excludes from the list the versions more recent than the *Remove versions older than* value.
1. Excludes from the list any versions not matching the *Remove versions matching* value.
1. Deletes the remaining versions.
