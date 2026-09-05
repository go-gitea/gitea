// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/test"
	"gitea.dev/modules/web/middleware"

	"github.com/stretchr/testify/assert"
)

func BenchmarkRecordDisabled(b *testing.B) {
	defer test.MockVariableValue(&setting.Audit.RecordOutput, setting.AuditRecordOutputDisabled)()
	ctx := context.Background()
	u := &user_model.User{ID: 1, Name: "user"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordAs(ctx, u, audit_model.UserPassword, u)
	}
}

func BenchmarkBuildEvent(b *testing.B) {
	params := RecordParams{
		Action: audit_model.RepositoryMirrorPushAdd,
		Actor:  audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "actor"},
		Scope:  audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: 2, Name: "owner/repo"},
		Metadata: map[string]any{
			"remote_address": "https://example.com/repo.git",
		},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = buildEvent(context.Background(), params)
	}
}

// newRequestContext mimics what routers/common.AuthShared publishes for a
// signed-in request.
func newRequestContext(t *testing.T, signedIn *user_model.User) context.Context {
	t.Helper()
	rc := reqctx.NewRequestContextForTest(t)
	rc.GetData()[middleware.ContextDataKeySignedUser] = signedIn
	return rc
}

func TestBuildEvent(t *testing.T) {
	doer := &user_model.User{ID: 2, Name: "Doer"}
	u := &user_model.User{ID: 1, Name: "TestUser"}

	t.Run("MessageFromTemplate", func(t *testing.T) {
		e := buildEvent(context.Background(), RecordParams{
			Action: audit_model.UserCreate,
			Actor:  actorRef(doer),
			Scope:  ScopeFromUser(u),
		})

		assert.Equal(t, audit_model.UserCreate, e.Action)
		assert.Equal(t, audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 2, Name: "Doer"}, e.Actor())
		assert.Equal(t, audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}, e.Scope())
		assert.Equal(t, "Created user TestUser.", e.Message)
	})

	t.Run("MetadataFillsPlaceholders", func(t *testing.T) {
		r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
		m := &repository_model.PushMirror{ID: 4, RemoteAddress: "git@example.com:repo.git"}

		e := buildEvent(context.Background(), RecordParams{
			Action: audit_model.RepositoryMirrorPushAdd,
			Actor:  actorRef(doer),
			Scope:  ScopeFromRepository(r),
			Metadata: metaPairs(
				"mirror_id", m.ID,
				"remote_address", m.RemoteAddress,
			),
		})

		assert.Equal(t, "TestUser/TestRepo", e.ScopeName)
		assert.Equal(t, "Added push mirror to git@example.com:repo.git for repository TestUser/TestRepo.", e.Message)
		assert.InDelta(t, float64(m.ID), audit_model.DecodeMetadata(e.Metadata)["mirror_id"], 0)
	})

	t.Run("StatusChangesIncludeTheirNewValue", func(t *testing.T) {
		e := buildEvent(context.Background(), RecordParams{
			Action:   audit_model.UserRestricted,
			Actor:    actorRef(doer),
			Scope:    ScopeFromUser(u),
			Metadata: metaPairs("restricted", true),
		})

		assert.Equal(t, "Changed restricted status of user TestUser to true.", e.Message)
	})

	t.Run("SystemActorNamesTaskOrKey", func(t *testing.T) {
		actions := user_model.NewActionsUserWithTaskID(42)
		e := buildEvent(context.Background(), RecordParams{
			Action:          audit_model.UserCreate,
			Actor:           actorRef(actions),
			ActorCredential: actorCredential(context.Background(), actions),
			Scope:           ScopeFromUser(u),
		})
		assert.Equal(t, user_model.ActionsUserID, e.ActorID)
		assert.Equal(t, "gitea-actions:42", e.ActorCredential)

		key := user_model.NewDeployKeyUserWithKeyID(7)
		assert.Equal(t, "deploy-key:7", actorCredential(context.Background(), key))
		assert.Empty(t, actorCredential(context.Background(), doer))
	})

	t.Run("CredentialFromRequest", func(t *testing.T) {
		ctx := newRequestContext(t, doer)
		middleware.GetContextData(ctx)[middleware.ContextDataKeyAuthCredential] = "access-token:9"
		assert.Equal(t, "access-token:9", actorCredential(ctx, doer))

		// an event recorded for someone other than the signed-in user is not
		// tied to the credential of that request
		assert.Empty(t, actorCredential(ctx, u))
	})

	t.Run("IPAddressFromRequest", func(t *testing.T) {
		params := RecordParams{Action: audit_model.UserCreate, Actor: actorRef(doer), Scope: ScopeFromUser(u)}

		assert.Empty(t, buildEvent(context.Background(), params).IPAddress)

		ctx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})
		assert.Equal(t, "127.0.0.1", buildEvent(ctx, params).IPAddress)
	})

	t.Run("OriginFromRequest", func(t *testing.T) {
		defer test.MockVariableValue(&setting.AppSubURL, "/gitea")()
		params := RecordParams{Action: audit_model.UserCreate, Actor: actorRef(doer), Scope: ScopeFromUser(u)}

		assert.Equal(t, audit_model.OriginSystem, buildEvent(context.Background(), params).Origin)

		cliCtx := WithOrigin(context.Background(), audit_model.OriginCLI)
		assert.Equal(t, audit_model.OriginCLI, buildEvent(cliCtx, params).Origin)

		uiCtx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{URL: &url.URL{Path: "/gitea/user/settings"}})
		assert.Equal(t, audit_model.OriginUI, buildEvent(uiCtx, params).Origin)

		apiCtx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{URL: &url.URL{Path: "/gitea/api/v1/user"}})
		assert.Equal(t, audit_model.OriginAPI, buildEvent(apiCtx, params).Origin)

		systemAPIContext := WithOrigin(apiCtx, audit_model.OriginSystem)
		assert.Equal(t, audit_model.OriginSystem, buildEvent(systemAPIContext, params).Origin)
	})
}

