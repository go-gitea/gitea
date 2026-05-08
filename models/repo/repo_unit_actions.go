// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"slices"

	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
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

func (ActionsTokenPermissionMode) EnumValues() []ActionsTokenPermissionMode {
	return []ActionsTokenPermissionMode{ActionsTokenPermissionModePermissive /* default */, ActionsTokenPermissionModeRestricted}
}

// ActionsTokenPermissions defines the permissions for different repository units
type ActionsTokenPermissions struct {
	UnitAccessModes map[unit.Type]perm.AccessMode `json:"unit_access_modes,omitempty"`
}

var ActionsTokenUnitTypes = []unit.Type{
	unit.TypeCode,
	unit.TypeIssues,
	unit.TypePullRequests,
	unit.TypePackages,
	unit.TypeActions,
	unit.TypeWiki,
	unit.TypeReleases,
	unit.TypeProjects,
}

func MakeActionsTokenPermissions(unitAccessMode perm.AccessMode) (ret ActionsTokenPermissions) {
	ret.UnitAccessModes = make(map[unit.Type]perm.AccessMode)
	for _, u := range ActionsTokenUnitTypes {
		ret.UnitAccessModes[u] = unitAccessMode
	}
	return ret
}

// ClampActionsTokenPermissions ensures that the given permissions don't exceed the maximum
func ClampActionsTokenPermissions(p1, p2 ActionsTokenPermissions) (ret ActionsTokenPermissions) {
	ret.UnitAccessModes = make(map[unit.Type]perm.AccessMode)
	for _, ut := range ActionsTokenUnitTypes {
		ret.UnitAccessModes[ut] = min(p1.UnitAccessModes[ut], p2.UnitAccessModes[ut])
	}
	return ret
}

// MakeRestrictedPermissions returns the restricted permissions
func MakeRestrictedPermissions() ActionsTokenPermissions {
	ret := MakeActionsTokenPermissions(perm.AccessModeNone)
	ret.UnitAccessModes[unit.TypeCode] = perm.AccessModeRead
	ret.UnitAccessModes[unit.TypePackages] = perm.AccessModeRead
	ret.UnitAccessModes[unit.TypeReleases] = perm.AccessModeRead
	return ret
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
	switch cfg.TokenPermissionMode {
	case ActionsTokenPermissionModeRestricted:
		return MakeRestrictedPermissions()
	case ActionsTokenPermissionModePermissive:
		return MakeActionsTokenPermissions(perm.AccessModeWrite)
	default:
		return ActionsTokenPermissions{}
	}
}

// GetMaxTokenPermissions returns the maximum allowed permissions
func (cfg *ActionsConfig) GetMaxTokenPermissions() ActionsTokenPermissions {
	if cfg.MaxTokenPermissions != nil {
		return *cfg.MaxTokenPermissions
	}
	// Default max is write for everything
	return MakeActionsTokenPermissions(perm.AccessModeWrite)
}

// ClampPermissions ensures that the given permissions don't exceed the maximum
func (cfg *ActionsConfig) ClampPermissions(perms ActionsTokenPermissions) ActionsTokenPermissions {
	maxPerms := cfg.GetMaxTokenPermissions()
	return ClampActionsTokenPermissions(perms, maxPerms)
}

// FromDB fills up a ActionsConfig from serialized format.
func (cfg *ActionsConfig) FromDB(bs []byte) error {
	_ = json.UnmarshalHandleDoubleEncode(bs, &cfg)
	cfg.TokenPermissionMode, _ = util.EnumValue(cfg.TokenPermissionMode)
	return nil
}

// ToDB exports a ActionsConfig to a serialized format.
func (cfg *ActionsConfig) ToDB() ([]byte, error) {
	return json.Marshal(cfg)
}
