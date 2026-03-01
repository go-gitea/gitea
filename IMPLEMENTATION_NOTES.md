# Actions Token Permissions Implementation

This document describes the implementation of configurable permissions for Actions job tokens (issue #24635).

## Overview

This feature allows repository administrators to configure default permissions for automatically generated Actions job tokens, and allows workflows to specify permissions at the workflow or job level using the `permissions:` keyword.

## Architecture

### Database Schema

**New Table: `action_token_permissions`**
- Stores default permission configurations for repositories and organizations
- Fields for each permission scope (contents, issues, pull-requests, packages, etc.)
- Unique constraint on (repo_id, org_id) to prevent duplicates

**Modified Table: `action_task`**
- Added `token_scopes` field to store the calculated permissions for each job token
- Stores comma-separated AccessTokenScope values

### Permission Resolution Flow

1. **Task Creation** (`CreateTaskForRunner`)
   - Parse workflow YAML to extract `permissions:` field
   - Get repository default permissions from database
   - Merge workflow/job permissions with repository defaults
   - Calculate final token scopes
   - Store in `ActionTask.TokenScopes`

2. **Token Validation** (existing middleware)
   - Token scopes are validated against requested operations
   - Uses existing AccessTokenScope infrastructure

### Components

#### Models (`models/actions/`)

- `token_permissions.go` - Database model for permission configuration
  - `ActionTokenPermissions` struct
  - CRUD operations
  - Conversion to AccessTokenScope

#### Modules (`modules/actions/`)

- `permissions.go` - Workflow permission parsing
  - `WorkflowPermissions` struct
  - YAML parsing for `permissions:` field
  - Support for both string (`read-all`, `write-all`) and map formats
  - Conversion to AccessTokenScope

#### API (`routers/api/v1/repo/`)

- `actions_permissions.go` - REST API endpoints
  - `GET /repos/{owner}/{repo}/actions/permissions`
  - `PUT /repos/{owner}/{repo}/actions/permissions`

#### Converters (`services/convert/`)

- `actions_permissions.go` - Model to API struct conversion

#### Migrations (`models/migrations/v1_26/`)

- `v326.go` - Database migration
  - Creates `action_token_permissions` table
  - Adds `token_scopes` column to `action_task`

## Features

### 1. Repository Default Permissions

Administrators can configure default permissions via:
- Web UI (Settings → Actions → Permissions)
- REST API

Supported permission levels:
- `read` - Read-only access
- `write` - Read and write access
- `none` - No access

### 2. Workflow-Level Permissions

Workflows can specify permissions using the `permissions:` keyword:

```yaml
permissions:
  contents: read
  issues: write
  pull-requests: write
```

Or shorthand:
```yaml
permissions: read-all  # or write-all, or {}
```

### 3. Job-Level Permissions

Individual jobs can override workflow permissions:

```yaml
jobs:
  deploy:
    permissions:
      contents: write
      packages: write
    steps:
      - run: deploy.sh
```

### 4. Fork Pull Request Protection

Tokens for workflows triggered by fork pull requests are automatically restricted to read-only, regardless of configured permissions.

## Permission Scopes

Mapping between workflow permissions and AccessTokenScope:

| Workflow Permission | AccessTokenScope |
|-------------------|------------------|
| `contents` | `read:repository` / `write:repository` |
| `issues` | `read:issue` / `write:issue` |
| `pull-requests` | `read:issue` / `write:issue` |
| `packages` | `read:package` / `write:package` |
| `metadata` | `read:repository` (always) |
| `organization` | `read:organization` / `write:organization` |
| `notifications` | `read:notification` / `write:notification` |

## Security Considerations

1. **Fork PR Protection** - Automatic read-only restriction for fork PRs
2. **Metadata Always Readable** - Repository metadata is always at least readable
3. **Validation** - Permission values are validated before storage
4. **Least Privilege** - Default is read-only access

## Compatibility

- **GitHub Actions Compatible** - Syntax matches GitHub Actions `permissions:` keyword
- **Backward Compatible** - Existing workflows without `permissions:` use repository defaults
- **Migration Safe** - New columns have default values

## Testing

### Unit Tests

- `models/actions/token_permissions_test.go` - Model tests
- `modules/actions/permissions_test.go` - Parser tests

### Integration Tests

- `tests/integration/actions_permissions_test.go` - End-to-end tests
- API endpoint tests
- Workflow execution tests

### Test Scenarios

1. Default permissions (no workflow permissions specified)
2. Workflow-level permissions override
3. Job-level permissions override
4. Fork PR read-only enforcement
5. Permission validation
6. API CRUD operations

## API Examples

### Get Permissions

```bash
curl -H "Authorization: token YOUR_TOKEN" \
  https://gitea.example.com/api/v1/repos/owner/repo/actions/permissions
```

Response:
```json
{
  "default_permissions": "read",
  "contents_permission": "write",
  "issues_permission": "write",
  "pull_requests_permission": "write",
  "packages_permission": "read",
  "metadata_permission": "read"
}
```

### Set Permissions

```bash
curl -X PUT \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_permissions": "read",
    "contents_permission": "write",
    "issues_permission": "write"
  }' \
  https://gitea.example.com/api/v1/repos/owner/repo/actions/permissions
```

## Future Enhancements

1. **Organization-Level Defaults** - Default permissions for all repos in an org
2. **Web UI** - Settings page for configuring permissions
3. **Audit Logging** - Log permission changes
4. **Permission Templates** - Predefined permission sets
5. **Cross-Repo Access** - Permissions for accessing other repos in same org

## Migration Guide

### For Repository Administrators

1. Review existing workflows
2. Configure repository default permissions via API or UI
3. Test workflows with new permissions
4. Update workflows to use `permissions:` keyword where needed

### For Workflow Authors

1. Add `permissions:` to workflows that need write access
2. Use job-level permissions for fine-grained control
3. Follow principle of least privilege

## References

- Issue: #24635
- GitHub Actions Permissions: https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions#permissions
- Gitea AccessTokenScope: `models/auth/access_token_scope.go`
