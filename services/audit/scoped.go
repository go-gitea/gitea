// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
)

// ScopedActions holds the action variants for a resource that can be owned by a
// repository, organization, user or the instance itself. RecordScoped selects
// the matching one based on the owner/repo passed at the call site.
type ScopedActions struct {
	Repo   audit_model.Action
	Org    audit_model.Action
	User   audit_model.Action
	System audit_model.Action
}

// resolveScope maps an (owner, repo) pair to the scoped action and audit scope.
// The rules cover every multi-scope resource (secrets, OAuth2 apps, webhooks):
// a repo wins when set, a nil owner means the instance, otherwise the owner's
// kind decides.
func resolveScope(actions ScopedActions, owner *user_model.User, repo *repository_model.Repository) (audit_model.Action, EntityRef) {
	switch {
	case repo != nil:
		return actions.Repo, ScopeFromRepository(repo)
	case owner == nil:
		return actions.System, ScopeSystem()
	case owner.IsOrganization():
		return actions.Org, ScopeFromUser(owner)
	default:
		return actions.User, ScopeFromUser(owner)
	}
}

// RecordScoped records an audit event for a resource owned by a repository (repo
// set), organization, user, or the instance (owner nil, repo nil). It picks the
// scoped action and scope; each variant carries its own message template, so the
// wording follows automatically. Metadata is supplied as alternating
// string-key/value pairs, like Record.
func RecordScoped(ctx context.Context, owner *user_model.User, repo *repository_model.Repository, actions ScopedActions, metadata ...any) {
	action, scope := resolveScope(actions, owner, repo)
	writeEvent(ctx, RecordParams{
		Action:   action,
		Actor:    actorRef(doerFromContext(ctx)),
		Scope:    scope,
		Metadata: metaPairs(metadata...),
	})
}
