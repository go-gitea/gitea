# Community governance and review process

This document describes maintainer expectations, project governance, and the detailed pull request review workflow (labels, merge queue, commit message format for mergers). For what contributors should do when opening and updating a PR, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Code review

### Milestone

A PR should only be assigned to a milestone if it will likely be merged into the given version. \
PRs without a milestone may not be merged.

### Labels

Almost all labels used inside Gitea can be classified as one of the following:

- `modifies/…`: Determines which parts of the codebase are affected. These labels will be set through the CI.
- `topic/…`:  Determines the conceptual component of Gitea that is affected, i.e. issues, projects, or authentication. At best, PRs should only target one component but there might be overlap. Must be set manually.
- `type/…`: Determines the type of an issue or PR (feature, refactoring, docs, bug, …). If GitHub supported scoped labels, these labels would be exclusive, so you should set **exactly** one, not more or less (every PR should fall into one of the provided categories, and only one).
- `issue/…` / `lgtm/…`: Labels that are specific to issues or PRs respectively and that are only necessary in a given context, i.e. `issue/not-a-bug` or `lgtm/need 2`

Every PR should be labeled correctly with every label that applies.

There are also some labels that will be managed automatically.\
In particular, these are

- the amount of pending required approvals
- has all `backport`s or needs a manual backport

### Reviewing PRs

Maintainers are encouraged to review pull requests in areas where they have expertise or particular interest.

#### For reviewers

- **Verification**: Verify that the PR accurately reflects the changes, and verify that the tests and documentation are complete and aligned with the implementation.
- **Actionable feedback**: Say what should change and why, and distinguish required changes from optional suggestions.
- **Feedback**: Focus feedback on the issue itself and avoid comments about the contributor's abilities.
- **Request changes**: If you request changes (i.e., block a PR), give a clear rationale and, whenever possible, a concrete path to resolution.
- **Approval**: Only approve a PR when you are fully satisfied with its current state - "rubber-stamp" approvals need to be highlighted as such.

### Getting PRs merged

Changes to Gitea must be reviewed before they are accepted, including changes from owners and maintainers. The exception is critical bugs that prevent Gitea from compiling or starting.

We require two maintainer approvals for every PR. When that is satisfied, your PR gets the `lgtm/done` label. After that, you mainly fix merge conflicts and respond to or implement maintainer requests; maintainers drive getting the PR merged.

If a PR has `lgtm/done`, no open discussions, and no merge conflicts, any maintainer may add `reviewed/wait-merge`. That puts the PR in the merge queue. PRs are merged from the queue in the order of this list:

<https://github.com/go-gitea/gitea/pulls?q=is%3Apr+label%3Areviewed%2Fwait-merge+sort%3Acreated-asc+is%3Aopen>

Gitea uses its own tool, <https://github.com/GiteaBot/gitea-backporter>, to automate parts of the review process. The backporter:

- Creates a backport PR when needed after the initial PR merges.
- Removes the PR from the merge queue after it merges.
- Keeps the oldest branch in the merge queue up to date with merges.

### Final call

If a PR has been ignored for more than 7 days with no comments or reviews, and the author or any maintainer believes it will not survive a long wait (such as a refactoring PR), they can send "final call" to the TOC by mentioning them in a comment.

After another 7 days, if there is still zero approval, this is considered a polite refusal, and the PR will be closed to avoid wasting further time. Therefore, the "final call" has a cost, and should be used cautiously.

However, if there are no objections from maintainers, the PR can be merged with only one approval from the TOC (not the author).

### Commit messages

Mergers are required to rewrite the PR title and the first comment (the summary) when necessary so the squash commit message is clear.

The final commit message should not hedge: replace phrases like `hopefully, <x> won't happen anymore` with definite wording.

#### PR Co-authors

A person counts as a PR co-author once they (co-)authored a commit that is not simply a `Merge base branch into branch` commit. Mergers must remove such false-positive co-authors when writing the squash message. Every true co-author must remain in the commit message.

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

