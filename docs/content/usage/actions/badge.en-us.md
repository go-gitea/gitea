---
date: "2023-02-25T00:00:00+00:00"
title: "Badge"
slug: "badge"
sidebar_position: 11
toc: false
draft: false
menu:
  sidebar:
    parent: "actions"
    name: "Badge"
    sidebar_position: 11
    identifier: "Badge"
---

# Badge

Gitea has its builtin Badge system which allows you to display the status of your repository in other places. You can use the following badges:

## Workflow Badge

The Gitea Actions workflow badge is a badge that shows the status of the latest workflow run.
It is designed to be compatible with [GitHub Actions workflow badge](https://docs.github.com/en/actions/monitoring-and-troubleshooting-workflows/adding-a-workflow-status-badge).

You can use the following URL to get the badge:

```
https://your-gitea-instance.com/{owner}/{repo}/actions/workflows/{workflow_file}/badge.svg?branch={branch}&event={event}
```

- `{owner}`: The owner of the repository.
- `{repo}`: The name of the repository.
- `{workflow_file}`: The name of the workflow file.
- `{branch}`: Optional. The branch of the workflow. Default to your repository's default branch.
- `{event}`: Optional. The event of the workflow. Default to none.
