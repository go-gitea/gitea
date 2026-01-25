# Actions Token Permissions

Gitea Actions provides automatic tokens (`GITHUB_TOKEN` / `GITEA_TOKEN`) to workflows for authentication. This document explains how to configure permissions for these tokens.

## Overview

By default, Actions job tokens have read-only access to repository contents. You can configure permissions at three levels:

1. **Repository-level defaults** - Set default permissions for all workflows in a repository
2. **Workflow-level** - Override repository defaults for all jobs in a workflow
3. **Job-level** - Override workflow permissions for specific jobs

## Permission Scopes

The following permission scopes are available:

- `contents` - Repository contents (code, commits, branches, tags)
- `issues` - Issues
- `pull-requests` - Pull requests
- `packages` - Packages
- `metadata` - Repository metadata (always at least `read`)
- `actions` - Actions workflows and runs
- `organization` - Organization settings
- `notifications` - Notifications

Each scope can be set to:
- `read` - Read-only access
- `write` - Read and write access
- `none` - No access

## Configuring Repository Defaults

### Via API

```bash
curl -X PUT https://gitea.example.com/api/v1/repos/owner/repo/actions/permissions \
  -H "Authorization: token YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "default_permissions": "read",
    "contents_permission": "write",
    "issues_permission": "write",
    "pull_requests_permission": "write",
    "packages_permission": "read"
  }'
```

### Via Web UI

1. Navigate to your repository
2. Go to Settings → Actions → Permissions
3. Configure default permissions
4. Save changes

## Workflow-Level Permissions

Add a `permissions` key at the workflow level:

```yaml
name: CI

# Set permissions for all jobs in this workflow
permissions:
  contents: read
  issues: write
  pull-requests: write

on: [push, pull_request]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: echo "Building..."
```

### Shorthand Syntax

```yaml
# Grant read access to all scopes
permissions: read-all

# Grant write access to all scopes
permissions: write-all

# Disable all permissions (except metadata)
permissions: {}
```

## Job-Level Permissions

Override workflow permissions for specific jobs:

```yaml
name: CI

permissions:
  contents: read

on: [push]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "Building with read-only access"

  deploy:
    runs-on: ubuntu-latest
    # Override workflow permissions for this job
    permissions:
      contents: write
      packages: write
    steps:
      - run: echo "Deploying with write access"
```

## Examples

### Release Workflow

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write  # Create releases
  packages: write  # Publish packages

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Create Release
        run: |
          # Use GITHUB_TOKEN to create release
          gh release create ${{ github.ref_name }}
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Issue Management

```yaml
name: Issue Triage

on:
  issues:
    types: [opened]

permissions:
  issues: write

jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - name: Add label
        run: |
          gh issue edit ${{ github.event.issue.number }} --add-label "triage"
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Read-Only CI

```yaml
name: CI

# Explicitly set read-only permissions
permissions:
  contents: read

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: npm test
```

## Fork Pull Requests

For security, tokens for workflows triggered by fork pull requests are automatically restricted to read-only access, regardless of configured permissions.

## Best Practices

1. **Principle of Least Privilege** - Grant only the minimum permissions needed
2. **Use Job-Level Permissions** - Restrict permissions to specific jobs that need them
3. **Avoid `write-all`** - Explicitly specify required permissions instead
4. **Review Workflow Permissions** - Regularly audit permissions in your workflows
5. **Use Secrets for Sensitive Operations** - For operations requiring elevated permissions, use personal access tokens stored as secrets

## Migration from GitHub Actions

Gitea Actions permissions syntax is compatible with GitHub Actions. Existing workflows using `permissions:` will work without modification.

## API Reference

### Get Repository Permissions

```
GET /api/v1/repos/{owner}/{repo}/actions/permissions
```

### Set Repository Permissions

```
PUT /api/v1/repos/{owner}/{repo}/actions/permissions
```

Request body:
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

## Troubleshooting

### Permission Denied Errors

If you encounter permission errors:

1. Check workflow/job `permissions` configuration
2. Verify repository default permissions
3. Ensure the operation is allowed for the token scope
4. For fork PRs, remember tokens are read-only

### Debugging Permissions

Add this step to your workflow to see effective permissions:

```yaml
- name: Debug Permissions
  run: |
    echo "Token scopes: ${{ toJSON(github.token) }}"
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