## Maintainers

We list [maintainers](../MAINTAINERS) so every PR gets proper review.

#### Review expectations

Every PR **must** be reviewed by at least two maintainers (or owners) before merge. **Exception:** after one week, refactoring PRs and documentation-only PRs need only one maintainer approval.

Maintainers are expected to spend time on code reviews.

#### Becoming a maintainer

A maintainer should already be a Gitea contributor with at least four merged PRs. To apply, use the [Discord](https://discord.gg/Gitea) `#develop` channel. Maintainer teams may also invite contributors.

#### Stepping down, advisors, and inactivity

If you cannot keep reviewing, apply to leave the maintainers team. You can join the [advisors team](https://github.com/orgs/go-gitea/teams/advisors); advisors who want to review again are welcome back as maintainers.

If a maintainer is inactive for more than three months and has not left the team, owners may move them to the advisors team.

#### Account security

For security, maintainers should enable 2FA and sign commits with GPG when possible:

- [Two-factor authentication](https://docs.github.com/en/authentication/securing-your-account-with-two-factor-authentication-2fa/configuring-two-factor-authentication)
- [Signing commits with GPG](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits)

Any account with write access (including bots and TOC members) **must** use [2FA](https://docs.github.com/en/authentication/securing-your-account-with-two-factor-authentication-2fa/configuring-two-factor-authentication).

## Technical Oversight Committee (TOC)

At the start of 2023, the `Owners` team was dissolved. Instead, the governance charter proposed a technical oversight committee (TOC) which expands the ownership team of the Gitea project from three elected positions to six positions. Three positions are elected as it has been over the past years, and the other three consist of appointed members from the Gitea company.
https://blog.gitea.com/quarterly-23q1/

### TOC election process

Any maintainer is eligible to be part of the community TOC if they are not associated with the Gitea company.
A maintainer can either nominate themselves, or can be nominated by other maintainers to be a candidate for the TOC election.
If you are nominated by someone else, you must first accept your nomination before the vote starts to be a candidate.

The TOC is elected for one year, the TOC election happens yearly.
After the announcement of the results of the TOC election, elected members have two weeks time to confirm or refuse the seat.
If an elected member does not answer within this timeframe, they are automatically assumed to refuse the seat.
Refusals result in the person with the next highest vote getting the same choice.
As long as seats are empty in the TOC, members of the previous TOC can fill them until an elected member accepts the seat.

If an elected member that accepts the seat does not have 2FA configured yet, they will be temporarily counted as `answer pending` until they manage to configure 2FA, thus leaving their seat empty for this duration.

### Current TOC members

- 2024-01-01 ~ 2024-12-31
  - Company
    - [Jason Song](https://gitea.com/wolfogre) <i@wolfogre.com>
    - [Lunny Xiao](https://gitea.com/lunny) <xiaolunwen@gmail.com>
    - [Matti Ranta](https://gitea.com/techknowlogick) <techknowlogick@gitea.com>
  - Community
    - [6543](https://gitea.com/6543) <6543@obermui.de>
    - [delvh](https://gitea.com/delvh) <dev.lh@web.de>
    - [John Olheiser](https://gitea.com/jolheiser) <john.olheiser@gmail.com>

### Previous TOC/owners members

Here's the history of the owners and the time they served:

- [Lunny Xiao](https://gitea.com/lunny) - 2016, 2017, [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [Kim Carlbäcker](https://github.com/bkcsoft) - 2016, 2017
- [Thomas Boerger](https://gitea.com/tboerger) - 2016, 2017
- [Lauris Bukšis-Haberkorns](https://gitea.com/lafriks) - [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801)
- [Matti Ranta](https://gitea.com/techknowlogick) - [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [Andrew Thornton](https://gitea.com/zeripath) - [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [6543](https://gitea.com/6543) - 2023
- [John Olheiser](https://gitea.com/jolheiser) - 2023
- [Jason Song](https://gitea.com/wolfogre) - 2023

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
