// Package actions provides services for Gitea Actions.
// Modified to include configurable permissions in job tokens. Modified by LAC | Ludwig investing
package actions

import (
	"context"
	"fmt"
	"time"

	actions_model "code.gitea.io/gitea/models/actions"
	"code.gitea.io/gitea/models/db"
	"github.com/golang-jwt/jwt/v4"
)

// CreateJobToken generates a JWT token for a job with embedded permissions.
// Modified to include configurable permissions. Modified by LAC | Ludwig investing
func CreateJobToken(ctx context.Context, run *actions_model.ActionRun, job *actions_model.ActionRunJob) (string, error) {
	now := time.Now()
	expiresAt := now.Add(1 * time.Hour) // token lifetime

	claims := jwt.MapClaims{
		"iat":        now.Unix(),
		"exp":        expiresAt.Unix(),
		"job_id":     job.ID,
		"run_id":     run.ID,
		"repo_id":    run.RepoID,
		"is_fork_pr": run.IsForkPullRequest,
	}

	// Compute effective permissions based on repo/org settings, workflow file, and fork status.
	// Added to support configurable permissions. Modified by LAC | Ludwig investing
	perms, err := computeEffectivePermissions(ctx, run, job)
	if err != nil {
		return "", fmt.Errorf("computeEffectivePermissions: %w", err)
	}
	claims["permissions"] = perms

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Use a secret key from settings (placeholder)
	secret := []byte("gitea-actions-secret") // TODO: use actual secret
	signedToken, err := token.SignedString(secret)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}

// computeEffectivePermissions calculates the final permissions for a job token.
// It merges base permissions (repo/org settings), workflow file permissions,
// applies fork PR restrictions, and adds cross-repo and package access.
// Added to implement configurable permissions. Modified by LAC | Ludwig investing
func computeEffectivePermissions(ctx context.Context, run *actions_model.ActionRun, job *actions_model.ActionRunJob) (map[string]interface{}, error) {
	// 1. Base permissions: repo settings > org settings > hardcoded defaults
	basePerms := getDefaultPermissions()
	repoPerms, err := actions_model.GetRepoActionsPermissions(ctx, run.RepoID)
	if err == nil && repoPerms != nil {
		basePerms, err = repoPerms.GetPermissionsMap()
		if err != nil {
			return nil, err
		}
	} else if run.Repo.Owner.IsOrganization() {
		orgPerms, err := actions_model.GetOrgActionsPermissions(ctx, run.Repo.OwnerID)
		if err == nil && orgPerms != nil {
			basePerms, err = orgPerms.GetPermissionsMap()
			if err != nil {
				return nil, err
			}
		}
	}

	// 2. Workflow file permissions: intersect with base (take minimum)
	workflowPerms := parseWorkflowPermissions(run, job)
	effective := intersectPermissions(basePerms, workflowPerms)

	// 3. Fork PR restriction: downgrade write to read
	if run.IsForkPullRequest {
		for scope, perm := range effective {
			if perm == actions_model.PermissionWrite {
				effective[scope] = actions_model.PermissionRead
			}
		}
	}

	// 4. Cross-repo access: add claim for target repos
	crossAccess, err := getCrossRepoAccessForToken(ctx, run.RepoID)
	if err != nil {
		return nil, err
	}

	// 5. Package access: add claim for packages
	pkgAccess, err := getPackageAccessForToken(ctx, run.RepoID)
	if err != nil {
		return nil, err
	}

	// Convert to map for JSON serialization
	result := make(map[string]interface{})
	for scope, perm := range effective {
		result[string(scope)] = perm.String()
	}
	if len(crossAccess) > 0 {
		result["cross_repo_access"] = crossAccess
	}
	if len(pkgAccess) > 0 {
		result["package_access"] = pkgAccess
	}
	return result, nil
}

// getDefaultPermissions returns the hardcoded default permissions (current behavior).
// These are used when no repo/org settings are configured.
// Added to provide backward compatibility. Modified by LAC | Ludwig investing
func getDefaultPermissions() map[actions_model.Scope]actions_model.Permission {
	return map[actions_model.Scope]actions_model.Permission{
		actions_model.ScopeActions:          actions_model.PermissionWrite,
		actions_model.ScopeChecks:          actions_model.PermissionWrite,
		actions_model.ScopeContents:        actions_model.PermissionWrite,
		actions_model.ScopeDeployments:     actions_model.PermissionWrite,
		actions_model.ScopeIssues:          actions_model.PermissionWrite,
		actions_model.ScopePackages:        actions_model.PermissionNone, // packages are not writable by default
		actions_model.ScopePullRequests:    actions_model.PermissionWrite,
		actions_model.ScopeRepositoryProjects: actions_model.PermissionWrite,
		actions_model.ScopeStatuses:        actions_model.PermissionWrite,
	}
}

// parseWorkflowPermissions extracts permissions from the workflow file.
// It merges workflow-level and job-level permissions (job overrides workflow).
// Added to support workflow-defined permissions. Modified by LAC | Ludwig investing
func parseWorkflowPermissions(run *actions_model.ActionRun, job *actions_model.ActionRunJob) map[actions_model.Scope]actions_model.Permission {
	// TODO: parse the actual workflow YAML from run.WorkflowID
	// For now, return nil meaning no restrictions (use base).
	return nil
}

// intersectPermissions returns the minimum permission for each scope.
// If workflowPerms is nil, base is returned unchanged.
// Added to enforce that workflow can only restrict, not expand. Modified by LAC | Ludwig investing
func intersectPermissions(base, workflow map[actions_model.Scope]actions_model.Permission) map[actions_model.Scope]actions_model.Permission {
	if workflow == nil {
		return base
	}
	result := make(map[actions_model.Scope]actions_model.Permission)
	for scope, basePerm := range base {
		if wfPerm, ok := workflow[scope]; ok {
			if wfPerm < basePerm {
				result[scope] = wfPerm
			} else {
				result[scope] = basePerm
			}
		} else {
			result[scope] = basePerm
		}
	}
	return result
}

// getCrossRepoAccessForToken retrieves cross-repo access rules for the source repo.
// Added to support cross-repo access. Modified by LAC | Ludwig investing
func getCrossRepoAccessForToken(ctx context.Context, sourceRepoID int64) (map[int64]map[string]string, error) {
	accesses, err := actions_model.GetCrossRepoAccess(ctx, sourceRepoID)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]map[string]string)
	for _, a := range accesses {
		perms, err := a.GetPermissionsMap()
		if err != nil {
			return nil, err
		}
		permMap := make(map[string]string)
		for scope, perm := range perms {
			permMap[string(scope)] = perm.String()
		}
		result[a.TargetRepoID] = permMap
	}
	return result, nil
}

// getPackageAccessForToken retrieves package access rules for the repo.
// Added to support package access. Modified by LAC | Ludwig investing
func getPackageAccessForToken(ctx context.Context, repoID int64) (map[int64]string, error) {
	accesses, err := actions_model.GetPackageAccess(ctx, repoID)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]string)
	for _, a := range accesses {
		result[a.PackageID] = a.Permission
	}
	return result, nil
}
