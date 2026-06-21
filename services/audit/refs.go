// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"

	audit_model "gitea.dev/models/audit"
	organization_model "gitea.dev/models/organization"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// EntityRef is a denormalized reference persisted at record time.
// The audit core only understands scope types; callers supply names and metadata.
type EntityRef struct {
	Type audit_model.ScopeType `json:"type"`
	ID   int64                 `json:"id,omitempty"`
	Name string                `json:"name,omitempty"`
}

func ActorFromUser(u *user_model.User) EntityRef {
	if u == nil {
		return EntityRef{}
	}
	return EntityRef{Type: audit_model.ScopeUser, ID: u.ID, Name: u.Name}
}

func ScopeFromUser(u *user_model.User) EntityRef {
	if u == nil {
		return EntityRef{}
	}
	if u.IsOrganization() {
		return EntityRef{Type: audit_model.ScopeOrganization, ID: u.ID, Name: u.Name}
	}
	return EntityRef{Type: audit_model.ScopeUser, ID: u.ID, Name: u.Name}
}

func ScopeFromOrganization(org *organization_model.Organization) EntityRef {
	if org == nil {
		return EntityRef{}
	}
	return EntityRef{Type: audit_model.ScopeOrganization, ID: org.ID, Name: org.Name}
}

func ScopeFromRepository(repo *repository_model.Repository) EntityRef {
	if repo == nil {
		return EntityRef{}
	}
	return EntityRef{Type: audit_model.ScopeRepository, ID: repo.ID, Name: repo.FullName()}
}

func ScopeSystem() EntityRef {
	return EntityRef{Type: audit_model.ScopeSystem, Name: "System"}
}

// scopeRef derives an EntityRef from the affected entity passed to Record.
// Supported types: *user_model.User, *organization_model.Organization,
// *repository_model.Repository, EntityRef, or nil for an instance-wide event.
func scopeRef(scope any) EntityRef {
	switch s := scope.(type) {
	case nil:
		return ScopeSystem()
	case EntityRef:
		return s
	case *user_model.User:
		return ScopeFromUser(s)
	case *organization_model.Organization:
		return ScopeFromOrganization(s)
	case *repository_model.Repository:
		return ScopeFromRepository(s)
	default:
		panic(fmt.Sprintf("audit: unsupported scope type %T", scope))
	}
}
