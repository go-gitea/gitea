# Immutable releases

A published immutable release never changes its tag, target, draft state or assets, and its tag name is never reused by its repository or at the path it was published at. Behavior follows github.com as tested, not its docs.

## Data

- `release.is_immutable` is only set by `LockRelease`, never by `UpdateRelease`. On an `is_tag` row it marks the tag of a deleted immutable release.
- `immutable_tag` permanently claims an exact tag name for a `repo_id` and for the lowercased owner and repository name at lock time.
- The Releases unit config `ImmutableReleases` enables it, `immutable_releases` in the API. Pull mirrors cannot enable it.

## Locking

- A release locks when created published, when a draft or tag becomes a published release, and on any edit of an existing published release, like on GitHub.
- A tag push does not publish a draft while enabled.
- Disabling keeps existing locks and claims.
- Tag, target, draft state and assets are locked. Title, notes, prerelease and latest stay editable.

## Tags

- The tag of an existing immutable release cannot be moved or deleted.
- Once the release is deleted, its tag can be deleted but never becomes a release again.
- A claimed name cannot be created again by the claiming repository or at the claimed path.
- A repository cannot arrive at a path with claims on its tag names. Fork, migration and adoption fail in `SyncReleasesWithTags`. Rename, transfer, owner rename and mirror conversion are rejected up front. A pull mirror itself is not covered.

## Limits

- Ref updates and API asset edits are not serialized with publication, so a push past the pre-receive check or an asset edit past its check can still change a release that locks meanwhile.
- No release attestations.
