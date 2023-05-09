---
date: "2023-04-27T15:00:00+08:00"
title: "Compared to GitHub Actions"
slug: "usage/actions/comparison"
weight: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "Comparison"
    weight: 30
    identifier: "actions-comparison"
---

# Compared to GitHub Actions

Even though Gitea Actions is designed to be compatible with GitHub Actions, there are some differences between them.

**Table of Contents**

{{< toc >}}

## Additional features

### Absolute action URLs

Gitea Actions supports defining actions via absolute URL, which means that you can use actions from any git repository.
Like `uses: https://github.com/actions/checkout@v3` or `uses: http://your_gitea.com/owner/repo@branch`.

### Actions written in Go

Gitea Actions supports writing actions in Go.
See [Creating Go Actions](https://blog.gitea.io/2023/04/creating-go-actions/).

## Unsupported workflows syntax

### `concurrency`

It's used to run a single job at a time.
See [Using concurrency](https://docs.github.com/en/actions/using-jobs/using-concurrency).

It's ignored by Gitea Actions now.

### `run-name`

The name for workflow runs generated from the workflow.
See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#run-name).

It's ignored by Gitea Actions now.

### `permissions` and `jobs.<job_id>.permissions`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#permissions).

It's ignored by Gitea Actions now.

### `jobs.<job_id>.timeout-minutes`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idtimeout-minutes).

It's ignored by Gitea Actions now.

### `jobs.<job_id>.continue-on-error`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idcontinue-on-error).

It's ignored by Gitea Actions now.

### `jobs.<job_id>.environment`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idenvironment).

It's ignored by Gitea Actions now.

### Complex `runs-on`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#jobsjob_idruns-on).

Gitea Actions only supports `runs-on: xyz` or `runs-on: [xyz]` now.

### `workflow_dispatch`

See [Workflow syntax for GitHub Actions](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#onworkflow_dispatch).

It's ignored by Gitea Actions now.

### `hashFiles` expression

See [Expressions](https://docs.github.com/en/actions/learn-github-actions/expressions#hashfiles)

Gitea Actions doesn't support it now, if you use it, the result will always be empty string.

As a workaround, you can use [go-hashfiles](https://gitea.com/actions/go-hashfiles) instead.

## Missing features

### Variables

See [Variables](https://docs.github.com/en/actions/learn-github-actions/variables).

It's under development.

### Problem Matchers

Problem Matchers are a way to scan the output of actions for a specified regex pattern and surface that information prominently in the UI.
See [Problem matchers](https://github.com/actions/toolkit/blob/main/docs/problem-matchers.md).

It's ignored by Gitea Actions now.

### Create an error annotation

See [Creating an annotation for an error](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#example-creating-an-annotation-for-an-error)

It's ignored by Gitea Actions now.

## Missing UI features

### Pre and Post steps

Pre and Post steps don't have their own section in the job log user interface.

## Different behavior

### Downloading actions

Gitea Actions doesn't download actions from GitHub by default.
"By default" means that you don't specify the host in the `uses` field, like `uses: actions/checkout@v3`.
As a contrast, `uses: https://github.com/actions/checkout@v3` has specified host.

The missing host will be filled with `https://gitea.com` if you don't configure it.
That means `uses: actions/checkout@v3` will download the action from [gitea.com/actions/checkout](https://gitea.com/actions/checkout), instead of [github.com/actions/checkout](https://github.com/actions/checkout).

As mentioned, it's configurable.
If you want your runners to download actions from GitHub or your own Gitea instance by default, you can configure it by setting `[actions].DEFAULT_ACTIONS_URL`. See [Configuration Cheat Sheet](({{ < relref "doc/administration/config-cheat-sheet.en-us.md#actions-actions" > }})).

### Context availability

Context availability is not checked, so you can use the env context on more places.
See [Context availability](https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability).

## Known issues

### `docker/build-push-action@v4`

See [act_runner#119](https://gitea.com/gitea/act_runner/issues/119#issuecomment-738294).

`ACTIONS_RUNTIME_TOKEN` is a random string in Gitea Actions, not a JWT.
But the `docker/build-push-action@v4` tries to parse the token as JWT and doesn't handle the error, so the job fails.

There are two workarounds:

Set the `ACTIONS_RUNTIME_TOKEN` to empty manually, like:

``` yml
- name: Build and push
  uses: docker/build-push-action@v4
  env:
    ACTIONS_RUNTIME_TOKEN: ''
  with:
...
```

The bug has been fixed in a newer [commit](https://gitea.com/docker/build-push-action/commit/d8823bfaed2a82c6f5d4799a2f8e86173c461aba?style=split&whitespace=show-all#diff-1af9a5bdf96ddff3a2f3427ed520b7005e9564ad), but it has not been released. So you could use the latest version by specifying the branch name, like:

``` yml
- name: Build and push
  uses: docker/build-push-action@master
  with:
...
```
