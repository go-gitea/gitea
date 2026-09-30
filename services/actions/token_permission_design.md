# Actions Token Permission System Design

This document details the design of the Actions Token Permission system within Gitea, originally proposed in [#24635](https://github.com/go-gitea/gitea/issues/24635).

## Design Philosophy & GitHub Differences

Gitea Actions uses a **strict clamping mechanism** for token permissions.
While workflows can request explicit permissions that exceed the repository's default baseline
(e.g., requesting `write` when the default mode is `Restricted`),
these requests are always bounded by a hard ceiling.

The maximum allowable permissions (`MaxTokenPermissions`) are set at the Repository or Organization level.
**Any permissions requested by a workflow are strictly clamped by this ceiling policy.**
This ensures that workflows cannot bypass organizational or repository-level security restrictions.

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
- Includes scopes like: Code, Releases (both grouped under `contents` in workflow syntax),
  Issues, PullRequests, Actions, Wiki, and Projects.
- **Note**: The `Packages` scope is supported in workflow/job `permissions:` blocks
  but is currently hidden from the settings UI.

### 4. Cross-Repository Access
- By default, a token can access the repository where the workflow is running,
  as well as any **public repositories (read-only)** on the instance.
- Users and organizations can configure an `AllowedCrossRepoIDs` list in their owner-level settings
  to grant the token **read-only** access to other private/internal repositories they own.
- If the `AllowedCrossRepoIDs` list is empty, there is no cross-repository access
  to other private repositories (default for enhanced security).
- In any configuration, individual jobs can disable or limit cross-repo access
  by explicitly restricting their permissions (e.g., `permissions: none`).
- **Note on Forks**: Cross-repository access to private repositories is fundamentally denied
  for workflows triggered by fork pull requests (see [Special Cases](#2-fork-pull-requests)).

## Token Lifecycle & Permission Evaluation

When a job starts, Gitea evaluates the requested permissions for the `GITEA_TOKEN` through a multistep clamping process:

### Step 1: Determine Base Permissions From Workflow
- If the job explicitly specifies a valid `permissions:` block, Gitea parses it.
- If the job inherits a top-level `permissions:` block, Gitea parses that.
- If an invalid or unparseable `permissions:` block is specified, or no explicit permissions are defined at all,
  Gitea falls back to using the repository's default `TokenPermissionMode` (Permissive or Restricted)
  to generate base permissions.

### Step 2: Apply Repository Clamping
- Repositories can define `MaxTokenPermissions` in their Actions settings.
- The base permissions from Step 1 are clamped against these maximum allowed permissions.
- If the repository says `Issues: read` and the workflow requests `Issues: write`, the final token gets `Issues: read`.

### Step 3: Apply Organization/User Clamping (Hierarchical Override)
- The organization (or user) has an owner-level configuration (`UserActionsConfig`) containing `MaxTokenPermissions`,
  and these restrictions cascade down.
- The repository's clamping limits cannot exceed the owner's limits
  UNLESS the repository explicitly enables `OverrideOwnerConfig`.
- If `OverrideOwnerConfig` is false, and the owner sets `MaxTokenPermissions` to `read` for all scopes,
  no repository under that owner can grant `write` access, regardless of their own settings or the workflow's request.

## Parsing Priority for "contents" Scope

In GitHub Actions compatibility, the `contents` scope maps to multiple granular scopes in Gitea.
- `contents: write` maps to `Code: write` and `Releases: write`.
- When a workflow specifies both `contents` and a more granular scope (e.g., `code`),
  the granular scope takes absolute priority.

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
- The base permissions for the current repository are automatically downgraded to `read` (or `none`),
  preventing untrusted code from modifying the repository.
- **Cross-Repo Access in Forks**: For workflows triggered by fork pull requests, cross-repository access
  to other private repositories is strictly denied, regardless of the `AllowedCrossRepoIDs` configuration.
  Fork PRs can only read the target repository and truly public repositories.

### 3. Public Repositories in Cross-Repo Access
- As mentioned in Cross-Repository Access, truly public repositories can always be read by the token,
  regardless of the `AllowedCrossRepoIDs` setting. The allowed list only governs access
  to private/internal repositories owned by the same user or organization.

## Packages Registry

"Packages" belong to "owner" but not "repository". Although there is a function "linking a package to a repository",
in most cases it doesn't really work. When accessing a package, usually there is no information about a repository.
So the "packages" permission should be designed separately from other permissions.

A possible approach is like this: let owner set packages permissions, and make the repositories follow.

- On owner-level:
   - Add a "Packages" permission section
   - "Default permissions for all repositories" can be set to none/read/write
   - Set different permissions for selected repositories (if needed), like the "Collaborators" permission setting

- On repository-level:
   - Now a repository can have "Packages" permission
   - The repository-level "Packages" permission is clamped by the owner-level "Packages" permission
   - If the owner-level "Packages" permission for this repository is read,
     then the repository cannot set its "Packages" permission to write

Maybe reusing the "org teams" permission system is a good choice: bind a repository's Actions token to a team.
