// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestResolveScope(t *testing.T) {
	actions := ScopedActions{
		Repo:   audit_model.RepositoryWebhookAdd,
		Org:    audit_model.OrganizationWebhookAdd,
		User:   audit_model.UserWebhookAdd,
		System: audit_model.SystemWebhookAdd,
	}

	org := &user_model.User{ID: 10, Name: "MyOrg", Type: user_model.UserTypeOrganization}
	usr := &user_model.User{ID: 11, Name: "MyUser", Type: user_model.UserTypeIndividual}
	repo := &repository_model.Repository{ID: 12, Name: "repo", OwnerName: "MyOrg"}

	t.Run("repo wins over owner", func(t *testing.T) {
		action, scope, label := resolveScope(actions, org, repo)
		assert.Equal(t, audit_model.RepositoryWebhookAdd, action)
		assert.Equal(t, audit_model.ScopeRepository, scope.Type)
		assert.Equal(t, "repository MyOrg/repo", label)
	})

	t.Run("organization owner", func(t *testing.T) {
		action, scope, label := resolveScope(actions, org, nil)
		assert.Equal(t, audit_model.OrganizationWebhookAdd, action)
		assert.Equal(t, audit_model.ScopeOrganization, scope.Type)
		assert.Equal(t, "organization MyOrg", label)
	})

	t.Run("user owner", func(t *testing.T) {
		action, scope, label := resolveScope(actions, usr, nil)
		assert.Equal(t, audit_model.UserWebhookAdd, action)
		assert.Equal(t, audit_model.ScopeUser, scope.Type)
		assert.Equal(t, "user MyUser", label)
	})

	t.Run("system when no owner and no repo", func(t *testing.T) {
		action, scope, label := resolveScope(actions, nil, nil)
		assert.Equal(t, audit_model.SystemWebhookAdd, action)
		assert.Equal(t, audit_model.ScopeSystem, scope.Type)
		assert.Equal(t, "the system", label)
	})
}

// Audit recording must never crash the request that triggered it.
func TestRecordHelpersNeverPanic(t *testing.T) {
	t.Run("metaPairs skips non-string keys", func(t *testing.T) {
		var m map[string]any
		assert.NotPanics(t, func() {
			m = metaPairs("ok", 1, 42 /* bad key */, "value", "second", 2)
		})
		assert.Equal(t, 1, m["ok"])
		assert.Equal(t, 2, m["second"])
		assert.Len(t, m, 2) // the pair with the non-string key is dropped
	})

	t.Run("scopeRef falls back to system on unsupported type", func(t *testing.T) {
		var ref EntityRef
		assert.NotPanics(t, func() {
			ref = scopeRef(struct{ Foo string }{Foo: "bar"})
		})
		assert.Equal(t, audit_model.ScopeSystem, ref.Type)
	})
}
