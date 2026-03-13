// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"slices"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

// ActionsTokenPermissionMode defines the default permission mode for Actions tokens
type ActionsTokenPermissionMode string

const (
	// ActionsTokenPermissionModePermissive - write access by default (current behavior, backwards compatible)
	ActionsTokenPermissionModePermissive ActionsTokenPermissionMode = "permissive"
	// ActionsTokenPermissionModeRestricted - read access by default
	ActionsTokenPermissionModeRestricted ActionsTokenPermissionMode = "restricted"
)

// ActionsCrossRepoMode defines the mode for cross-repository access
type ActionsCrossRepoMode string

const (
	// ActionsCrossRepoModeNone - no cross-repository access allowed
	ActionsCrossRepoModeNone ActionsCrossRepoMode = "none"
	// ActionsCrossRepoModeSelected - access allowed only to selected repositories
	ActionsCrossRepoModeSelected ActionsCrossRepoMode = "selected"
)

// ActionsTokenPermissions defines the permissions for different repository units
type ActionsTokenPermissions struct {
	// Code (repository code) - read/write/none
	Code perm.AccessMode `json:"code"`
	// Issues - read/write/none
	Issues perm.AccessMode `json:"issues"`
	// PullRequests - read/write/none
	PullRequests perm.AccessMode `json:"pull_requests"`
	// Packages - read/write/none
	Packages perm.AccessMode `json:"packages"`
	// Actions - read/write/none
	Actions perm.AccessMode `json:"actions"`
	// Wiki - read/write/none
	Wiki perm.AccessMode `json:"wiki"`
	// Releases - read/write/none
	Releases perm.AccessMode `json:"releases"`
	// Projects - read/write/none
	Projects perm.AccessMode `json:"projects"`
}

// HasAccess checks if the permission meets the required access level for the given scope
func (p ActionsTokenPermissions) HasAccess(scope string, required perm.AccessMode) bool {
	var mode perm.AccessMode
	switch scope {
	case "actions":
		mode = p.Actions
	case "contents":
		mode = min(p.Code, p.Releases)
	case "code":
		mode = p.Code
	case "issues":
		mode = p.Issues
	case "packages":
		mode = p.Packages
	case "pull_requests":
		mode = p.PullRequests
	case "wiki":
		mode = p.Wiki
	case "releases":
		mode = p.Releases
	case "projects":
		mode = p.Projects
	}
	return mode >= required
}

// HasRead checks if the permission has read access for the given scope (convenience wrapper for templates)
func (p ActionsTokenPermissions) HasRead(scope string) bool {
	return p.HasAccess(scope, perm.AccessModeRead)
}

