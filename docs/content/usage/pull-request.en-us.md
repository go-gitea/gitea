---
date: "2018-06-01T19:00:00+02:00"
title: "Pull Request"
slug: "pull-request"
sidebar_position: 13
toc: false
draft: false
aliases:
  - /en-us/pull-request
menu:
  sidebar:
    parent: "usage"
    name: "Pull Request"
    sidebar_position: 13
    identifier: "pull-request"
---

# Pull Request

A Pull Request (PR) is a way to propose changes to a repository.
It is a request to merge one branch into another, accompanied by a description of the changes that were made.
Pull Requests are commonly used as a way for contributors to propose changes and for maintainers to review and merge those changes.

## Creating a pull request

To create a PR, you'll need to follow these steps:

1. **Fork the repository** - If you don't have permission to make changes to the repository directly, you'll need to fork the repository to your own account.
This creates a copy of the repository that you can make changes to.

2. **Create a branch (optional)** - Create a new branch on your forked repository that contains the changes you want to propose.
Give the branch a descriptive name that indicates what the changes are for.

3. **Make your changes** - Make the changes you want, commit, and push them to your forked repository.

4. **Create the PR** - Go to the original repository and go to the "Pull Requests" tab. Click the "New Pull Request" button and select your new branch as the source branch.
Enter a descriptive title and description for your Pull Request and click "Create Pull Request".

## Reviewing a pull request

When a PR is created, it triggers a review process. The maintainers of the repository are notified of the PR and can review the changes that were made.
They can leave comments, request changes, or approve the changes.

If the maintainers request changes, you'll need to make those changes in your branch and push the changes to your forked repository.
The PR will be updated automatically with the new changes.

If the maintainers approve the changes, they can merge the PR into the repository.

## Closing a pull request

If you decide that you no longer want to merge a PR, you can close it.
To close a PR, go to the open PR and click the "Close Pull Request" button. This will close the PR without merging it.

## "Work In Progress" pull requests

Marking a pull request as being a work in progress will prevent that pull request from being accidentally merged.
To mark a pull request as being a work in progress, you must prefix its title by `WIP:` or `[WIP]` (case insensitive).
Those values are configurable in your `app.ini` file:

```ini
[repository.pull-request]
WORK_IN_PROGRESS_PREFIXES=WIP:,[WIP]
```

The first value of the list will be used in helpers.

## Pull Request Templates

You can find more information about pull request templates at the page [Issue and Pull Request templates](usage/issue-pull-request-templates.md).
