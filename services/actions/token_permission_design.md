# Actions Token Permission System Design

This document details the design of the Actions Token Permission system within Gitea, originally proposed in [#24635](https://github.com/go-gitea/gitea/issues/24635).

## Design Philosophy & GitHub Differences

Gitea Actions draws inspiration from GitHub Actions but diverges in its security philosophy regarding token permissions:
- On GitHub, workflows can escalate their permissions beyond the repository's default "Restricted" mode simply by declaring higher permissions in the workflow file.
- **In Gitea, workflows cannot bypass organizational or repository-level restrictions.** Because Gitea relies heavily on trust boundaries, allowing workflows to arbitrarily escalate permissions would pose a severe security risk.
- Therefore, Gitea relies on a **strict clamping mechanism**. The maximum allowable permissions are set at the Repository or Organization level, and any permissions requested by a workflow are strictly clamped by this ceiling policy.

## Terminology

### 1. `GITEA_TOKEN`
- The automatic token generated for each Actions job.
- Its permissions (read/write/none) are scoped to the repository and specific features (Code, Issues, etc.).

### 2. Token Permission Mode
- The default access level granted to a token when no explicit `permissions:` block is present in a workflow.
- **Permissive**: Grants `write` access to most repository scopes by default.
- **Restricted**: Grants `read` access (or none) to repository scopes by default.

### 3. Actions Token Permissions
- A structure representing the granular permission scopes available to a token.
- Includes scopes like: Code (contents), Issues, PullRequests, Packages, Actions, Wiki, Releases, and Projects.

### 4. Cross-Repository Access
- By default, a token can only access the repository where the workflow is running.
- Users and organizations can configure `ActionsCrossRepoMode` to allow the token to access other repositories they own.
- Allowed modes:
  - **None**: No cross-repository access allowed (default for enhanced security).
  - **All**: The token can access all repositories owned by the user/org (subject to the target repository's own permissions).
  - **Selected**: The token can access a specific list of repositories (`AllowedCrossRepoIDs`).

## Token Lifecycle & Permission Evaluation

When a job starts, Gitea evaluates the requested permissions for the `GITEA_TOKEN` through a multi-step clamping process:

### Step 1: Determine Base Permissions From Workflow
- If the job explicitly specifies a `permissions:` block, Gitea parses it.
- If the job inherits a top-level `permissions:` block, Gitea parses that.
- If no explicit permissions are found, Gitea uses the repository's default `TokenPermissionMode` (Permissive or Restricted) to generate base permissions.

### Step 2: Apply Repository Clamping
- Repositories can define `MaxTokenPermissions` in their Actions settings.
- The base permissions from Step 1 are clamped against these maximum allowed permissions.
- If the repository says `Issues: read` and the workflow requests `Issues: write`, the final token gets `Issues: read`.

### Step 3: Apply Organization/User Clamping (Hierarchical Override)
- If the repository belongs to an organization (or user) with its own `MaxTokenPermissions`, these restrictions cascade down.
- The repository's clamping limits cannot exceed the organization's limits UNLESS the repository explicitly enables `OverrideOrgConfig`.
- If `OverrideOrgConfig` is false, and the org sets `MaxTokenPermissions` to `read` for all scopes, no repository inside that org can grant `write` access, regardless of their own settings or the workflow's request.

## Parsing Priority for "contents" Scope

In GitHub Actions compatibility, the `contents` scope maps to multiple granular scopes in Gitea.
- `contents: write` maps to `Code: write` and `Releases: write`.
- When a workflow specifies both `contents` and a more granular scope (e.g., `code`), the granular scope takes absolute priority.

**Example YAML**:
```yaml
permissions:
  contents: write
  code: read
```
**Result**: The token gets `Code: read` (from granular) and `Releases: write` (from contents).

## Special Cases & Edge Scenarios

### 1. Empty Permissions Mapping (`permissions: {}`)
- Explicitly setting an empty mapping means "revoke all permissions".
- The token gets `none` for all scopes.

### 2. Fork Pull Requests
- Workflows triggered by Pull Requests from forks inherently operate in `Restricted` mode for security reasons.
- The base permissions are automatically downgraded to `read` (or `none`), preventing untrusted code from modifying the repository.

### 3. Public Repositories in Cross-Repo Access
- If `CrossRepoMode` is `None`, a token can still read from another repository in the same user/org IF that target repository is public and accessible anonymously.
