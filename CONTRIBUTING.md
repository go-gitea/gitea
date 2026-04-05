# Contribution Guidelines

This document explains how to contribute changes to the Gitea project. Topic-specific guides live in separate files so the essentials are easier to find.

| Topic | Document |
| :---- | :------- |
| Backend (Go modules, API v1) | [docs/guideline-backend.md](docs/guideline-backend.md) |
| Frontend (npm, UI guidelines) | [docs/guideline-frontend.md](docs/guideline-frontend.md) |
| Maintainers, TOC, labels, merge queue, commit format for mergers | [docs/community-governance.md](docs/community-governance.md) |
| Release cycle, backports, tagging releases | [docs/release-management.md](docs/release-management.md) |

<details><summary>Table of Contents</summary>

- [Contribution Guidelines](#contribution-guidelines)
  - [Introduction](#introduction)
  - [AI Contribution Policy](#ai-contribution-policy)
  - [Issues](#issues)
    - [How to report issues](#how-to-report-issues)
    - [Types of issues](#types-of-issues)
    - [Discuss your design before the implementation](#discuss-your-design-before-the-implementation)
    - [Issue locking](#issue-locking)
  - [Building Gitea](#building-gitea)
  - [Styleguide](#styleguide)
  - [Copyright](#copyright)
  - [Testing](#testing)
  - [Translation](#translation)
  - [Code review](#code-review)
    - [Pull request format](#pull-request-format)
    - [PR title and summary](#pr-title-and-summary)
    - [Breaking PRs](#breaking-prs)
      - [What is a breaking PR?](#what-is-a-breaking-pr)
      - [How to handle breaking PRs?](#how-to-handle-breaking-prs)
    - [Maintaining open PRs](#maintaining-open-prs)
    - [Reviewing PRs](#reviewing-prs)
      - [For PR authors](#for-pr-authors)
  - [Documentation](#documentation)
  - [Developer Certificate of Origin (DCO)](#developer-certificate-of-origin-dco)

</details>

## Introduction

It assumes you have followed the [installation instructions](https://docs.gitea.com/category/installation). \
Sensitive security-related issues should be reported to [security@gitea.io](mailto:security@gitea.io).

For configuring IDEs for Gitea development, see the [contributed IDE configurations](contrib/ide/).

## AI Contribution Policy

Contributions made with the assistance of AI tools are welcome, but contributors must use them responsibly and disclose that use clearly.

1. Review AI-generated code closely before marking a pull request ready for review.
2. Manually test the changes and add appropriate automated tests where feasible.
3. Only use AI to assist in contributions that you understand well enough to explain, defend, and revise yourself during review.
4. Disclose AI-assisted content clearly.
5. Do not use AI to reply to questions about your issue or pull request. The questions are for you, not an AI model.
6. AI may be used to help draft issues and pull requests, but contributors remain responsible for the accuracy, completeness, and intent of what they submit.

Maintainers reserve the right to close pull requests and issues that do not disclose AI assistance, that appear to be low-quality AI-generated content, or where the contributor cannot explain or defend the proposed changes themselves.

We welcome new contributors, but cannot sustain the effort of supporting contributors who primarily defer to AI rather than engaging substantively with the review process.

## Issues

### How to report issues

Please search the issues on the issue tracker with a variety of related keywords to ensure that your issue has not already been reported.

If your issue has not been reported yet, [open an issue](https://github.com/go-gitea/gitea/issues/new)
and answer the questions so we can understand and reproduce the problematic behavior. \
Please write clear and concise instructions so that we can reproduce the behavior — even if it seems obvious. \
The more detailed and specific you are, the faster we can fix the issue. \
It is really helpful if you can reproduce your problem on a site running on the latest commits, i.e. <https://demo.gitea.com>, as perhaps your problem has already been fixed on a current version. \
Please follow the guidelines described in [How to Report Bugs Effectively](http://www.chiark.greenend.org.uk/~sgtatham/bugs.html) for your report.

Please be kind—remember that Gitea comes at no cost to you, and you're getting free help.

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

### Issue locking

Commenting on closed or merged issues/PRs is strongly discouraged.
Such comments will likely be overlooked as some maintainers may not view notifications on closed issues, thinking that the item is resolved.
As such, commenting on closed/merged issues/PRs may be disabled prior to the scheduled auto-locking if a discussion starts or if unrelated comments are posted.
If further discussion is needed, we encourage you to open a new issue instead and we recommend linking to the issue/PR in question for context.

## Building Gitea

See the [development setup instructions](https://docs.gitea.com/development/hacking-on-gitea).

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

|                       |                                                                              |
| :-------------------- | :--------------------------------------------------------------------------- |
|``make lint``          | lint everything (not needed if you only change the front- **or** backend)    |
|``make lint-frontend`` | lint frontend files                                                          |
|``make lint-backend``  | lint backend files                                                           |

- run tests (we suggest running them on Linux)

|  Command                                    | Action                                                   |                                             |
| :------------------------------------------ | :------------------------------------------------------- | ------------------------------------------- |
|``make test[\#SpecificTestName]``            |  run unit test(s)                                        |                                             |
|``make test-sqlite[\#SpecificTestName]``     |  run [integration](tests/integration) test(s) for SQLite | [More details](tests/integration/README.md) |
|``make test-e2e``                            |  run [end-to-end](tests/e2e) test(s) using Playwright    |                                             |

- E2E test environment variables

| Variable                          | Description                                                 |
| :-------------------------------- | :---------------------------------------------------------- |
| ``GITEA_TEST_E2E_DEBUG``          | When set, show Gitea server output                          |
| ``GITEA_TEST_E2E_FLAGS``          | Additional flags passed to Playwright, for example ``--ui`` |
| ``GITEA_TEST_E2E_TIMEOUT_FACTOR`` | Timeout multiplier (default: 3 on CI, 1 locally)            |

## Translation

All translation work happens on [Crowdin](https://translate.gitea.com).
The only translation that is maintained in this repository is [the English translation](https://github.com/go-gitea/gitea/blob/main/options/locale/locale_en-US.json).
It is synced regularly with Crowdin. \
Other locales on main branch **should not** be updated manually as they will be overwritten with each sync. \
Once a language has reached a **satisfactory percentage** of translated keys (~25%), it will be synced back into this repo and included in the next released version.

The tool `go run build/backport-locale.go` can be used to backport locales from the main branch to release branches that were missed.

## Code review

How labels, milestones, and the merge queue work is documented in [docs/community-governance.md](docs/community-governance.md).

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
In the PR summary, you can describe exactly how you are fixing this problem.

Keep this summary up-to-date as the PR evolves. \
If your PR changes the UI, you must add **after** screenshots in the PR summary. \
If you are not implementing a new feature, you should also post **before** screenshots for comparison.

If you are implementing a new feature, your PR will only be merged if your screenshots are up to date.\
Furthermore, feature PRs will only be merged if their summary contains a clear usage description (understandable for users) and testing description (understandable for reviewers).
You should strive to combine both into a single description.

Another requirement for merging PRs is that the PR is labeled correctly.\
However, this is not your job as a contributor, but the job of the person merging your PR.\
If you think that your PR was labeled incorrectly, or notice that it was merged without labels, please let us know.

If your PR closes some issues, you must note that in a way that both GitHub and Gitea understand, i.e. by appending a paragraph like

```text
Fixes/Closes/Resolves #<ISSUE_NR_X>.
Fixes/Closes/Resolves #<ISSUE_NR_Y>.
```

to your summary. \
Each issue that will be closed must stand on a separate line.

### Breaking PRs

#### What is a breaking PR?

A PR is breaking if it meets one of the following criteria:

- It changes API output in an incompatible way for existing users
- It removes a setting that an admin could previously set (i.e. via `app.ini`)
- An admin must do something manually to restore the old behavior

In particular, this means that adding new settings is not breaking.\
Changing the default value of a setting or replacing the setting with another one is breaking, however.

#### How to handle breaking PRs?

If your PR has a breaking change, you must add two things to the summary of your PR:

1. A reasoning why this breaking change is necessary
2. A `BREAKING` section explaining in simple terms (understandable for a typical user) how this PR affects users and how to mitigate these changes. This section can look for example like

```md
## :warning: BREAKING :warning:
```

Breaking PRs will not be merged as long as not both of these requirements are met.

### Maintaining open PRs

Code review starts when you open a non-draft PR or move a draft out of draft state. After that, do not rebase or squash your branch; it makes new changes harder to review.

Merge the base branch into yours only when you need to, for example because of conflicting changes elsewhere. That limits unnecessary CI runs.

Every PR is squash-merged, so merge commits on your branch do not matter for final history. The squash produces a single commit; mergers follow the [commit message format](docs/community-governance.md#commit-messages) in the governance guide.

### Reviewing PRs

Maintainers are encouraged to review pull requests in areas where they have expertise or particular interest.

#### For PR authors

- **Response**: When answering reviewer questions, use real-world cases or examples and avoid speculation.
- **Discussion**: A discussion is always welcome and should be used to clarify the changes and the intent of the PR.
- **Help**: If you need help with the PR or comments are unclear, ask for clarification.

Guidance for reviewers, the merge queue, and the squash commit message format is in [docs/community-governance.md](docs/community-governance.md).

## Documentation

If you add a new feature or change an existing aspect of Gitea, the documentation for that feature must be created or updated in another PR at [https://gitea.com/gitea/docs](https://gitea.com/gitea/docs).
**The docs directory on main repository will be removed at some time. We will have a yaml file to store configuration file's meta data. After that completed, configuration documentation should be in the main repository.**

## Developer Certificate of Origin (DCO)

We consider the act of contributing to the code by submitting a Pull Request as the "Sign off" or agreement to the certifications and terms of the [DCO](DCO) and [MIT license](LICENSE). \
No further action is required. \
You can also decide to sign off your commits by adding the following line at the end of your commit messages:

```
Signed-off-by: Joe Smith <joe.smith@email.com>
```

If you set the `user.name` and `user.email` Git config options, you can add the line to the end of your commits automatically with `git commit -s`.

We assume in good faith that the information you provide is legally binding.
