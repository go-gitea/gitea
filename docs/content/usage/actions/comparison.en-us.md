---
date: "2023-04-27T15:00:00+08:00"
title: "Compared to GitHub Actions"
slug: "comparison"
sidebar_position: 30
draft: false
toc: false
menu:
  sidebar:
    parent: "actions"
    name: "Comparison"
    sidebar_position: 30
    identifier: "actions-comparison"
---

# Compared to GitHub Actions

Even though Gitea Actions is designed to be compatible with GitHub Actions, there are some differences between them.

## Additional features

### Absolute action URLs

Gitea Actions supports defining actions via absolute URL, which means that you can use actions from any git repository.
Like `uses: https://github.com/actions/checkout@v4` or `uses: http://your_gitea.com/owner/repo@branch`.

### Actions written in Go

Gitea Actions supports writing actions in Go.
See [Creating Go Actions](https://blog.gitea.com/creating-go-actions/).

### Support the non-standard syntax @yearly, @monthly, @weekly, @daily, @hourly on schedule

Github Actions doesn't support that. https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#schedule

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

### Problem Matchers

Problem Matchers are a way to scan the output of actions for a specified regex pattern and surface that information prominently in the UI.
See [Problem matchers](https://github.com/actions/toolkit/blob/main/docs/problem-matchers.md).

It's ignored by Gitea Actions now.

### Create an error annotation

See [Creating an annotation for an error](https://docs.github.com/en/actions/using-workflows/workflow-commands-for-github-actions#example-creating-an-annotation-for-an-error)

It's ignored by Gitea Actions now.

### Expressions

For [expressions](https://docs.github.com/en/actions/learn-github-actions/expressions), only [`always()`](https://docs.github.com/en/actions/learn-github-actions/expressions#always) is supported.

## Missing UI features

### Pre and Post steps

Pre and Post steps don't have their own section in the job log user interface.

### Services steps

Services steps don't have their own section in the job log user interface.

## Different behavior

### Downloading actions

Previously (Pre 1.21.0), `[actions].DEFAULT_ACTIONS_URL` defaulted to `https://gitea.com`.
We have since restricted this option to only allow two values (`github` and `self`).
When set to `github`, the new default, Gitea will download non-fully-qualified actions from `https://github.com`.
For example, if you use `uses: actions/checkout@v4`, it will download the checkout repository from `https://github.com/actions/checkout.git`.

If you want to download an action from another git hoster, you can use an absolute URL, e.g. `uses: https://gitea.com/actions/checkout@v4`.

If your Gitea instance is in an intranet or a restricted area, you can set the URL to `self` to only download actions from your own instance by default.
Of course, you can still use absolute URLs in workflows.

More details about the `[actions].DEFAULT_ACTIONS_URL` configuration can be found in the [Configuration Cheat Sheet](administration/config-cheat-sheet.md#actions-actions)。

### Context availability

Context availability is not checked, so you can use the env context on more places.
See [Context availability](https://docs.github.com/en/actions/learn-github-actions/contexts#context-availability).
