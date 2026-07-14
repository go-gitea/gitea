// Package actions provides models and constants for Actions token permissions.
// Defines scopes and permission levels for Actions tokens. Modified by LAC | Ludwig investing
package actions

// Scope defines the permission scope for Actions tokens.
type Scope string

const (
	ScopeActions          Scope = "actions"
	ScopeChecks          Scope = "checks"
	ScopeContents        Scope = "contents"
	ScopeDeployments     Scope = "deployments"
	ScopeIssues          Scope = "issues"
	ScopePackages        Scope = "packages"
	ScopePullRequests    Scope = "pull-requests"
	ScopeRepositoryProjects Scope = "repository-projects"
	ScopeStatuses        Scope = "statuses"
)

// AllScopes lists all available scopes.
var AllScopes = []Scope{
	ScopeActions, ScopeChecks, ScopeContents, ScopeDeployments,
	ScopeIssues, ScopePackages, ScopePullRequests,
	ScopeRepositoryProjects, ScopeStatuses,
}

// Permission level for a scope.
type Permission int

const (
	PermissionNone  Permission = iota
	PermissionRead
	PermissionWrite
)

// String returns the string representation of the permission.
func (p Permission) String() string {
	switch p {
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	default:
		return "none"
	}
}

// PermissionFromString converts a string to Permission.
func PermissionFromString(s string) Permission {
	switch s {
	case "read":
		return PermissionRead
	case "write":
		return PermissionWrite
	default:
		return PermissionNone
	}
}
