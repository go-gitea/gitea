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

### Preview and publish

In GitHub Actions, select **release-create-tag**, then **Run workflow**:

1. Select the release branch, such as `release/v28` or `release/v1.27`.
2. Enter the chosen version, such as `28.0.1` or `29.0.0-rc0` (an initial `v` is optional).
3. Leave **dry-run** enabled and review the generated release notes in the workflow summary.
4. Run again with **dry-run** disabled to sign and push the release commit and tag.

Both the original actor and the actor rerunning the workflow must have the Maintain or Admin repository role.
API failures deny authorization. The workflow rejects non-release branches, versions outside the selected
release line, existing remote tags or GitHub releases, and release branches that have moved during the run.

[git-cliff](https://git-cliff.org/) generates notes from the nearest stable or release-candidate tag reachable
from the selected branch to the selected commit. Development tags and tags on unrelated branches are ignored.
`cliff.toml` groups Conventional Commits and excludes `chore`, `ci`, translation synchronization,
and previous release marker commits. Historical commits without a conventional type appear under MISC.
Release notes are stored in the signed commit, tag annotation, and GitHub release. The existing changelog is preserved in `CHANGELOG-archived.md`; new releases do not add to it.

Dry-run does not import the signing key, create a commit or tag, or push anything.
Publication creates an empty GPG-signed commit titled with the version, with the notes in its body,
and a GPG-signed annotated tag with the same notes. After verifying both signatures and repeating the
permission and version checks, it atomically pushes the branch and tag without force.
If the branch changed after a preview, review a fresh preview before publishing.

The existing tag-triggered workflows build and sign assets, upload them to `dl.gitea.com` through R2,
publish containers, and create the GitHub release using the tag annotation as release notes.
Stable versions create published releases; release candidates currently create draft releases.
Snap publishing and GitHub release immutability configuration remain managed as before.
A retry after the tag has been pushed intentionally fails the version check; retry the existing build workflow instead.

### GitHub configuration

- Protect `release/v*` branches with the required reviews and checks, including workflow changes.
- Create the **release-signing** environment and restrict its deployment branches to `release/v*`.
  Configure trusted release reviewers and the appropriate environment protection rules for the signing secrets.
- Add environment secrets `RELEASE_TAG_GPG_KEY` and `RELEASE_TAG_GPG_PASSPHRASE` for a dedicated release signing key.
  Its primary identity must use `giteabot@users.noreply.github.com`, matching the workflow's public bot identity.
  Register the public key with the corresponding GitHub bot account for verified signatures.
- Add environment secret `RELEASE_TOKEN`: a GitHub App token or PAT with repository Contents write permission.
  It must be able to read collaborator permissions and be allowed by branch and tag rules to push the signed marker commit
  and tag. A PAT or App token is necessary because pushes with `GITHUB_TOKEN` do not trigger the existing push workflows.

The workflow must be present on the default branch to appear in the Actions dispatch menu,
and on the selected release branch to run there. Install it through the normal reviewed process.
The workflow does not configure repository settings or install secrets automatically.
The existing build workflows still need their current signing, R2, and registry credentials.

### After publication

Verify the binaries, signatures, checksums, containers, and GitHub release before updating
`https://dl.gitea.com/gitea/version.json` or announcing the release.
The [blog post](https://gitea.com/gitea/blog) remains an optional manual step.