// HasWrite checks if the permission has write access for the given scope (convenience wrapper for templates)
func (p ActionsTokenPermissions) HasWrite(scope string) bool {
	return p.HasAccess(scope, perm.AccessModeWrite)
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (p ActionsTokenPermissions) ClampPermissions(maxPerms ActionsTokenPermissions) ActionsTokenPermissions {
	return ActionsTokenPermissions{
		Code:         min(p.Code, maxPerms.Code),
		Issues:       min(p.Issues, maxPerms.Issues),
		PullRequests: min(p.PullRequests, maxPerms.PullRequests),
		Packages:     min(p.Packages, maxPerms.Packages),
		Actions:      min(p.Actions, maxPerms.Actions),
		Wiki:         min(p.Wiki, maxPerms.Wiki),
		Releases:     min(p.Releases, maxPerms.Releases),
		Projects:     min(p.Projects, maxPerms.Projects),
	}
}

// GetRestrictedPermissions returns the restricted permissions
func GetRestrictedPermissions() ActionsTokenPermissions {
	return ActionsTokenPermissions{
		Code:     perm.AccessModeRead,
		Packages: perm.AccessModeRead,
		Releases: perm.AccessModeRead,
	}
}

// MarshalTokenPermissions serializes ActionsTokenPermissions to JSON
func MarshalTokenPermissions(perms ActionsTokenPermissions) (string, error) {
	data, err := json.Marshal(perms)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalTokenPermissions deserializes JSON to ActionsTokenPermissions
func UnmarshalTokenPermissions(data string) (ActionsTokenPermissions, error) {
	var perms ActionsTokenPermissions
	if data == "" {
		return perms, nil
	}
	err := json.Unmarshal([]byte(data), &perms)
	return perms, err
}

type ActionsConfig struct {
	DisabledWorkflows []string
	// CollaborativeOwnerIDs is a list of owner IDs used to share actions from private repos.
	// Only workflows from the private repos whose owners are in CollaborativeOwnerIDs can access the current repo's actions.
	CollaborativeOwnerIDs []int64
	// TokenPermissionMode defines the default permission mode (permissive, restricted, or custom)
	TokenPermissionMode ActionsTokenPermissionMode `json:"token_permission_mode,omitempty"`
	// MaxTokenPermissions defines the absolute maximum permissions any token can have in this context.
	// Workflow YAML "permissions" keywords can reduce permissions but never exceed this ceiling.
	MaxTokenPermissions *ActionsTokenPermissions `json:"max_token_permissions,omitempty"`
	// CrossRepoMode indicates which repos in the org can be accessed (none, all, or selected)
	CrossRepoMode ActionsCrossRepoMode `json:"cross_repo_mode,omitempty"`
	// AllowedCrossRepoIDs is a list of specific repo IDs that can be accessed cross-repo (only used if CrossRepoMode is ActionsCrossRepoModeSelected)
	AllowedCrossRepoIDs []int64 `json:"allowed_cross_repo_ids,omitempty"`
	// OverrideOwnerConfig indicates if this repository should override the owner-level configuration (User or Org)
	OverrideOwnerConfig bool `json:"override_owner_config,omitempty"`
}

func (cfg *ActionsConfig) EnableWorkflow(file string) {
	cfg.DisabledWorkflows = util.SliceRemoveAll(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) IsWorkflowDisabled(file string) bool {
	return slices.Contains(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) DisableWorkflow(file string) {
	if slices.Contains(cfg.DisabledWorkflows, file) {
		return
	}

	cfg.DisabledWorkflows = append(cfg.DisabledWorkflows, file)
}

func (cfg *ActionsConfig) AddCollaborativeOwner(ownerID int64) {
	if !slices.Contains(cfg.CollaborativeOwnerIDs, ownerID) {
		cfg.CollaborativeOwnerIDs = append(cfg.CollaborativeOwnerIDs, ownerID)
	}
}

func (cfg *ActionsConfig) RemoveCollaborativeOwner(ownerID int64) {
	cfg.CollaborativeOwnerIDs = util.SliceRemoveAll(cfg.CollaborativeOwnerIDs, ownerID)
}

func (cfg *ActionsConfig) IsCollaborativeOwner(ownerID int64) bool {
	return slices.Contains(cfg.CollaborativeOwnerIDs, ownerID)
}

// GetDefaultTokenPermissions returns the default token permissions by its TokenPermissionMode.
// It does not apply MaxTokenPermissions; callers must clamp if needed.
func (cfg *ActionsConfig) GetDefaultTokenPermissions() ActionsTokenPermissions {
	mode := cfg.TokenPermissionMode
	if mode == "" {
		mode = ActionsTokenPermissionModePermissive
	}
	switch mode {
	case ActionsTokenPermissionModeRestricted:
		return GetRestrictedPermissions()
	case ActionsTokenPermissionModePermissive:
		return ActionsTokenPermissions{
			Code:         perm.AccessModeWrite,
			Issues:       perm.AccessModeWrite,
			PullRequests: perm.AccessModeWrite,
			Packages:     perm.AccessModeWrite,
			Actions:      perm.AccessModeWrite,
			Wiki:         perm.AccessModeWrite,
			Releases:     perm.AccessModeWrite,
			Projects:     perm.AccessModeWrite,
		}
	default:
		return ActionsTokenPermissions{
			Code:         perm.AccessModeNone,
			Issues:       perm.AccessModeNone,
			PullRequests: perm.AccessModeNone,
			Packages:     perm.AccessModeNone,
			Actions:      perm.AccessModeNone,
			Wiki:         perm.AccessModeNone,
			Releases:     perm.AccessModeNone,
			Projects:     perm.AccessModeNone,
		}
	}
}

// GetMaxTokenPermissions returns the maximum allowed permissions
func (cfg *ActionsConfig) GetMaxTokenPermissions() ActionsTokenPermissions {
	if cfg.MaxTokenPermissions != nil {
		return *cfg.MaxTokenPermissions
	}
	// Default max is write for everything
	return ActionsTokenPermissions{
		Code:         perm.AccessModeWrite,
		Issues:       perm.AccessModeWrite,
		PullRequests: perm.AccessModeWrite,
		Packages:     perm.AccessModeWrite,
		Actions:      perm.AccessModeWrite,
		Wiki:         perm.AccessModeWrite,
		Releases:     perm.AccessModeWrite,
		Projects:     perm.AccessModeWrite,
	}
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (cfg *ActionsConfig) ClampPermissions(perms ActionsTokenPermissions) ActionsTokenPermissions {
	maxPerms := cfg.GetMaxTokenPermissions()
	return perms.ClampPermissions(maxPerms)
}

// FromDB fills up a ActionsConfig from serialized format.
func (cfg *ActionsConfig) FromDB(bs []byte) error {
	if err := json.UnmarshalHandleDoubleEncode(bs, &cfg); err != nil {
		return err
	}

	switch cfg.TokenPermissionMode {
	case ActionsTokenPermissionModeRestricted, ActionsTokenPermissionModePermissive:
	default:
		cfg.TokenPermissionMode = ActionsTokenPermissionModePermissive
	}

	switch cfg.CrossRepoMode {
	case ActionsCrossRepoModeSelected:
	default:
		cfg.CrossRepoMode = ActionsCrossRepoModeNone
	}

	return nil
}

// ToDB exports a ActionsConfig to a serialized format.
func (cfg *ActionsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}
