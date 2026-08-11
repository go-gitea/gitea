// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	audit_model "gitea.dev/models/audit"
	organization_model "gitea.dev/models/organization"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
)

func actorFromUser(u *user_model.User) audit_model.EntityRef {
	if u == nil {
		return audit_model.EntityRef{}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeUser, ID: u.ID, Name: u.Name}
}

// actorRef builds the actor reference of an event. An unresolvable actor means
// an entry point neither runs inside an authenticated request nor called
// WithDoer; record the event with an "Unknown" actor rather than dropping it,
// and log so the missing entry point is visible.
func actorRef(doer *user_model.User) audit_model.EntityRef {
	if doer == nil {
		log.Error("audit: no actor in context, recording event as unknown actor")
		return audit_model.EntityRef{Type: audit_model.ScopeUser, Name: "Unknown"}
	}
	return actorFromUser(doer)
}

func ScopeFromUser(u *user_model.User) audit_model.EntityRef {
	if u == nil {
		return audit_model.EntityRef{}
	}
	if u.IsOrganization() {
		return audit_model.EntityRef{Type: audit_model.ScopeOrganization, ID: u.ID, Name: u.Name}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeUser, ID: u.ID, Name: u.Name}
}

func ScopeFromOrganization(org *organization_model.Organization) audit_model.EntityRef {
	if org == nil {
		return audit_model.EntityRef{}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeOrganization, ID: org.ID, Name: org.Name}
}

func ScopeFromRepository(repo *repository_model.Repository) audit_model.EntityRef {
	if repo == nil {
		return audit_model.EntityRef{}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: repo.ID, Name: repo.FullName()}
}

func ScopeSystem() audit_model.EntityRef {
	return audit_model.EntityRef{Type: audit_model.ScopeSystem, Name: "System"}
}

// scopeRef derives an EntityRef from the affected entity passed to Record.
// Supported types: *user_model.User, *organization_model.Organization,
// *repository_model.Repository, EntityRef, or nil for an instance-wide event.
func scopeRef(scope any) audit_model.EntityRef {
	switch s := scope.(type) {
	case nil:
		return ScopeSystem()
	case audit_model.EntityRef:
		return s
	case *user_model.User:
		return ScopeFromUser(s)
	case *organization_model.Organization:
		return ScopeFromOrganization(s)
	case *repository_model.Repository:
		return ScopeFromRepository(s)
	default:
		// Audit recording must never crash the request that triggered it; record
		// a system-scoped event instead of panicking on an unexpected type.
		log.Error("audit: unsupported scope type %T; recording as system scope", scope)
		return ScopeSystem()
	}
}