func TestEntityRefDisplay(t *testing.T) {
	ref := audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}
	assert.Equal(t, "TestUser", ref.DisplayName())
	assert.Equal(t, "/TestUser", ref.HomeLink())
	assert.True(t, ref.HasLink())

	sys := ScopeSystem()
	assert.Equal(t, "System", sys.DisplayName())
	assert.Empty(t, sys.HomeLink())
	assert.False(t, sys.HasLink())

	// a scope whose entity was deleted keeps its ID but has no name to link to
	deleted := audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: 3}
	assert.Empty(t, deleted.DisplayName())
	assert.False(t, deleted.HasLink())

	repo := audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: 3, Name: "Test User/Test Repo"}
	assert.Equal(t, "/Test%20User/Test%20Repo", repo.HomeLink())
	assert.True(t, repo.HasLink())
}

func TestEncodeDecodeMetadata(t *testing.T) {
	raw := audit_model.EncodeMetadata(metaPairs("repo_id", int64(42), "repo", "o/r"))
	decoded := audit_model.DecodeMetadata(raw)
	assert.InDelta(t, 42.0, decoded["repo_id"], 0) // json numbers decode as float64
	assert.Equal(t, "o/r", decoded["repo"])
}

func TestDoerFromContext(t *testing.T) {
	doer := &user_model.User{ID: 2, Name: "Doer"}
	signedIn := &user_model.User{ID: 3, Name: "SignedIn"}

	t.Run("NoActor", func(t *testing.T) {
		assert.Nil(t, doerFromContext(context.Background()))
	})

	t.Run("WithDoer", func(t *testing.T) {
		assert.Equal(t, doer, doerFromContext(WithDoer(context.Background(), doer)))
	})

	t.Run("SignedInUserOfRequest", func(t *testing.T) {
		ctx := newRequestContext(t, signedIn)
		assert.Equal(t, signedIn, doerFromContext(ctx))
	})

	t.Run("WithDoerWinsOverSignedInUser", func(t *testing.T) {
		ctx := WithDoer(newRequestContext(t, signedIn), doer)
		assert.Equal(t, doer, doerFromContext(ctx))
	})
}

// An impersonated session must never pin an event on the impersonated user
// alone, otherwise an admin could act in someone else's name untraceably.
func TestImpersonatorRef(t *testing.T) {
	admin := &user_model.User{ID: 1, Name: "Admin"}
	impersonated := &user_model.User{ID: 2, Name: "Impersonated"}

	rc := reqctx.NewRequestContextForTest(t)
	rc.GetData()[middleware.ContextDataKeySignedUser] = impersonated
	rc.GetData()[middleware.ContextDataKeyImpersonator] = admin

	assert.Equal(t, admin, ImpersonatorFromContext(rc))

	e := buildEvent(rc, RecordParams{
		Action:       audit_model.UserPassword,
		Actor:        actorRef(doerFromContext(rc)),
		Impersonator: impersonatorRef(ImpersonatorFromContext(rc), doerFromContext(rc)),
		Scope:        ScopeFromUser(impersonated),
	})
	assert.Equal(t, int64(2), e.ActorID)
	assert.Equal(t, &audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "Admin"}, e.Impersonator())

	// an admin acting as themselves is not an impersonation
	assert.Nil(t, impersonatorRef(admin, admin))
	assert.Nil(t, impersonatorRef(nil, impersonated))
}

// An unresolvable actor must still produce an event, so a missing entry point
// never silently drops security relevant records.
func TestActorRefWithoutDoer(t *testing.T) {
	ref := actorRef(nil)
	assert.Equal(t, "Unknown", ref.DisplayName())
	assert.False(t, ref.HasLink())
}

func TestRenderMessage(t *testing.T) {
	actor := audit_model.EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "Actor"}
	scope := audit_model.EntityRef{Type: audit_model.ScopeRepository, ID: 2, Name: "owner/repo"}

	t.Run("EveryActionHasATemplate", func(t *testing.T) {
		for _, action := range audit_model.AllActions() {
			tmpl, ok := audit_model.MessageTemplate(action)
			assert.True(t, ok, "action %q has no message template", action)
			assert.NotEmpty(t, tmpl, "action %q has empty message template", action)
		}
	})

	t.Run("ReservedPlaceholders", func(t *testing.T) {
		assert.Equal(t,
			"User Actor started impersonating user owner/repo.",
			renderMessage(audit_model.UserImpersonation, actor, scope, nil),
		)
	})

	t.Run("NonStringMetadata", func(t *testing.T) {
		assert.Equal(t,
			"Removed external login from authentication source 7 for user owner/repo.",
			renderMessage(audit_model.UserExternalLoginRemove, actor, scope, map[string]any{"auth_source_id": int64(7)}),
		)
	})

	t.Run("MissingMetadataKeepsTheKey", func(t *testing.T) {
		assert.Equal(t,
			"Added deploy key deploy_key for repository owner/repo.",
			renderMessage(audit_model.RepositoryDeployKeyAdd, actor, scope, nil),
		)
	})

	t.Run("UnknownActionFallsBackToItsName", func(t *testing.T) {
		assert.Equal(t, "not:an:action", renderMessage(audit_model.Action("not:an:action"), actor, scope, nil))
	})
}

func TestActionFilters(t *testing.T) {
	assert.True(t, audit_model.IsActionFilter("user:impersonation"))
	assert.True(t, audit_model.IsActionFilter(audit_model.UserImpersonation))
	assert.False(t, audit_model.IsActionFilter("user:unknown"))
}
