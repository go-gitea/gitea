// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

// actorRef builds the actor reference of an event. An unresolvable actor means
// an entry point neither runs inside an authenticated request nor called
// WithDoer; record the event with an "Unknown" actor rather than dropping it,
// and log so the missing entry point is visible.
func actorRef(doer *user_model.User) audit_model.EntityRef {
	if doer == nil {
		log.Error("audit: no actor in context, recording event as unknown actor")
		return audit_model.EntityRef{Type: audit_model.ScopeUser, Name: "Unknown"}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeUser, ID: doer.ID, Name: doer.Name}
}

// impersonatorRef names the admin behind an impersonated session. It is dropped
// when the actor is the admin themselves, so events an admin performs before
// entering or after leaving an impersonation are not marked as impersonated.
func impersonatorRef(impersonator, doer *user_model.User) *audit_model.EntityRef {
	if impersonator == nil || (doer != nil && impersonator.ID == doer.ID) {
		return nil
	}
	return &audit_model.EntityRef{Type: audit_model.ScopeUser, ID: impersonator.ID, Name: impersonator.Name}
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

// ScopeFromUserID resolves the scope of a user known only by ID, for call sites
// that would otherwise load the user solely to name it. It costs nothing while
// audit logging is off, and a failed lookup still yields a usable scope so the
// event is never dropped.
func ScopeFromUserID(ctx context.Context, id int64) audit_model.EntityRef {
	ref := audit_model.EntityRef{Type: audit_model.ScopeUser, ID: id}
	if !setting.Audit.Enabled {
		return ref
	}
	u, err := user_model.GetUserByID(ctx, id)
	if err != nil {
		log.Error("audit: GetUserByID(%d): %v", id, err)
		return ref
	}
	return ScopeFromUser(u)
}

func ScopeFromRepository(repo *repository_model.Repository) audit_model.EntityRef {
	if repo == nil {
		return audit_model.EntityRef{}
	}
	return audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: repo.ID, Name: repo.FullName()}
}

func ScopeSystem() audit_model.EntityRef {
	return audit_model.EntityRef{Type: audit_model.ScopeSystem}
}

// scopeRef derives an EntityRef from the affected entity passed to Record.
// Supported types: *user_model.User, *repository_model.Repository, EntityRef,
// or nil for an instance-wide event.
func scopeRef(scope any) audit_model.EntityRef {
	switch s := scope.(type) {
	case nil:
		return ScopeSystem()
	case audit_model.EntityRef:
		return s
	case *user_model.User:
		return ScopeFromUser(s)
	case *repository_model.Repository:
		return ScopeFromRepository(s)
	default:
		// Audit recording must never crash the request that triggered it; record
		// a system-scoped event instead of panicking on an unexpected type.
		log.Error("audit: unsupported scope type %T; recording as system scope", scope)
		return ScopeSystem()
	}
}
