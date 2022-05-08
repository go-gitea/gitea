---
date: "2018-05-10T16:00:00+02:00"
title: "Usage: Issue and Pull Request templates"
slug: "issue-pull-request-templates"
weight: 15
toc: false
draft: false
menu:
  sidebar:
    parent: "usage"
    name: "Issue and Pull Request templates"
    weight: 15
    identifier: "issue-pull-request-templates"
---

# Issue and Pull Request Templates

**Table of Contents**

{{< toc >}}

Some projects have a standard list of questions that users need to answer
when creating an issue or pull request. Gitea supports adding templates to the
main branch of the repository so that they can autopopulate the form when users are
creating issues and pull requests. This will cut down on the initial back and forth
of getting some clarifying details.

Possible file names for issue templates:

- `ISSUE_TEMPLATE.md`
- `issue_template.md`
- `.gitea/ISSUE_TEMPLATE.md`
- `.gitea/issue_template.md`
- `.github/ISSUE_TEMPLATE.md`
- `.github/issue_template.md`

Possible file names for PR templates:

- `PULL_REQUEST_TEMPLATE.md`
- `pull_request_template.md`
- `.gitea/PULL_REQUEST_TEMPLATE.md`
- `.gitea/pull_request_template.md`
- `.github/PULL_REQUEST_TEMPLATE.md`
- `.github/pull_request_template.md`

Possible file names for PR default merge message templates:

- `.gitea/default_merge_message/MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/SQUASH_TEMPLATE.md`
- `.gitea/default_merge_message/MANUALLY-MERGED_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-UPDATE-ONLY_TEMPLATE.md`

Possible file names for PR default merge message templates:

- `.gitea/default_merge_message/MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-MERGE_TEMPLATE.md`
- `.gitea/default_merge_message/SQUASH_TEMPLATE.md`
- `.gitea/default_merge_message/MANUALLY-MERGED_TEMPLATE.md`
- `.gitea/default_merge_message/REBASE-UPDATE-ONLY_TEMPLATE.md`

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

Additionally, the New Issue page URL can be suffixed with `?title=Issue+Title&body=Issue+Text` and the form will be populated with those strings. Those strings will be used instead of the template if there is one.

## Issue Template Directory

Alternatively, users can create multiple issue templates inside a special directory and allow users to choose one that more specifically
addresses their problem.

Possible directory names for issue templates:

- `ISSUE_TEMPLATE`
- `issue_template`
- `.gitea/ISSUE_TEMPLATE`
- `.gitea/issue_template`
- `.github/ISSUE_TEMPLATE`
- `.github/issue_template`
- `.gitlab/ISSUE_TEMPLATE`
- `.gitlab/issue_template`

Inside the directory can be multiple issue templates with the form

```md
---

name: "Template Name"
about: "This template is for testing!"
title: "[TEST] "
ref: "main"
labels:

- bug
- "help needed"

---

This is the template!
```

In the above example, when a user is presented with the list of issues they can submit, this would show as `Template Name` with the description
`This template is for testing!`. When submitting an issue with the above example, the issue title would be pre-populated with
`[TEST] ` while the issue body would be pre-populated with `This is the template!`. The issue would also be assigned two labels,
`bug` and `help needed`, and the issue will have a reference to `main`.
