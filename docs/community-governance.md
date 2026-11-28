# Community governance and review process

This document describes the project roles and what is expected of each, project governance, and the detailed pull request review workflow (labels, merge queue, commit message format). For what contributors should do when opening and updating a PR, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Table of contents

- [Community governance and review process](#community-governance-and-review-process)
  - [Table of contents](#table-of-contents)
  - [Code review](#code-review)
    - [Milestone](#milestone)
    - [Labels](#labels)
    - [Reviewing PRs](#reviewing-prs)
      - [For reviewers](#for-reviewers)
    - [Required approvals](#required-approvals)
    - [Getting PRs merged](#getting-prs-merged)
    - [Final call](#final-call)
    - [Commit messages](#commit-messages)
      - [PR Co-authors](#pr-co-authors)
      - [PRs targeting `main`](#prs-targeting-main)
      - [Backport PRs](#backport-prs)
  - [Roles](#roles)
    - [Overview](#overview)
    - [Collaborators](#collaborators)
      - [Becoming a collaborator](#becoming-a-collaborator)
    - [Maintainers](#maintainers)
      - [Becoming a maintainer](#becoming-a-maintainer)
      - [Losing merge rights](#losing-merge-rights)
    - [Core Team](#core-team)
      - [Core Team election process](#core-team-election-process)
    - [Advisors](#advisors)
      - [Inactivity](#inactivity)
      - [Returning from advisors](#returning-from-advisors)
    - [Account security](#account-security)
    - [Current Core Team members](#current-core-team-members)
    - [Previous Core Team/TOC/owners members](#previous-core-teamtocowners-members)
  - [Funding](#funding)
  - [Roadmap](#roadmap)

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

Collaborators and maintainers are encouraged to review pull requests in areas where they have expertise or particular interest.

#### For reviewers

- **Verification**: Verify that the PR accurately reflects the changes, and verify that the tests and documentation are complete and aligned with the implementation.
- **Actionable feedback**: Say what should change and why, and distinguish required changes from optional suggestions.
- **Feedback**: Focus feedback on the issue itself and avoid comments about the contributor's abilities.
- **Request changes**: If you request changes (i.e., block a PR), give a clear rationale and, whenever possible, a concrete path to resolution.
- **Approval**: Only approve a PR when you are fully satisfied with its current state - "rubber-stamp" approvals need to be highlighted as such.

### Required approvals

Every change to Gitea must be reviewed before it is merged, including changes from maintainers and the Core Team. Approvals from collaborators, maintainers and Core Team members count; the author's own does not.

| PR                                                                    | Required approvals         |
|-----------------------------------------------------------------------|----------------------------|
| Any PR                                                                | 2                          |
| Refactoring or documentation-only PR, open for more than one week     | 1                          |
| PR that went through a [final call](#final-call) without objections   | 1, from a Core Team member |
| Fix for a critical bug that prevents Gitea from compiling or starting | 0                          |

### Getting PRs merged

When a PR has enough approvals, it gets the `lgtm/done` label. After that, the author mainly fixes merge conflicts and responds to or implements review requests; collaborators and maintainers drive getting the PR merged.

If a PR has `lgtm/done`, no open discussions, and no merge conflicts, a maintainer rewrites its title and description into the final [commit message](#commit-messages) and clicks **Merge when ready**. That adds the PR to the [merge queue](https://github.com/go-gitea/gitea/queue/main), which tests it on top of the latest `main` and the PRs ahead of it, and merges it once CI passes. Edits to the title and description no longer reach the commit once the queue starts building the PR, so make them before queueing.

Gitea uses its own tool, <https://github.com/go-gitea/gitea/blob/main/.github/workflows/giteabot.yml>, to automate parts of the review process. It:

- Counts approvals and sets the `lgtm/*` labels and the `giteabot/lgtm` status.
- Takes a PR out of the merge queue when it loses its approval.
- Creates a backport PR when needed after the initial PR merges.

### Final call

If a PR has been ignored for more than 7 days with no comments or reviews, and the author or any collaborator believes it will not survive a long wait (such as a refactoring PR), they can send a "final call" to the Core Team by mentioning them in a comment.

After another 7 days, if there is still zero approval, this is considered a polite refusal, and the PR will be closed to avoid wasting further time. Therefore, the "final call" has a cost, and should be used cautiously.

However, if there are no objections from collaborators or maintainers, the PR can be merged with only one approval from the Core Team (not the author).

### Commit messages

Maintainers are required to rewrite the PR title and the first comment (the summary) when necessary so the squash commit message is clear.
Usually the Pull Request description and commit message body should not be empty, unless the title is already clear enough or the description would be a copy of the comments in code.

The final commit message:

- should match the code changes.
- should only keep true co-authors, false-positive co-authors should be removed.
- should not hedge: replace phrases like `hopefully, <x> won't happen anymore` with definite wording.
- should not contain hidden information like `<!-- -->` or extra information after the description's divider `----`.
- should not contain unrelated contents (e.g.: Release Notes, Configuration, etc.) from a Renovate update PR.

#### PR Co-authors

A person counts as a PR co-author once they (co-)authored a commit that is not simply a `Merge base branch into branch` commit. Maintainers must remove such false-positive co-authors when writing the squash message. Every true co-author must remain in the commit message.

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

## Roles

### Overview

Maintainers can do everything collaborators can, and Core Team members everything maintainers can.

| Role                           | Who                                  | Expected to                                                                                                    | Can                                                                                                                                 | GitHub team                                                                                           |
|--------------------------------|--------------------------------------|----------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| [Collaborator](#collaborators) | Trusted regular contributors         | Review PRs, triage issues, stay [active](#inactivity)                                                          | Approve PRs (counts toward [required approvals](#required-approvals)), triage and label issues and PRs, vote in Core Team elections | [maintainers](https://github.com/orgs/go-gitea/teams/maintainers)                                     |
| [Maintainer](#maintainers)     | Experienced collaborators            | Queue approved PRs, write clear commit messages, enforce project conventions, help with backports and releases | Merge PRs                                                                                                                           | [mergers](https://github.com/orgs/go-gitea/teams/mergers)                                             |
| [Core Team](#core-team)        | Elected and appointed leadership     | Set technical direction and the roadmap, decide on roles, resolve disputes                                     | Make final decisions, revert merges, grant and remove roles                                                                         | [technical-oversight-committee](https://github.com/orgs/go-gitea/teams/technical-oversight-committee) |
| [Advisor](#advisors)           | Former collaborators and maintainers | Nothing                                                                                                        | Ask to [return](#returning-from-advisors)                                                                                           | [advisors](https://github.com/orgs/go-gitea/teams/advisors)                                           |

The GitHub teams are the source of truth for who currently holds a role. The [MAINTAINERS](../MAINTAINERS) file is a historic record of everyone who has been a collaborator or maintainer.

### Collaborators

Collaborators (formerly called maintainers) make sure every PR gets proper review. They are expected to:

- Spend time reviewing PRs, especially in areas where they have expertise.
- Triage issues and PRs: apply labels and milestones, close duplicates and low-quality submissions (see the [AI Contribution Policy](../CONTRIBUTING.md#ai-contribution-policy)).
- Stay [active](#inactivity) in the project.

Collaborators cannot merge PRs.

#### Becoming a collaborator

To become a collaborator, you must:

- have been active in the community for at least three months, and
- have at least four merged non-trivial PRs. Typo fixes, documentation-only changes and dependency bumps do not count.

To apply, use the [Discord](https://discord.gg/Gitea) `#develop` channel. The Core Team may also invite contributors. Either way, the Core Team decides by vote.

### Maintainers

Maintainers (formerly called mergers) are collaborators who carry out the final merge of approved PRs. In addition to the collaborator duties, they are expected to:

- Add PRs to the [merge queue](#getting-prs-merged) once a PR has `lgtm/done`, no open discussions, and no merge conflicts.
- Rewrite the PR title and description before the merge so the [commit message](#commit-messages) is clear.\
  Maintainers should **not** edit the actual commit message except to remove unnecessary information, so that people looking at the PR can understand what changed.
- Assign the correct labels (including `type/…`) needed for changelog and backport decisions.
- Agree, together with the Core Team, on when a release is ready (see [release management](release-management.md)).

Merging a PR also means the PR looks good to the maintainer and is approved by them.

#### Becoming a maintainer

To become a maintainer, you must:

- be a collaborator, and
- have participated actively in the community for at least four months before applying.

To apply, use the [Discord](https://discord.gg/Gitea) `#maintainers` channel. The Core Team may also invite collaborators. Either way, regardless of previous participation, the Core Team decides by vote.

#### Losing merge rights

If a maintainer violates the merge rules above more than 3 times in the past 365 days (e.g.: merging with unresolved reviews without a Core Team decision to ignore the review, merging with garbage commit messages), the Core Team may revoke their merge rights for at least three months. They remain a collaborator.

### Core Team

The Core Team (formerly the Technical Oversight Committee, TOC) leads the project. It:

- Has the final say on technical direction and architecture, and maintains the [roadmap](#roadmap).
- Resolves disputes and breaks ties.
- Decides on applications for collaborator and maintainer, and moves inactive members to [advisors](#advisors).
- Handles security decisions.
- Decides the roadmap for the project.
- May revert any merged PR that should not have been merged.

At the start of 2023, the `Owners` team was dissolved and replaced by the TOC, which expanded the project leadership from three elected positions to six positions. Three positions are elected by the community, and the other three are appointed members from the Gitea company.
<https://blog.gitea.com/quarterly-23q1/>

#### Core team decisions

The core team decides decisions by absolute majority.
This means, at least fifty percent of the casted votes must be in favor of a result.
Abstained votes are included in the calculation.

Every decision within the core team must give a minimum deadline of 72 hours (three days) for participating.
If a core team member does not participate in three subsequent decisions, they will be asked to step down from the core team.
The core team will then continue with one member less until the next election.

#### Core Team election process

Any active collaborator or maintainer who is not associated with the Gitea company is eligible for a community Core Team seat.
Only active collaborators and maintainers can nominate candidates and vote; advisors cannot.
A collaborator or maintainer can either nominate themselves, or can be nominated by others to be a candidate.
If you are nominated by someone else, you must first accept your nomination before the vote starts to be a candidate.

The Core Team is elected for one year, the election happens yearly.
After the announcement of the results of the election, elected members have two weeks time to confirm or refuse the seat.
If an elected member does not answer within this timeframe, they are automatically assumed to refuse the seat.
Refusals result in the person with the next highest vote getting the same choice.
As long as seats are empty, members of the previous Core Team can fill them until an elected member accepts the seat.

If an elected member that accepts the seat does not have 2FA configured yet, they will be temporarily counted as `answer pending` until they manage to configure 2FA, thus leaving their seat empty for this duration.

If multiple persons have the same amount of votes, a random draw will be used to determine the order of the candidates with the same amount of votes, and thus who gets the seat first.
The candidates will be placed in the list in an alphabetical insensitive order by their username.
We use this script to determine the order of candidates with the same amount of votes:

```python
import random
random.seed("Gitea TOC <YEAR> Election")
random.choice([<CANDIDATE_1>, <CANDIDATE_2>, ...])
```

The result of this script needs then to be published in the election issue to ensure transparency of the process.

### Advisors

Advisors are former collaborators and maintainers. They keep no rights, but their past work is recognized and they are welcome back at any time.

If you cannot keep contributing, ask the Core Team to move you to the advisors team.

#### Inactivity

A maintainer is inactive after three consecutive months, and a collaborator after twelve consecutive months, without any project-related participation in any of the official spaces (GitHub, gitea.com, Discord, forum, …).

The Core Team notifies inactive members that they will be moved to advisors and waits at least one week before doing so.

### Returning from advisors

Advisors who want to contribute again can ask the Core Team to be re-added as collaborators or maintainers, depending on their previous role. A Core Team seat can only be regained through an [election](#core-team-election-process).

### Account security

Maintainers and Core Team members **must** enable [two-factor authentication](https://docs.github.com/en/authentication/securing-your-account-with-two-factor-authentication-2fa/configuring-two-factor-authentication) and [sign their commits](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits).
Collaborators are encouraged, but not required, to do the same.
Bot accounts with write access **must** use two-factor authentication.

### Current Core Team members

- 2026-06-14 ~ 2026-12-31
  - Company
    - [Yu Tian](https://gitea.com/Zettat123) <zettat123@gmail.com>
    - [Lunny Xiao](https://gitea.com/lunny) <xiaolunwen@gmail.com>
    - [Matti Ranta](https://gitea.com/techknowlogick) <techknowlogick@gitea.com>
  - Community
    - [bircni](https://gitea.com/bircni) <bircni@icloud.com>
    - [delvh](https://gitea.com/delvh) <dev.lh@web.de>
    - [TheFox0x7](https://gitea.com/TheFox0x7) <thefox0x7@gmail.com>

### Previous Core Team/TOC/owners members

Here's the history of the owners and the time they served:

- [Lunny Xiao](https://gitea.com/lunny) - 2016, 2017, [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [Kim Carlbäcker](https://github.com/bkcsoft) - 2016, 2017
- [Thomas Boerger](https://gitea.com/tboerger) - 2016, 2017
- [Lauris Bukšis](https://gitea.com/lafriks) - [2018](https://github.com/go-gitea/gitea/issues/3255), [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), 2025
- [Matti Ranta](https://gitea.com/techknowlogick) - [2019](https://github.com/go-gitea/gitea/issues/5572), [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [Andrew Thornton](https://gitea.com/zeripath) - [2020](https://github.com/go-gitea/gitea/issues/9230), [2021](https://github.com/go-gitea/gitea/issues/13801), [2022](https://github.com/go-gitea/gitea/issues/17872), 2023
- [6543](https://gitea.com/6543) - 2023, 2025
- [John Olheiser](https://gitea.com/jolheiser) - 2023, 2024
- [Jason Song](https://gitea.com/wolfogre) - 2023, 2025

## Funding

Community funds, such as donations through the [OpenCollective](https://opencollective.com/gitea), are spent on what the project needs for development, for example:

- CI runners, hosting, domains and other infrastructure.
- Tools and services used to develop, test and release Gitea.
- Reimbursing purchases someone made for the project.

Holding a role in the project is not paid from these funds.

## Roadmap

Each year a roadmap will be discussed with all collaborators and maintainers, and feedback will be solicited from various stakeholders.
Core Team members need to review the roadmap every year and work together on the direction of the project.

When a vote is required for a proposal or other change, the vote of community elected Core Team members count slightly more than the vote of company appointed Core Team members. With this approach, we both avoid ties and ensure that changes align with the mission statement and community opinion.

You can visit our roadmap on the wiki.
