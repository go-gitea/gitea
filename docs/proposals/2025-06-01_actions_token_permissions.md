# Proposal: Support Configuring Permissions of Automatic Tokens of Actions Jobs

## Summary

This proposal documents the design for implementing configurable permissions for automatic tokens generated for Gitea Actions job runners.

## Background

When a Gitea Actions job is picked by a runner, an automatically generated short-lived token is provided. Currently, this token's permissions are undocumented and cannot be configured, creating both security risks (over-privileged tokens) and usability issues (unable to write releases).

## Design

### Permission Modes

Two modes at repo and org level:
- `permissive` (write by default)
- `restricted` (read by default)

### MaxTokenPermissions

Hard upper bound on token permissions that workflow declarations cannot exceed. Workflow declarations can only reduce permissions below this ceiling.

### Hierarchical Configuration

- **Org level**: `OwnerActionsConfig` with `TokenPermissionMode`, `MaxTokenPermissions`, `AllowedCrossRepoIDs`
- **Repo level**: `OverrideOwnerConfig` to override org settings

### Strict Clamping Model

Unlike GitHub's workflow-override-allowed model, Gitea uses strict clamping to prevent privilege escalation (since Gitea lacks code owners).

### Fork PR Handling

Tokens from fork pull requests are always restricted.

## Existing Implementation

The codebase already contains significant partial implementation in `models/actions/token_permissions.go`, `services/actions/permission_parser.go`, and `routers/web/shared/actions/general.go`.

## References

- Issue: https://github.com/go-gitea/gitea/issues/24635
- Existing Design: `services/actions/token_permission_design.md`
