// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/log"
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

var (
	SecretAdd = ScopedActions{
		Repo: audit_model.RepositorySecretAdd,
		Org:  audit_model.OrganizationSecretAdd,
		User: audit_model.UserSecretAdd,
	}
	SecretUpdate = ScopedActions{
		Repo: audit_model.RepositorySecretUpdate,
		Org:  audit_model.OrganizationSecretUpdate,
		User: audit_model.UserSecretUpdate,
	}
	SecretRemove = ScopedActions{
		Repo: audit_model.RepositorySecretRemove,
		Org:  audit_model.OrganizationSecretRemove,
		User: audit_model.UserSecretRemove,
	}

	OAuth2ApplicationAdd = ScopedActions{
		User:   audit_model.UserOAuth2ApplicationAdd,
		Org:    audit_model.OrganizationOAuth2ApplicationAdd,
		System: audit_model.SystemOAuth2ApplicationAdd,
	}
	OAuth2ApplicationUpdate = ScopedActions{
		User:   audit_model.UserOAuth2ApplicationUpdate,
		Org:    audit_model.OrganizationOAuth2ApplicationUpdate,
		System: audit_model.SystemOAuth2ApplicationUpdate,
	}
	OAuth2ApplicationSecret = ScopedActions{
		User:   audit_model.UserOAuth2ApplicationSecret,
		Org:    audit_model.OrganizationOAuth2ApplicationSecret,
		System: audit_model.SystemOAuth2ApplicationSecret,
	}
	OAuth2ApplicationRemove = ScopedActions{
		User:   audit_model.UserOAuth2ApplicationRemove,
		Org:    audit_model.OrganizationOAuth2ApplicationRemove,
		System: audit_model.SystemOAuth2ApplicationRemove,
	}
	OAuth2ApplicationRevoke = ScopedActions{
		User: audit_model.UserOAuth2ApplicationRevoke,
	}

	WebhookAdd = ScopedActions{
		Repo:   audit_model.RepositoryWebhookAdd,
		Org:    audit_model.OrganizationWebhookAdd,
		User:   audit_model.UserWebhookAdd,
		System: audit_model.SystemWebhookAdd,
	}
	WebhookUpdate = ScopedActions{
		Repo:   audit_model.RepositoryWebhookUpdate,
		Org:    audit_model.OrganizationWebhookUpdate,
		User:   audit_model.UserWebhookUpdate,
		System: audit_model.SystemWebhookUpdate,
	}
	WebhookRemove = ScopedActions{
		Repo:   audit_model.RepositoryWebhookRemove,
		Org:    audit_model.OrganizationWebhookRemove,
		User:   audit_model.UserWebhookRemove,
		System: audit_model.SystemWebhookRemove,
	}
)

// resolveScope maps an (owner, repo) pair to the scoped action and audit scope.
// The rules cover every multi-scope resource (secrets, OAuth2 apps, webhooks):
// a repo wins when set, a nil owner means the instance, otherwise the owner's
// kind decides.
func resolveScope(actions ScopedActions, owner *user_model.User, repo *repository_model.Repository) (audit_model.Action, audit_model.EntityRef) {
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
	if action == "" {
		log.Error("audit: no action configured for scope type %s", scope.Type)
		return
	}
	doer := doerFromContext(ctx)
	writeEvent(ctx, RecordParams{
		Action:          action,
		Actor:           actorRef(doer),
		ActorCredential: actorCredential(ctx, doer),
		Scope:           scope,
		Metadata:        metaPairs(metadata...),
	})
}
