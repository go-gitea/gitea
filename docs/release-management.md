# Release management

This document describes the release cycle, backports, versioning, and the release manager checklist. For everyday contribution workflow, see [CONTRIBUTING.md](../CONTRIBUTING.md).

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

## Release Cycle

We use a release schedule so work, stabilization, and releases stay predictable.

### Cadence

- Aim for a major release about every three or four months.
- Roughly two or three months of general development, then about one month of testing and polish called the **release freeze**.
- *Starting with v1.26 the release cycle will be more predictable and follow a more regular schedule.*

### Release schedule

We will try to publish a new major version every three months:

- v1.26.0 in April 2026
- v1.27.0 in June 2026
- v1.28.0 in September 2026
- v1.29.0 in December 2026

#### How is the release handled?

- The release manager will tag the release candidate (e.g. `v1.26.0-rc0`) and publish it for testing in the **first week of the release month**.
- If there are no major issues, the release manager will check with the other maintainers and then tag the final release (e.g. `v1.26.0`) in the **one or two weeks following the release candidate**.

### Feature freeze

- Merge feature PRs before the freeze when you can.
- Feature PRs still open at the freeze move to the next milestone. Watch Discord for the freeze announcement.
- During the freeze, a **release branch** takes fixes backported from `main`. Release candidates ship for testing; the final release for that line is maintained from that branch.

### Patch releases

During a cycle we may ship patch releases for an older line. For example, if the latest release is v1.2, we can still publish v1.1.1 after v1.1.0.

### End of life (EOL)

We support per standard the last major release. For example, if the latest release is v1.26, we support v1.26 and v1.25, but not v1.24 anymore. We will only publish security fixes for the last major release, so if you are using an older release, please upgrade to a supported release as soon as possible.
Also we always try to support the latest on main branch, so if you are using the latest on main, you should be fine.

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

Confirm the release milestone is ready and agree on the release with the maintainers before publishing.
Release preparation and tagging are separate steps. The version is selected by the release manager.

### Prepare the changelog locally

Install [git-cliff](https://git-cliff.org/docs/installation/) (2.13.1 or newer), Node.js, and pnpm.
Fetch the release branch and tags, then create a changelog branch from the release branch:

```sh
git fetch origin --tags
git switch -c docs/changelog-28.0.1 origin/release/v28
make release-changelog RELEASE_VERSION=28.0.1
```

The command finds the nearest stable or release-candidate tag reachable from the current branch,
ignoring development tags and tags on unrelated branches. It generates only that tag-to-HEAD range
and inserts a new release section above the existing releases in `CHANGELOG.md`.
The header and historical changelog remain unchanged. It does not commit, tag, or push anything.
Set `RELEASE_PREVIOUS=v28.0.0` to override the starting tag, or `RELEASE_DATE=2026-10-02` to select the date.

PR labels are fetched from GitHub to retain the groups in `.changelog.yml`, including SECURITY and BREAKING.
Set `GITHUB_TOKEN` in your environment for authenticated API requests, especially for large release ranges.
The command excludes `chore`, `ci`, translation synchronization commits, and `skip-changelog` entries.
On release branches, backport scheduling labels do not exclude shipped fixes: the Git range already limits entries to included commits.
API failures stop generation before writing `CHANGELOG.md`.

Review and edit the new section, commit it, and push the changelog branch.
Then preview the PR targeting the release branch:

```sh
make release-changelog-pr RELEASE_VERSION=28.0.1 RELEASE_BRANCH=release/v28
```

The preview prints the exact title, body, and target without posting.
After reviewing them, create the PR with:

```sh
make release-changelog-pr RELEASE_VERSION=28.0.1 RELEASE_BRANCH=release/v28 RELEASE_PR_DRY_RUN=false
```

Posting requires `GITHUB_TOKEN` with Pull requests write permission.
It opens a PR titled `docs(changelog): prepare v28.0.1`, with a description explaining that
the release-maintainer workflow will create the signed release tag after review and merge.
It requires a clean, pushed branch that changes only `CHANGELOG.md` against the selected release branch.
If an open PR already exists for the same branches, it prints that PR's URL instead of creating a duplicate.
It does not commit or push the branch, or start a release.

Merge the reviewed changelog before tagging. For a new release line, create its release branch through the normal reviewed process first.

### Preview and create the signed release tag

In GitHub Actions, select **release-create-tag**, then **Run workflow**:

1. Select the release branch, such as `release/v28` or `release/v1.27`.
2. Enter the chosen version, such as `28.0.1` or `29.0.0-rc0` (an initial `v` is optional).
3. Leave **dry-run** enabled for the first run.

Both dry-run and publication require active membership in the configured release-maintainers team.
The original actor and the actor rerunning the workflow are both checked; API errors deny authorization.
The workflow rejects non-release branches, versions outside the selected release line, existing remote tags or GitHub releases,
and missing, duplicate, empty, or non-current changelog sections.
It pins the selected branch's commit and fails if the branch moves before tagging.

Dry-run writes the commit, tag, and exact tag message to the workflow summary.
It does not import the signing key, sign, push, or create a GitHub release.
After reviewing the preview, run the workflow again on the same branch and version with **dry-run** disabled.
The workflow creates and verifies a GPG-signed annotated tag containing that release's changelog,
rechecks authorization and remote version availability, and pushes only the new tag without force.
If the branch moved since the preview, review a fresh preview first.

The existing tag-triggered workflows then build and sign assets, upload them to the download server,
publish containers, and create the GitHub release using the tag annotation as release notes.
Stable versions create published releases; release candidates currently create draft releases.
A retry after the tag has been pushed intentionally fails the version check; retry the existing build workflow instead.

### GitHub configuration

Before enabling publication, configure the following in GitHub:

- Create an explicit organization team for release maintainers.
- Protect the `release/v*` branches with the required PR reviews and checks, including workflow changes.
- Create the **release-signing** environment and restrict its deployment branches to `release/v*`.
  Configure the release-maintainers team as a required reviewer, prevent self-review, and disable administrator bypass.
  These environment rules protect the signing secrets even if another workflow is edited to reference the environment.
- Set the environment variable `RELEASE_MAINTAINERS_TEAM` to that team's slug.
- Add environment secret `RELEASE_TEAM_TOKEN`: a token able to read organization team membership
  (organization Members read permission for a fine-grained token, or `read:org` for a classic token).
- Add environment secrets `RELEASE_TAG_GPG_KEY` and `RELEASE_TAG_GPG_PASSPHRASE` for a dedicated release-tag signing key.
  The key's primary identity must use `giteabot@users.noreply.github.com`, matching the workflow's public bot identity.
  Register the public key with the corresponding GitHub bot account for verified signatures.
- Add environment secret `RELEASE_TOKEN`: a GitHub App token or PAT with repository Contents write permission,
  allowed by the repository's tag rules. A PAT or App token is necessary because a tag pushed with `GITHUB_TOKEN`
  does not trigger the existing push workflows.

The workflow does not create the team, configure environment protection, or install secrets automatically.
The existing build workflows still need their current signing, R2, and container registry credentials.

### After publication

Verify the binaries, signatures, checksums, containers, and GitHub release before updating
`https://dl.gitea.com/gitea/version.json` or announcing the release.
Frontport the changelog to `main` if needed, prepare the [blog post](https://gitea.com/gitea/blog),
and announce the release in Discord after publication is confirmed.
