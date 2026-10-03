# Release management

This document describes the release cycle, backports, versioning, and the release manager checklist. For everyday contribution workflow, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Backports and Frontports

### What is backported?

We backport PRs given the following circumstances:

1. During feature freeze, we backport bug- and security-fixes and small enhancements. Large changes such as refactors are not backported.
2. We never backport new features.
3. We never backport breaking changes except when
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

- The release manager checks with the other maintainers before tagging and publishing the stable release (e.g. `v28.0.1`).

### Feature freeze

- Merge feature PRs before the freeze when you can.
- Feature PRs still open at the freeze move to the next milestone. Watch Discord for the freeze announcement.
- During the freeze, a **release branch** takes fixes backported from `main`. Stable releases for that line are maintained from that branch.

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
2. Enter the chosen version, such as `28.0.1` or `29.0.0` (an initial `v` is optional).
3. Leave **dry-run** enabled and review the generated release notes in the workflow summary.
4. Run again with **dry-run** disabled to sign and push the release commit and tag.

Both the original actor and the actor rerunning the workflow must have the Maintain or Admin repository role.
API failures deny authorization. The workflow rejects non-release branches, versions outside the selected
release line, existing remote tags or GitHub releases, and release branches that have moved during the run.

[git-cliff](https://git-cliff.org/) generates notes from the nearest stable tag reachable
from the selected branch to the selected commit. Development tags and tags on unrelated branches are ignored.
`cliff.toml` groups Conventional Commits and excludes `chore`, `ci`, translation synchronization,
and previous release marker commits. Historical commits without a conventional type appear under MISC.
Release notes appear in the preview and are regenerated from the same Git range and `cliff.toml`
configuration when publishing the GitHub release. They are not duplicated in commit or tag messages.

Dry-run does not import the signing key, create a commit or tag, or push anything.
Publication creates an empty GPG-signed commit and a GPG-signed annotated tag, both with the version
as their message. After verifying both signatures and repeating the
permission and version checks, it atomically pushes the branch and tag without force.
If the branch changed after a preview, review a fresh preview before publishing.

The existing tag-triggered workflows build and sign assets, upload them to `dl.gitea.com` through R2,
publish containers, and create the GitHub release using notes generated from the commit history.
Stable versions create published releases.
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

To prepare downstream updates after verifying a stable release, run:

```bash
node tools/release-updates.ts 28.0.1 --workdir /path/to/release-updates
```

The script needs Node.js (the version required by `package.json`), Git, SSH access to gitea.com,
and an authenticated `tea` login named
`gitea.com` (override with `--login`). It uses dedicated clean checkouts to preview
updates to deployment's root `version.json`, Helm's `appVersion`, and Terraform's
Gitea test image. Equal or newer versions are skipped; prereleases are rejected.
Documentation updates remain manual.

Add `--publish` to confirm the displayed diffs and exact PR targets, titles, and bodies,
then commit with the public Gitea Release Bot identity, push release branches, and open
PRs using `tea`. The login needs branch push and PR creation access to all three repositories.
Reruns reuse the branches and existing open PRs; pushes never force-update branches.
If a push or PR creation fails, rerun the same command to finish the remaining updates.
Merging the deployment PR triggers its existing upload and public-file verification workflow.
