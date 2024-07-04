---
date: "2022-08-31T17:35:40+08:00"
title: "Merge Message templates"
slug: "merge-message-templates"
sidebar_position: 15
toc: false
draft: false
aliases:
  - /en-us/merge-message-templates
menu:
  sidebar:
    parent: "usage"
    name: "Merge Message templates"
    sidebar_position: 15
    identifier: "merge-message-templates"
---

# Merge Message templates

## File names

Possible file names for PR default merge message templates:

- `.gitea/default_merge_message/MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/SQUASH_TEMPLATE.md`
- `.gitea/default_merge_message/MANUALLY-MERGED_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-UPDATE-ONLY_TEMPLATE.md`

## Variables

You can use the following variables enclosed in `${}` inside these templates which follow [os.Expand](https://pkg.go.dev/os#Expand) syntax:

- BaseRepoOwnerName: Base repository owner name of this pull request
- BaseRepoName: Base repository name of this pull request
- BaseBranch: Base repository target branch name of this pull request
- HeadRepoOwnerName: Head repository owner name of this pull request
- HeadRepoName: Head repository name of this pull request
- HeadBranch: Head repository branch name of this pull request
- PullRequestTitle: Pull request's title
- PullRequestDescription: Pull request's description
- PullRequestPosterName: Pull request's poster name
- PullRequestIndex: Pull request's index number
- PullRequestReference: Pull request's reference char with index number. i.e. #1, !2
- ClosingIssues: return a string contains all issues which will be closed by this pull request i.e. `close #1, close #2`
- ReviewedOn: Which pull request this commit belongs to. For example `Reviewed-on: https://gitea.com/foo/bar/pulls/1`
- ReviewedBy: Who approved the pull request before the merge. For example `Reviewed-by: Jane Doe <jane.doe@example.com>`

## Rebase

When rebasing without a merge commit, `REBASE_TEMPLATE.md` modifies the message of the last commit. The following additional variables are available in this template:

- CommitTitle: Commit's title
- CommitBody: Commits's body text
