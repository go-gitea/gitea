// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/modules/json"
)

// TokenPermissionLevel represents the access level for a specific scope
type TokenPermissionLevel string

const (
	TokenPermissionNone  TokenPermissionLevel = "none"
	TokenPermissionRead  TokenPermissionLevel = "read"
	TokenPermissionWrite TokenPermissionLevel = "write"
)

// TokenPermissions represents the permissions configured for an Actions job token.
// The keys are GitHub-compatible scope names (e.g., "contents", "issues", "pull-requests", "packages").
// The values are TokenPermissionLevel (none, read, write).
type TokenPermissions map[string]TokenPermissionLevel

// TokenPermissionsJSON is used for database storage
type TokenPermissionsJSON struct {
	data []byte
}

// FromDB implements convert.Conversion for xorm
func (t *TokenPermissionsJSON) FromDB(bs []byte) error {
	t.data = bs
	return nil
}

// ToDB implements convert.Conversion for xorm
func (t *TokenPermissionsJSON) ToDB() ([]byte, error) {
	return t.data, nil
}

// ScopeToUnitType maps a GitHub Actions permission scope name to a Gitea unit type.
// "contents" maps to TypeCode (read/write code, releases, etc.)
// "issues" maps to TypeIssues
// "pull-requests" maps to TypePullRequests
// "packages" maps to TypePackages
// "actions" maps to TypeActions
func ScopeToUnitType(scope string) (unit.Type, bool) {
	switch scope {
	case "contents":
		return unit.TypeCode, true
	case "issues":
		return unit.TypeIssues, true
	case "pull-requests":
		return unit.TypePullRequests, true
	case "packages":
		return unit.TypePackages, true
	case "actions":
		return unit.TypeActions, true
	default:
		return unit.TypeInvalid, false
	}
}

// UnitTypeToScope maps a Gitea unit type back to a GitHub Actions permission scope name.
func UnitTypeToScope(ut unit.Type) (string, bool) {
	switch ut {
	case unit.TypeCode:
		return "contents", true
	case unit.TypeIssues:
		return "issues", true
	case unit.TypePullRequests:
		return "pull-requests", true
	case unit.TypePackages:
		return "packages", true
	case unit.TypeActions:
		return "actions", true
	default:
		return "", false
	}
}

// PermissionLevelToAccessMode converts a TokenPermissionLevel to a perm.AccessMode
func PermissionLevelToAccessMode(level TokenPermissionLevel) perm.AccessMode {
	switch level {
	case TokenPermissionWrite:
		return perm.AccessModeWrite
	case TokenPermissionRead:
		return perm.AccessModeRead
	default:
		return perm.AccessModeNone
	}
}

// AccessModeToPermissionLevel converts a perm.AccessMode to a TokenPermissionLevel
func AccessModeToPermissionLevel(mode perm.AccessMode) TokenPermissionLevel {
	switch {
	case mode >= perm.AccessModeWrite:
		return TokenPermissionWrite
	case mode >= perm.AccessModeRead:
		return TokenPermissionRead
	default:
		return TokenPermissionNone
	}
}

// MarshalTokenPermissions serializes TokenPermissions to JSON bytes
func MarshalTokenPermissions(perms TokenPermissions) (string, error) {
	if perms == nil {
		return "", nil
	}
	bs, err := json.Marshal(perms)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// UnmarshalTokenPermissions deserializes TokenPermissions from a JSON string
func UnmarshalTokenPermissions(data string) (TokenPermissions, error) {
	if data == "" {
		return nil, nil //nolint:nilnil
	}
	var perms TokenPermissions
	if err := json.Unmarshal([]byte(data), &perms); err != nil {
		return nil, err
	}
	return perms, nil
}
