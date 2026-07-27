// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"testing"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"
	"gitea.dev/modules/reqctx"
	"gitea.dev/modules/web/middleware"

	"github.com/stretchr/testify/assert"
)

// newRequestContext mimics what routers/common.AuthShared publishes for a
// signed-in request.
func newRequestContext(t *testing.T, signedIn *user_model.User) context.Context {
	t.Helper()
	rc := reqctx.NewRequestContextForTest(context.Background())
	rc.GetData()[middleware.ContextDataKeySignedUser] = signedIn
	return rc
}

func TestBuildEvent(t *testing.T) {
	doer := &user_model.User{ID: 2, Name: "Doer"}
	u := &user_model.User{ID: 1, Name: "TestUser"}

	t.Run("MessageFromTemplate", func(t *testing.T) {
		e := buildEvent(context.Background(), RecordParams{
			Action: audit_model.UserCreate,
			Actor:  ActorFromUser(doer),
			Scope:  ScopeFromUser(u),
		})

		assert.Equal(t, audit_model.UserCreate, e.Action)
		assert.Equal(t, EntityRef{Type: audit_model.ScopeUser, ID: 2, Name: "Doer"}, e.Actor)
		assert.Equal(t, EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}, e.Scope)
		assert.Equal(t, "Created user TestUser.", e.Message)
	})

	t.Run("MetadataFillsPlaceholders", func(t *testing.T) {
		r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
		m := &repository_model.PushMirror{ID: 4, RemoteAddress: "git@example.com:repo.git"}

		e := buildEvent(context.Background(), RecordParams{
			Action: audit_model.RepositoryMirrorPushAdd,
			Actor:  ActorFromUser(doer),
			Scope:  ScopeFromRepository(r),
			Metadata: metaPairs(
				"mirror_id", m.ID,
				"remote_address", m.RemoteAddress,
			),
		})

		assert.Equal(t, "TestUser/TestRepo", e.Scope.Name)
		assert.Equal(t, "Added push mirror to git@example.com:repo.git for repository TestUser/TestRepo.", e.Message)
		assert.Equal(t, m.ID, e.Metadata["mirror_id"])
	})

	t.Run("IPAddressFromRequest", func(t *testing.T) {
		params := RecordParams{Action: audit_model.UserCreate, Actor: ActorFromUser(doer), Scope: ScopeFromUser(u)}

		assert.Empty(t, buildEvent(context.Background(), params).IPAddress)

		ctx := context.WithValue(context.Background(), httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})
		assert.Equal(t, "127.0.0.1", buildEvent(ctx, params).IPAddress)
	})
}

func TestEntityRefDisplay(t *testing.T) {
	ref := EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}
	assert.Equal(t, "TestUser", ref.DisplayName())
	assert.Equal(t, "/TestUser", ref.HomeLink())
	assert.True(t, ref.HasLink())

	sys := ScopeSystem()
	assert.Equal(t, "System", sys.DisplayName())
	assert.Empty(t, sys.HomeLink())
	assert.False(t, sys.HasLink())

	// a scope whose entity was deleted keeps its ID but has no name to link to
	deleted := EntityRef{Type: audit_model.ScopeRepository, ID: 3}
	assert.Empty(t, deleted.DisplayName())
	assert.False(t, deleted.HasLink())

	repo := EntityRef{Type: audit_model.ScopeRepository, ID: 3, Name: "Test User/Test Repo"}
	assert.Equal(t, "/Test%20User/Test%20Repo", repo.HomeLink())
	assert.True(t, repo.HasLink())
}

func TestEncodeDecodeMetadata(t *testing.T) {
	raw := encodeMetadata(metaPairs("repo_id", int64(42), "repo", "o/r"))
	decoded := decodeMetadata(raw)
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

// An unresolvable actor must still produce an event, so a missing entry point
// never silently drops security relevant records.
func TestActorRefWithoutDoer(t *testing.T) {
	ref := actorRef(nil)
	assert.Equal(t, "Unknown", ref.DisplayName())
	assert.False(t, ref.HasLink())
}

func TestRenderMessage(t *testing.T) {
	actor := EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "Actor"}
	scope := EntityRef{Type: audit_model.ScopeRepository, ID: 2, Name: "owner/repo"}

	t.Run("EveryActionHasATemplate", func(t *testing.T) {
		for _, action := range audit_model.AllActions() {
			assert.NotEmpty(t, messages[action], "action %q has no message template", action)
		}
	})

	t.Run("ReservedPlaceholders", func(t *testing.T) {
		assert.Equal(t,
			"User Actor impersonating user owner/repo.",
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
