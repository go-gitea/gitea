# Proposal: Support Configuring Permissions of Automatic Tokens of Actions Jobs

- Start Date: 2025-06-01
- Author(s): @Ikalus1988 (Misaka10004)
- Related Issue: [#24635](https://github.com/go-gitea/gitea/issues/24635)
- Status: Draft

## Summary

This proposal outlines a complete design for configuring the permissions of automatic job tokens in Gitea Actions. When a Gitea Actions job is picked up by a runner, Gitea generates a short-lived JWT token that the runner uses to authenticate API calls. This token currently grants an opaque and poorly documented set of permissions. This proposal provides a way to explicitly configure and restrict these permissions at the repository and organization level, improving both security and usability.

## Motivation

The current behavior of the automatic token is not clearly documented and is inconsistent across different use cases:

1. **Lack of Write Access to Packages**: The token cannot write packages, making it impossible to use the automatic token for releasing artifacts.
2. **Over-Privileged by Default**: In some configurations, the token may have more access than intended, creating a security risk.
3. **No User Control**: Users have no way to restrict the token's permissions for their repositories.
4. **Inconsistent Fork Behavior**: Tokens from fork pull requests may have more access than expected.

## Prior Art

This proposal is heavily inspired by GitHub's Actions token permission system but with modifications appropriate for Gitea's architecture and security model. The key differences are:

1. **Strict Ceiling Model**: Unlike GitHub, where workflow files can override the default mode to obtain higher permissions, Gitea uses a strict ceiling model where `MaxTokenPermissions` acts as an absolute upper bound that workflows cannot exceed. This is necessary because Gitea lacks GitHub's "code owners" feature, which prevents write-access users from escalating their token's permissions.
2. **Hierarchical Configuration**: Permissions can be configured at the organization level and overridden at the repository level (with explicit opt-in).
3. **Packages as a Separate Concern**: Package permissions are managed at the owner level due to packages belonging to owners rather than repositories.

## Design

### 1. Token Permission Modes

At both the repository and organization level, administrators can choose between two default modes:

| Mode | Description |
|------|-------------|
| `permissive` | Grants `write` access to most repository scopes by default (current behavior, backwards compatible). |
| `restricted` | Grants `read` access to most repository scopes by default (safer for untrusted contributions). |

The default mode is `permissive` to maintain backwards compatibility.

### 2. Granular Permissions (`MaxTokenPermissions`)

Beyond the simple mode, administrators can set a `MaxTokenPermissions` object that defines the absolute maximum permissions for any token. This is a hard ceiling — workflow-level permission declarations can only reduce permissions, never exceed this ceiling.

The supported permission scopes are:

| Scope | Description | Maps to Gitea Units |
|-------|-------------|---------------------|
| `contents` | Read/write access to repository contents and releases | `unit.TypeCode`, `unit.TypeReleases` |
| `code` | Read/write access to repository code only | `unit.TypeCode` |
| `issues` | Read/write access to issues | `unit.TypeIssues` |
| `pull-requests` | Read/write access to pull requests | `unit.TypePullRequests` |
| `packages` | Read/write access to packages | `unit.TypePackages` |
| `actions` | Read/write access to actions | `unit.TypeActions` |
| `wiki` | Read/write access to wiki | `unit.TypeWiki` |
| `releases` | Read/write access to releases | `unit.TypeReleases` |
| `projects` | Read/write access to projects | `unit.TypeProjects` |

#### Special Shorthand Values

In workflow YAML, the `permissions:` block also supports:

- `read-all` — Sets all scopes to `read`.
- `write-all` — Sets all scopes to `write`.
- `none` — Revokes all permissions.

#### Workflow-Level Permission Declarations

Workflow files can declare permissions at the workflow level or job level:

```yaml
name: My Workflow
on: [push]

permissions:
  contents: write
  issues: read

jobs:
  build:
    permissions:
      packages: write
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: some/action@v1
```

The effective permissions are computed as:
```
effective = clamp(job_perms, clamp(repo_perms, owner_perms))
```

Where `clamp(a, b)` takes the minimum of each scope in `a` and `b`.

### 3. Repository-Level Override

By default, repositories inherit the organization-level token permissions. A repository can opt out of this inheritance and define its own permissions by enabling `OverrideOwnerConfig` in its Actions settings.

When `OverrideOwnerConfig` is enabled, the repository's `MaxTokenPermissions` and `TokenPermissionMode` take precedence, and the organization-level settings are ignored for that repository.

### 4. Cross-Repository Access

By default, a job token can access:
- The repository where the workflow runs (with configured permissions).
- All public repositories on the instance (read-only).

For private cross-repository access within the same organization, administrators can configure `AllowedCrossRepoIDs` at the organization level. This allows job tokens from specified repositories to have read access to other private repositories owned by the same organization.

**Security Note**: Cross-repository access is always read-only, even when the token has write permission for the source repository.

### 5. Fork Pull Request Handling

For workflows triggered by pull requests from forked repositories, cross-repository access and write permissions are strictly disabled. The token's effective permissions are always clamped to `MakeRestrictedPermissions()` (read-only for `contents`, `packages`, `releases`; none for all other scopes). This ensures that untrusted code from forks cannot modify the target repository.

### 6. Token Lifecycle

The automatic token is a short-lived JWT signed with the instance's `GeneralTokenSigningSecret`. It contains:

```json
{
  "scp": "Actions.Results:<runID>:<jobID>",
  "task_id": 123,
  "run_id": 456,
  "job_id": 789,
  "ac": "[{\"Scope\":\"\",\"Permission\":2}]",
  "exp": <expiry>,
  "nbf": <not_before>
}
```

The `ac` (actions cache) field encodes the permission scopes as a JSON array.

## Implementation

### Key Files

| File | Purpose |
|------|---------|
| `models/repo/repo_unit_actions.go` | Defines `ActionsTokenPermissions`, `ActionsTokenPermissionMode`, `ActionsConfig` structs |
| `models/actions/config.go` | Defines `OwnerActionsConfig` with `GetDefaultTokenPermissions`, `GetMaxTokenPermissions`, `ClampPermissions` |
| `models/actions/token_permissions.go` | `ComputeTaskTokenPermissions` — central permission computation logic |
| `services/actions/permission_parser.go` | Parses `permissions:` YAML blocks in workflow files |
| `routers/web/shared/actions/general.go` | Web handler for parsing token permission form data |
| `routers/web/repo/setting/actions.go` | Repository settings page handler |

### Code Flow: Permission Computation

```
ComputeTaskTokenPermissions(ctx, task, targetRepo)
  1. Load the job and its repository
  2. Get repo-level ActionsConfig (OverrideOwnerConfig, MaxTokenPermissions, TokenPermissionMode)
  3. Get owner-level OwnerActionsConfig (MaxTokenPermissions, TokenPermissionMode)
  4. Determine base permissions:
     - If job has explicit TokenPermissions → use them
     - Else if OverrideOwnerConfig → use repo's default mode
     - Else → use owner's default mode
  5. Apply clamping:
     - If OverrideOwnerConfig → clamp by repo's MaxTokenPermissions
     - Else → clamp by owner's MaxTokenPermissions
  6. If fork PR or cross-repo access → clamp to restricted (read-only)
  7. Return effective permissions
```

### Example Configuration

**Repository Actions Settings (`repo ActionsConfig`)**:
```json
{
  "token_permission_mode": "permissive",
  "max_token_permissions": {
    "unit_access_modes": {
      "contents": "write",
      "issues": "write",
      "packages": "read",
      "pull-requests": "write",
      "releases": "write"
    }
  },
  "override_owner_config": true
}
```

**Organization Actions Settings (`OwnerActionsConfig`)**:
```json
{
  "token_permission_mode": "restricted",
  "max_token_permissions": {
    "unit_access_modes": {
      "contents": "write",
      "issues": "write",
      "packages": "none",
      "pull-requests": "read"
    }
  },
  "allowed_cross_repo_ids": [12345, 67890]
}
```

## Backwards Compatibility

- **Default**: `TokenPermissionMode` defaults to `permissive`, preserving existing behavior.
- **Existing workflows**: Unchanged; they continue to work as before.
- **Fork PRs**: Fork pull request tokens were already read-only in practice; this proposal formalizes and documents this behavior.

## Future Work

1. **Packages Permission UI**: Currently, the `packages` permission is implemented but hidden from the settings UI.
2. **Per-Repository Package Permissions**: Allow fine-grained control over which repositories can access which packages.
3. **Organization Teams Integration**: Bind Actions token permissions to organization teams using the existing permission system.
4. **Audit Logging**: Log when a token is used to access resources, for security auditing.

## References

- GitHub Actions Permissions Documentation: <https://docs.github.com/en/actions/security-guides/automatic-token-authentication>
- GitHub Workflow Syntax for `permissions`: <https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#permissions>
- Original Issue: [#24635](https://github.com/go-gitea/gitea/issues/24635)
