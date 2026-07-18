// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"testing"
	"time"

	audit_model "gitea.dev/models/audit"
	repository_model "gitea.dev/models/repo"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/httplib"

	"github.com/stretchr/testify/assert"
)

func TestBuildEvent(t *testing.T) {
	ctx := context.Background()
	doer := &user_model.User{ID: 2, Name: "Doer"}
	u := &user_model.User{ID: 1, Name: "TestUser"}

	e := buildEvent(ctx, RecordParams{
		Action:  audit_model.UserCreate,
		Actor:   ActorFromUser(doer),
		Scope:   ScopeFromUser(u),
		Message: "Created user TestUser.",
		Metadata: metaPairs(
			"user_id", int64(1),
			"user_name", "TestUser",
		),
	})
	e.Time = time.Time{}

	assert.Equal(t, audit_model.UserCreate, e.Action)
	assert.Equal(t, EntityRef{Type: audit_model.ScopeUser, ID: 2, Name: "Doer"}, e.Actor)
	assert.Equal(t, EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}, e.Scope)
	assert.Equal(t, "Created user TestUser.", e.Message)
	assert.Equal(t, "TestUser", e.Metadata["user_name"])

	r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
	m := &repository_model.PushMirror{ID: 4, RemoteAddress: "git@example.com:repo.git"}

	e = buildEvent(ctx, RecordParams{
		Action:  audit_model.RepositoryMirrorPushAdd,
		Actor:   ActorFromUser(doer),
		Scope:   ScopeFromRepository(r),
		Message: "Added push mirror for repository TestUser/TestRepo.",
		Metadata: metaPairs(
			"repo_id", r.ID,
			"repo", r.FullName(),
			"push_mirror_id", m.ID,
			"remote_address", m.RemoteAddress,
		),
	})
	e.Time = time.Time{}
	assert.Equal(t, audit_model.RepositoryMirrorPushAdd, e.Action)
	assert.Equal(t, "TestUser/TestRepo", e.Scope.Name)
	assert.Equal(t, r.ID, e.Metadata["repo_id"])

	assert.Empty(t, e.IPAddress)
	ctx = context.WithValue(ctx, httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})
	e = buildEvent(ctx, RecordParams{
		Action:  audit_model.UserCreate,
		Actor:   ActorFromUser(doer),
		Scope:   ScopeFromUser(u),
		Message: "",
	})
	assert.Equal(t, "127.0.0.1", e.IPAddress)
}

func TestEntityRefDisplay(t *testing.T) {
	settingAppURL := func() { /* AppSubURL set in init elsewhere */ }

	_ = settingAppURL

	ref := EntityRef{Type: audit_model.ScopeUser, ID: 1, Name: "TestUser"}
	assert.Equal(t, "TestUser", ref.DisplayName())

	sys := ScopeSystem()
	assert.Equal(t, "System", sys.DisplayName())
}

func TestEncodeDecodeMetadata(t *testing.T) {
	raw := encodeMetadata(metaPairs("repo_id", int64(42), "repo", "o/r"))
	decoded := decodeMetadata(raw)
	assert.InDelta(t, 42.0, decoded["repo_id"], 0) // json numbers decode as float64
	assert.Equal(t, "o/r", decoded["repo"])
}
