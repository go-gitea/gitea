# Contribution Guidelines

<details><summary>Table of Contents</summary>

- [Contribution Guidelines](#contribution-guidelines)
  - [Introduction](#introduction)
  - [Issues](#issues)
    - [How to report issues](#how-to-report-issues)
    - [Types of issues](#types-of-issues)
    - [Discuss your design before the implementation](#discuss-your-design-before-the-implementation)
  - [Building Gitea](#building-gitea)
  - [Dependencies](#dependencies)
    - [Backend](#backend)
    - [Frontend](#frontend)
  - [Design guideline](#design-guideline)
  - [Styleguide](#styleguide)
  - [Copyright](#copyright)
  - [Testing](#testing)
  - [Translation](#translation)
  - [Code review](#code-review)
    - [Pull request format](#pull-request-format)
    - [PR title and summary](#pr-title-and-summary)
    - [Milestone](#milestone)
    - [Labels](#labels)
    - [Breaking PRs](#breaking-prs)
      - [What is a breaking PR?](#what-is-a-breaking-pr)
      - [How to handle breaking PRs?](#how-to-handle-breaking-prs)
    - [Maintaining open PRs](#maintaining-open-prs)
    - [Getting PRs merged](#getting-prs-merged)
    - [Final call](#final-call)
    - [Commit messages](#commit-messages)
      - [PR Co-authors](#pr-co-authors)
      - [PRs targeting `main`](#prs-targeting-main)
      - [Backport PRs](#backport-prs)
  - [Documentation](#documentation)
  - [API v1](#api-v1)
    - [GitHub API compatability](#github-api-compatability)
    - [Adding/Maintaining API routes](#addingmaintaining-api-routes)
    - [When to use what HTTP method](#when-to-use-what-http-method)
    - [Requirements for API routes](#requirements-for-api-routes)
  - [Backports and Frontports](#backports-and-frontports)
    - [What is backported?](#what-is-backported)
    - [How to backport?](#how-to-backport)
    - [Format of backport PRs](#format-of-backport-prs)
    - [Frontports](#frontports)
  - [Developer Certificate of Origin (DCO)](#developer-certificate-of-origin-dco)
  - [Release Cycle](#release-cycle)
  - [Maintainers](#maintainers)
  - [Technical Oversight Committee (TOC)](#technical-oversight-committee-toc)
    - [Current TOC members](#current-toc-members)
    - [Previous TOC/owners members](#previous-tocowners-members)
  - [Governance Compensation](#governance-compensation)
  - [TOC \& Working groups](#toc--working-groups)
  - [Roadmap](#roadmap)
  - [Versions](#versions)
  - [Releasing Gitea](#releasing-gitea)

</details>

## Introduction

This document explains how to contribute changes to the Gitea project. \
It assumes you have followed the [installation instructions](https://docs.gitea.io/en-us/). \
Sensitive security-related issues should be reported to [security@gitea.io](mailto:security@gitea.io).

For configuring IDEs for Gitea development, see the [contributed IDE configurations](contrib/ide/).

## Issues

### How to report issues

Please search the issues on the issue tracker with a variety of related keywords to ensure that your issue has not already been reported.

If your issue has not been reported yet, [open an issue](https://github.com/go-gitea/gitea/issues/new)
and answer the questions so we can understand and reproduce the problematic behavior. \
Please write clear and concise instructions so that we can reproduce the behavior — even if it seems obvious. \
The more detailed and specific you are, the faster we can fix the issue. \
It is really helpful if you can reproduce your problem on a site running on the latest commits, i.e. <https://try.gitea.io>, as perhaps your problem has already been fixed on a current version. \
Please follow the guidelines described in [How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html) for your report.

Please be kind, remember that Gitea comes at no cost to you, and you're getting free help.

### Types of issues

Typically, issues fall in one of the following categories:

- `bug`: Something in the frontend or backend behaves unexpectedly
- `security issue`: bug that has serious implications such as leaking another users data. Please do not file such issues on the public tracker and send a mail to security@gitea.io instead
- `feature`: Completely new functionality. You should describe this feature in enough detail that anyone who reads the issue can understand how it is supposed to be implemented
- `enhancement`: An existing feature should get an upgrade
- `refactoring`: Parts of the code base don't conform with other parts and should be changed to improve Gitea's maintainability

### Discuss your design before the implementation

We welcome submissions. \
If you want to change or add something, please let everyone know what you're working on — [file an issue](https://github.com/go-gitea/gitea/issues/new) or comment on an existing one before starting your work!

Significant changes such as new features must go through the change proposal process before they can be accepted. \
This is mainly to save yourself the trouble of implementing it, only to find out that your proposed implementation has some potential problems. \
Furthermore, this process gives everyone a chance to validate the design, helps prevent duplication of effort, and ensures that the idea fits inside
the goals for the project and tools.

Pull requests should not be the place for architecture discussions.

## Building Gitea

See the [development setup instructions](https://docs.gitea.com/development/hacking-on-gitea).

## Dependencies

### Backend

Go dependencies are managed using [Go Modules](https://golang.org/cmd/go/#hdr-Module_maintenance). \
You can find more details in the [go mod documentation](https://go.dev/ref/mod) and the [Go Modules Wiki](https://github.com/golang/go/wiki/Modules).

Pull requests should only modify `go.mod` and `go.sum` where it is related to your change, be it a bugfix or a new feature. \
Apart from that, these files should only be modified by Pull Requests whose only purpose is to update dependencies.

The `go.mod`, `go.sum` update needs to be justified as part of the PR description,
and must be verified by the reviewers and/or merger to always reference
an existing upstream commit.

### Frontend

For the frontend, we use [npm](https://www.npmjs.com/).

The same restrictions apply for frontend dependencies as for backend dependencies, with the exceptions that the files for it are `package.json` and `package-lock.json`, and that new versions must always reference an existing version.

## Design guideline

Depending on your change, please read the

- [backend development guideline](https://docs.gitea.com/contributing/guidelines-backend)
- [frontend development guideline](https://docs.gitea.com/contributing/guidelines-frontend)
- [refactoring guideline](https://docs.gitea.com/contributing/guidelines-refactoring)

## Styleguide

You should always run `make fmt` before committing to conform to Gitea's styleguide.

## Copyright

New code files that you contribute should use the standard copyright header:

```
// Copyright <current year> The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT
```

Afterwards, copyright should only be modified when the copyright author changes.

## Testing

Before submitting a pull request, run all tests to make sure your changes don't cause a regression elsewhere.

Here's how to run the test suite:

- code lint

|                       |                                                                   |
| :-------------------- | :---------------------------------------------------------------- |
|``make lint``          | lint everything (not needed if you only change the front- **or** backend)    |
|``make lint-frontend`` | lint frontend files  |
|``make lint-backend``  | lint backend files   |

- run tests (we suggest running them on Linux)

|  Command                               | Action                                           |              |
| :------------------------------------- | :----------------------------------------------- | ------------ |
|``make test[\#SpecificTestName]``       |  run unit test(s)  |
|``make test-sqlite[\#SpecificTestName]``|  run [integration](tests/integration) test(s) for SQLite |[More details](tests/integration/README.md)  |
|``make test-e2e-sqlite[\#SpecificTestName]``|  run [end-to-end](tests/e2e) test(s) for SQLite |[More details](tests/e2e/README.md)  |

## Translation

All translation work happens on [Crowdin](https://crowdin.com/project/gitea).
The only translation that is maintained in this repository is [the English translation](https://github.com/go-gitea/gitea/blob/main/options/locale/locale_en-US.ini).
It is synced regularly with Crowdin. \
Other locales on main branch **should not** be updated manually as they will be overwritten with each sync. \
Once a language has reached a **satisfactory percentage** of translated keys (~25%), it will be synced back into this repo and included in the next released version.

The tool `go run build/backport-locale.go` can be used to backport locales from the main branch to release branches that were missed.

## Code review

### Pull request format

Please try to make your pull request easy to review for us. \
For that, please read the [*Best Practices for Faster Reviews*](https://github.com/kubernetes/community/blob/261cb0fd089b64002c91e8eddceebf032462ccd6/contributors/guide/pull-requests.md#best-practices-for-faster-reviews) guide. \
It has lots of useful tips for any project you may want to contribute to. \
Some of the key points:

- Make small pull requests. \
  The smaller, the faster to review and the more likely it will be merged soon.
- Don't make changes unrelated to your PR. \
  Maybe there are typos on some comments, maybe refactoring would be welcome on a function... \
  but if that is not related to your PR, please make *another* PR for that.
- Split big pull requests into multiple small ones. \
  An incremental change will be faster to review than a huge PR.
- Allow edits by maintainers. This way, the maintainers will take care of merging the PR later on instead of you.

### PR title and summary

In the PR title, describe the problem you are fixing, not how you are fixing it. \
Use the first comment as a summary of your PR. \
In the PR summary, you can describe exactly how you are fixing this problem. \
Keep this summary up-to-date as the PR evolves. \
If your PR changes the UI, you must add **after** screenshots in the PR summary. \
If you are not implementing a new feature, you should also post **before** screenshots for comparison. \
If your PR closes some issues, you must note that in a way that both GitHub and Gitea understand, i.e. by appending a paragraph like

```text
Fixes/Closes/Resolves #<ISSUE_NR_X>.
Fixes/Closes/Resolves #<ISSUE_NR_Y>.
```

to your summary. \
Each issue that will be closed must stand on a separate line.

### Milestone

A PR should only be assigned to a milestone if it will likely be merged into the given version. \
As a rule of thumb, assume that a PR will stay open for an additional month for every 100 added lines. \
PRs without a milestone may not be merged.

### Labels

Every PR should be labeled correctly with every label that applies. \
This includes especially the distinction between `bug` (fixing existing functionality), `feature` (new functionality), `enhancement` (upgrades for existing functionality), and `refactoring` (improving the internal code structure without changing the output (much)). \
Furthermore,

- the amount of pending required approvals
- whether this PR is `blocked`, a `backport` or `breaking`
- if it targets the `ui` or `api`
- if it increases the application `speed`
- reduces `memory usage`

are oftentimes notable labels.

### Breaking PRs

#### What is a breaking PR?

A PR is breaking if it meets one of the following criteria:

- It changes API output in an incompatible way for existing users
- It removes a setting that an admin could previously set (i.e. via `app.ini`)
- An admin must do something manually to restore the old behavior

In particular, this means that adding new settings is not breaking.\
Changing the default value of a setting or replacing the setting with another one is breaking, however.

#### How to handle breaking PRs?

If your PR has a breaking change, you must add a `BREAKING` section to your PR summary, e.g.

```
## :warning: BREAKING :warning:
```

To explain how this will affect users and how to mitigate these changes.

### Maintaining open PRs

The moment you create a non-draft PR or the moment you convert a draft PR to a non-draft PR is the moment code review starts for it. \
Once that happens, do not rebase or squash your branch anymore as it makes it difficult to review the new changes. \
Merge the base branch into your branch only when you really need to, i.e. because of conflicting changes in the mean time. \
This reduces unnecessary CI runs. \
Don't worry about merge commits messing up your commit history as every PR will be squash merged. \
This means that all changes are joined into a single new commit whose message is as described below.

### Getting PRs merged

Changes to Gitea must be reviewed before they are accepted — no matter who
makes the change, even if they are an owner or a maintainer. \
The only exception are critical bugs that prevent Gitea from being compiled or started. \
Specifically, we require two approvals from maintainers for every PR. \
Once this criteria has been met, your PR receives the `lgtm/done` label. \
From this point on, your only responsibility is to fix merge conflicts or respond to/implement requests by maintainers. \
It is the responsibility of the maintainers from this point to get your PR merged.

If a PR has the `lgtm/done` label and there are no open discussions or merge conflicts anymore, any maintainer can add the `reviewed/wait-merge` label. \
This label means that the PR is part of the merge queue and will be merged as soon as possible. \
The merge queue will be cleared in the order of the list below:

<https://github.com/go-gitea/gitea/pulls?q=is%3Apr+label%3Areviewed%2Fwait-merge+sort%3Acreated-asc+is%3Aopen>

Gitea uses it's own tool, the <https://github.com/GiteaBot/gitea-backporter> to automate parts of the review process. \
This tool does the things listed below automatically:

- create a backport PR if needed once the initial PR was merged
- remove the PR from the merge queue after the PR merged
- keep the oldest branch in the merge queue up to date with merges

### Final call

If a PR has been ignored for more than 7 days with no comments or reviews, and the author or any maintainer believes it will not survive a long wait (such as a refactoring PR), they can send "final call" to the TOC by mentioning them in a comment.

After another 7 days, if there is still zero approval, this is considered a polite refusal, and the PR will be closed to avoid wasting further time. Therefore, the "final call" has a cost, and should be used cautiously.

However, if there are no objections from maintainers, the PR can be merged with only one approval from the TOC (not the author).

### Commit messages

Mergers are able and required to rewrite the PR title and summary (the first comment of a PR) so that it can produce an easily understandable commit message if necessary. \
The final commit message should no longer contain any uncertainty such as `hopefully, <x> won't happen anymore`. Replace uncertainty with certainty.

#### PR Co-authors

A person counts as a PR co-author the moment they (co-)authored a commit that is not simply a `Merge base branch into branch` commit. \
Mergers are required to remove such "false-positive" co-authors when writing the commit message. \
The true co-authors must remain in the commit message.

#### PRs targeting `main`

The commit message of PRs targeting `main` is always

```bash
$PR_TITLE ($PR_INDEX)

$REWRITTEN_PR_SUMMARY
```

#### Backport PRs

The commit message of backport PRs is always

```bash
$PR_TITLE ($INITIAL_PR_INDEX) ($BACKPORT_PR_INDEX)

$REWRITTEN_PR_SUMMARY
```

## Documentation

If you add a new feature or change an existing aspect of Gitea, the documentation for that feature must be created or updated in the same PR.

## API v1

The API is documented by [swagger](http://try.gitea.io/api/swagger) and is based on [the GitHub API](https://docs.github.com/en/rest).

### GitHub API compatability

Gitea's API should use the same endpoints and fields as the GitHub API as far as possible, unless there are good reasons to deviate. \
If Gitea provides functionality that GitHub does not, a new endpoint can be created. \
If information is provided by Gitea that is not provided by the GitHub API, a new field can be used that doesn't collide with any GitHub fields. \
Updating an existing API should not remove existing fields unless there is a really good reason to do so. \
The same applies to status responses. If you notice a problem, feel free to leave a comment in the code for future refactoring to API v2 (which is currently not planned).

### Adding/Maintaining API routes

All expected results (errors, success, fail messages) must be documented ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L319-L327)). \
All JSON input types must be defined as a struct in [modules/structs/](modules/structs/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L76-L91)) \
and referenced in [routers/api/v1/swagger/options.go](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/options.go). \
They can then be used like [this example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L318). \
All JSON responses must be defined as a struct in [modules/structs/](modules/structs/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/modules/structs/issue.go#L36-L68)) \
and referenced in its category in [routers/api/v1/swagger/](routers/api/v1/swagger/) ([example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/swagger/issue.go#L11-L16)) \
They can be used like [this example](https://github.com/go-gitea/gitea/blob/c620eb5b2d0d874da68ebd734d3864c5224f71f7/routers/api/v1/repo/issue.go#L277-L279).

### When to use what HTTP method

In general, HTTP methods are chosen as follows:

- **GET** endpoints return the requested object(s) and status **OK (200)**
- **DELETE** endpoints return the status **No Content (204)** and no content either
- **POST** endpoints are used to **create** new objects (e.g. a User) and return the status **Created (201)** and the created object
- **PUT** endpoints are used to **add/assign** existing Objects (e.g. a user to a team) and return the status **No Content (204)** and no content either
- **PATCH** endpoints are used to **edit/change** an existing object and return the changed object and the status **OK (200)**

### Requirements for API routes

All parameters of endpoints changing/editing an object must be optional (except the ones to identify the object, which are required).

Endpoints returning lists must

- support pagination (`page` & `limit` options in query)
- set `X-Total-Count` header via **SetTotalCountHeader** ([example](https://github.com/go-gitea/gitea/blob/7aae98cc5d4113f1e9918b7ee7dd09f67c189e3e/routers/api/v1/repo/issue.go#L444))

## Backports and Frontports

### What is backported?

We backport PRs given the following circumstances:

1. Feature freeze is active, but `<version>-rc0` has not been released yet. Here, we backport as much as possible. <!-- TODO: Is that our definition with the new backport bot? -->
2. `rc0` has been released. Here, we only backport bug- and security-fixes, and small enhancements. Large PRs such as refactors are not backported anymore. <!-- TODO: Is that our definition with the new backport bot? -->
3. We never backport new features.
4. We never backport breaking changes except when
    1. The breaking change has no effect on the vast majority of users
    2. The component triggering the breaking change is marked as experimental

### How to backport?

In the past, it was necessary to manually backport your PRs. \
Now, that's not a requirement anymore as our [backport bot](https://github.com/GiteaBot) tries to create backports automatically once the PR is merged when the PR

- does not have the label `backport/manual`
- has the label `backport/<version>`

The `backport/manual` label signifies either that you want to backport the change yourself, or that there were conflicts when backporting, thus you **must** do it yourself.

### Format of backport PRs

The title of backport PRs should be

```
<original PR title> (#<original pr number>)
```

The first two lines of the summary of the backporting PR should be

```
Backport #<original pr number>

```

with the rest of the summary and labels matching the original PR.

### Frontports

Frontports behave exactly as described above for backports.

## Developer Certificate of Origin (DCO)

We consider the act of contributing to the code by submitting a Pull Request as the "Sign off" or agreement to the certifications and terms of the [DCO](DCO) and [MIT license](LICENSE). \
No further action is required. \
You can also decide to sign off your commits by adding the following line at the end of your commit messages:

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

If you set the `user.name` and `user.email` Git config options, you can add the line to the end of your commits automatically with `git commit -s`.

We assume in good faith that the information you provide is legally binding.

## Release Cycle

We adopted a release schedule to streamline the process of working on, finishing, and issuing releases. \
The overall goal is to make a major release every three or four months, which breaks down into two or three months of general development followed by one month of testing and polishing known as the release freeze. \
All the feature pull requests should be
merged before feature freeze. And, during the frozen period, a corresponding
release branch is open for fixes backported from main branch. Release candidates
are made during this period for user testing to
obtain a final version that is maintained in this branch.

During a development cycle, we may also publish any necessary minor releases
for the previous version. For example, if the latest, published release is
v1.2, then minor changes for the previous release—e.g., v1.1.0 -> v1.1.1—are
still possible.

## Maintainers

To make sure every PR is checked, we have [maintainers](MAINTAINERS). \
Every PR **must** be reviewed by at least two maintainers (or owners) before it can get merged. \
For refactoring PRs after a week and documentation only PRs, the approval of only one maintainer is enough. \
A maintainer should be a contributor of Gitea and contributed at least
4 accepted PRs. A contributor should apply as a maintainer in the
[Discord](https://discord.gg/Gitea) `#develop` channel. The team maintainers may invite the contributor. A maintainer
should spend some time on code reviews. If a maintainer has no
time to do that, they should apply to leave the maintainers team
and we will give them the honor of being a member of the [advisors
team](https://github.com/orgs/go-gitea/teams/advisors). Of course, if
an advisor has time to code review, we will gladly welcome them back
to the maintainers team. If a maintainer is inactive for more than 3
months and forgets to leave the maintainers team, the owners may move
him or her from the maintainers team to the advisors team.
For security reasons, Maintainers should use 2FA for their accounts and
if possible provide GPG signed commits.
https://help.github.com/articles/securing-your-account-with-two-factor-authentication-2fa/
https://help.github.com/articles/signing-commits-with-gpg/

## Technical Oversight Committee (TOC)

At the start of 2023, the `Owners` team was dissolved. Instead, the governance charter proposed a technical oversight committee (TOC) which expands the ownership team of the Gitea project from three elected positions to six positions. Three positions would be elected as it has been over the past years, and the other three would consist of appointed members from the Gitea company.
https://blog.gitea.io/2023/02/gitea-quarterly-report-23q1/

When the new community members have been elected, the old members will give up ownership to the newly elected members. For security reasons, TOC members or any account with write access (like a bot) must use 2FA.
https://help.github.com/articles/securing-your-account-with-two-factor-authentication-2fa/

### Current TOC members

- 2023-01-01 ~ 2023-12-31 - https://blog.gitea.io/2023/02/gitea-quarterly-report-23q1/
  - Company
    - [Jason Song](https://gitea.com/wolfogre) <i@wolfogre.com>
    - [Lunny Xiao](https://gitea.com/lunny) <xiaolunwen@gmail.com>
    - [Matti Ranta](https://gitea.com/techknowlogick) <techknowlogick@gitea.io>
  - Community
    - [6543](https://gitea.com/6543) <6543@obermui.de>
    - [Andrew Thornton](https://gitea.com/zeripath) <art27@cantab.net>
    - [John Olheiser](https://gitea.com/jolheiser) <john.olheiser@gmail.com>

### Previous TOC/owners members

Here's the history of the owners and the time they served:

- [Lunny Xiao](https://gitea.com/lunny) - 2016, 2017, [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872)
- [Kim Carlbäcker](https://github.com/bkcsoft) - 2016, 2017
- [Thomas Boerger](https://gitea.com/tboerger) - 2016, 2017
- [Lauris Bukšis-Haberkorns](https://gitea.com/lafriks) - [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801)
- [Matti Ranta](https://gitea.com/techknowlogick) - [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872)
- [Andrew Thornton](https://gitea.com/zeripath) - [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872)

## Governance Compensation

Each member of the community elected TOC will be granted $500 each month as compensation for their work.

Furthermore, any community release manager for a specific release or LTS will be compensated $500 for the delivery of said release.

These funds will come from community sources like the OpenCollective rather than directly from the company.
Only non-company members are eligible for this compensation, and if a member of the community TOC takes the responsibility of release manager, they would only be compensated for their TOC duties.
Gitea Ltd employees are not eligible to receive any funds from the OpenCollective unless it is reimbursement for a purchase made for the Gitea project itself.

## TOC & Working groups

With Gitea covering many projects outside of the main repository, several groups will be created to help focus on specific areas instead of requiring maintainers to be a jack-of-all-trades. Maintainers are of course more than welcome to be part of multiple groups should they wish to contribute in multiple places.

The currently proposed groups are:

- **Core Group**: maintain the primary Gitea repository
- **Integration Group**: maintain the Gitea ecosystem's related tools, including go-sdk/tea/changelog/bots etc.
- **Documentation Group**: maintain related documents and repositories
- **Translation Group**: coordinate with translators and maintain translations
- **Security Group**: managed by TOC directly, members are decided by TOC, maintains security patches/responsible for security items

## Roadmap

Each year a roadmap will be discussed with the entire Gitea maintainers team, and feedback will be solicited from various stakeholders.
TOC members need to review the roadmap every year and work together on the direction of the project.

When a vote is required for a proposal or other change, the vote of community elected TOC members count slightly more than the vote of company elected TOC members. With this approach, we both avoid ties and ensure that changes align with the mission statement and community opinion.

You can visit our roadmap on the wiki.

## Versions

Gitea has the `main` branch as a tip branch and has version branches
such as `release/v1.19`. `release/v1.19` is a release branch and we will
tag `v1.19.0` for binary download. If `v1.19.0` has bugs, we will accept
pull requests on the `release/v1.19` branch and publish a `v1.19.1` tag,
after bringing the bug fix also to the main branch.

Since the `main` branch is a tip version, if you wish to use Gitea
in production, please download the latest release tag version. All the
branches will be protected via GitHub, all the PRs to every branch must
be reviewed by two maintainers and must pass the automatic tests.

## Releasing Gitea

- Let $vmaj, $vmin and $vpat be Major, Minor and Patch version numbers, $vpat should be rc1, rc2, 0, 1, ...... $vmaj.$vmin will be kept the same as milestones on github or gitea in future.
- Before releasing, confirm all the version's milestone issues or PRs has been resolved. Then discuss the release on Discord channel #maintainers and get agreed with almost all the owners and mergers. Or you can declare the version and if nobody against in about serval hours.
- If this is a big version first you have to create PR for changelog on branch `main` with PRs with label `changelog` and after it has been merged do following steps:
  - Create `-dev` tag as `git tag -s -F release.notes v$vmaj.$vmin.0-dev` and push the tag as `git push origin v$vmaj.$vmin.0-dev`.
  - When CI has finished building tag then you have to create a new branch named `release/v$vmaj.$vmin`
- If it is bugfix version create PR for changelog on branch `release/v$vmaj.$vmin` and wait till it is reviewed and merged.
- Add a tag as `git tag -s -F release.notes v$vmaj.$vmin.$`, release.notes file could be a temporary file to only include the changelog this version which you added to `CHANGELOG.md`.
- And then push the tag as `git push origin v$vmaj.$vmin.$`. Drone CI will automatically create a release and upload all the compiled binary. (But currently it doesn't add the release notes automatically. Maybe we should fix that.)
- If needed send a frontport PR for the changelog to branch `main` and update the version in `docs/config.yaml` to refer to the new version.
- Send PR to [blog repository](https://gitea.com/gitea/blog) announcing the release.
- Verify all release assets were correctly published through CI on dl.gitea.com and GitHub releases. Once ACKed:
  - bump the version of https://dl.gitea.com/gitea/version.json
  - merge the blog post PR
  - announce the release in discord `#announcements`
