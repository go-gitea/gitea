# Owner Repositories Design

Users and organizations ("owners") can have repositories with reserved names that affect their profile or their other repositories.

## Names

| Repository | Used for |
| --- | --- |
| `.profile` | Profile README |
| `.profile-private` | Members-only profile README of an organization |
| `.gitea` | Profile README and defaults for all repositories of the owner |
| `.github` | Same as `.gitea` for compatibility with GitHub, used for whatever `.gitea` lacks |

New owner-wide content goes into `.gitea` and `.github` instead of new names.

## Profile README

The profile README is the first readable one of `.profile/README.md`, `.gitea/profile/README.md` and `.github/profile/README.md`.
It lives in `profile/` like on GitHub, so the root README of `.gitea` keeps describing the repository itself.

## Templates

Issue templates, including their config, and pull request templates resolve separately.
For each, the source is the first of the target repository, the owner's `.gitea` and the owner's `.github` that has any file of that type.
Types are never merged across repositories, like on GitHub.

## Permissions

Owner repositories are read with the doer's permission, so a private `.gitea` only applies to users who can read it.
GitHub instead requires these repositories to be public.
