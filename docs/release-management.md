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

- Let MAJOR, MINOR and PATCH be Major, Minor and Patch version numbers, PATCH should be rc1, rc2, 0, 1, ...... MAJOR.MINOR will be kept the same as milestones on github or gitea in future.
- Before releasing, confirm all the version's milestone issues or PRs has been resolved. Then discuss the release on Discord channel #maintainers and get agreed with almost all the owners and mergers. Or you can declare the version and if nobody is against it in about several hours.
- If this is a big version first you have to create PR for changelog on branch `main` with PRs with label `changelog` and after it has been merged do following steps:
  - Create `-dev` tag as `git tag -s -F release.notes vMAJOR.MINOR.0-dev` and push the tag as `git push origin vMAJOR.MINOR.0-dev`.
  - When CI has finished building tag then you have to create a new branch named `release/vMAJOR.MINOR`
- If it is bugfix version create PR for changelog on branch `release/vMAJOR.MINOR` and wait till it is reviewed and merged.
- Add a tag as `git tag -s -F release.notes vMAJOR.MINOR.PATCH`, release.notes file could be a temporary file to only include the changelog this version which you added to `CHANGELOG.md`.
- And then push the tag as `git push origin vMAJOR.MINOR.$`. CI will automatically create a release and upload all the compiled binary. (But currently it doesn't add the release notes automatically. Maybe we should fix that.)
- If needed send a frontport PR for the changelog to branch `main` and update the version in `docs/config.yaml` to refer to the new version.
- Send PR to [blog repository](https://gitea.com/gitea/blog) announcing the release.
- Verify all release assets were correctly published through CI on dl.gitea.com and GitHub releases. Once ACKed:
  - bump the version of https://dl.gitea.com/gitea/version.json
  - merge the blog post PR
  - announce the release in discord `#announcements`
