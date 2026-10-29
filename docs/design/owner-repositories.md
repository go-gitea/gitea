# Owner Repositories Design

Users and organizations ("owners") can have repositories with reserved names that affect their profile or their other repositories.
First discussed in https://github.com/go-gitea/gitea/pull/39924.

## Names

| Repository | Used for |
| --- | --- |
| `.profile` | Profile README |
| `.profile-private` | Members-only profile README of an organization |
| `.gitea` | Profile README and defaults for all repositories of the owner |
| `.github` | Same as `.gitea` for compatibility with GitHub, used for whatever `.gitea` lacks |

New owner-wide content, for example contributing guidelines, goes into `.gitea` and `.github` instead of new names.
`.profile` keeps precedence, so `.gitea` can replace it without breaking existing profiles.
`.profile-private` has no replacement, because one repository cannot be public and members-only at once.

## Profile README

The profile README is the first readable one of `.profile/README.md`, `.gitea/profile/README.md` and `.github/profile/README.md`.
It lives in `profile/` like on GitHub, because the root README of `.gitea` describes that repository,
and every `.gitea` created with an initial README would otherwise become a profile.
Moving from `.profile` to `.gitea` means renaming the repository and moving `README.md` into `profile/`.

## Templates

Issue templates, including their config, and pull request templates resolve separately.
For each, the source is the first of the target repository, the owner's `.gitea` and the owner's `.github` that has any file of that type,
at the paths a repository uses for its own templates.
Types are never merged across repositories, matching [GitHub](https://docs.github.com/en/communities/setting-up-your-project-for-healthy-contributions/creating-a-default-community-health-file),
but presence decides, not validity, so an invalid template of the target repository shows its error instead of falling back.

An explicit `?template=` path or submitted `template-file` keeps the target repository as the source when the file exists there.
It never selects an owner repository.

## Permissions

Owner repositories are read with the doer's permission from `GetDoerRepoPermission`, like `.profile-private`, so token restrictions apply as usual.
Unlike on GitHub, `.gitea` can be private. Users who cannot read it get the defaults of `.github`, or none,
so different users can see different templates for the same repository.
`blank_issues_enabled: false` stays a UI default, issue creation never enforced it.

## Entry Points

"New Issue" buttons always link to `/issues/new/choose`, which redirects to `/issues/new` when there is nothing to choose,
so issue lists, issue pages and milestone pages never look up templates.
The template and config API endpoints use the same source repository as the web pages.

## Upgrading

Existing `.gitea` and `.github` repositories, including ones migrated or mirrored from GitHub, take effect on upgrade.
A repository opts out of a template type by adding its own file of that type, for issue templates a `.gitea/ISSUE_TEMPLATE/config.yaml` is enough.
