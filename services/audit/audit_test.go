// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package audit

import (
	"context"
	"net/http"
	"testing"
	"time"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	git_model "code.gitea.io/gitea/models/git"
	organization_model "code.gitea.io/gitea/models/organization"
	repository_model "code.gitea.io/gitea/models/repo"
	secret_model "code.gitea.io/gitea/models/secret"
	user_model "code.gitea.io/gitea/models/user"
	webhook_model "code.gitea.io/gitea/models/webhook"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestBuildEvent(t *testing.T) {
	equal := func(expected, e *Event) {
		expected.Time = time.Time{}
		e.Time = time.Time{}

		assert.Equal(t, expected, e)
	}

	ctx := context.Background()

	u := &user_model.User{ID: 1, Name: "TestUser"}
	r := &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"}
	m := &repository_model.PushMirror{ID: 4}
	doer := &user_model.User{ID: 2, Name: "Doer"}

	equal(
		&Event{
			Action:  audit_model.UserCreate,
			Actor:   TypeDescriptor{Type: "user", ID: 2, Object: doer},
			Scope:   TypeDescriptor{Type: "user", ID: 1, Object: u},
			Target:  TypeDescriptor{Type: "user", ID: 1, Object: u},
			Message: "Created user TestUser.",
		},
		buildEvent(
			ctx,
			audit_model.UserCreate,
			doer,
			u,
			u,
			"Created user %s.",
			u.Name,
		),
	)
	equal(
		&Event{
			Action:  audit_model.RepositoryMirrorPushAdd,
			Actor:   TypeDescriptor{Type: "user", ID: 2, Object: doer},
			Scope:   TypeDescriptor{Type: "repository", ID: 3, Object: r},
			Target:  TypeDescriptor{Type: "push_mirror", ID: 4, Object: m},
			Message: "Added push mirror for repository TestUser/TestRepo.",
		},
		buildEvent(
			ctx,
			audit_model.RepositoryMirrorPushAdd,
			doer,
			r,
			m,
			"Added push mirror for repository %s.",
			r.FullName(),
		),
	)

	e := buildEvent(ctx, audit_model.UserCreate, doer, u, u, "")
	assert.Empty(t, e.IPAddress)

	ctx = context.WithValue(ctx, httplib.RequestContextKey, &http.Request{RemoteAddr: "127.0.0.1:1234"})

	e = buildEvent(ctx, audit_model.UserCreate, doer, u, u, "")
	assert.Equal(t, "127.0.0.1", e.IPAddress)
}

func TestScopeToDescription(t *testing.T) {
	cases := []struct {
		ShouldPanic bool
		Scope       any
		Expected    TypeDescriptor
	}{
		{
			Scope:       nil,
			ShouldPanic: true,
		},
		{
			Scope:    &systemObject,
			Expected: TypeDescriptor{Type: audit_model.TypeSystem, ID: 0},
		},
		{
			Scope:    &user_model.User{ID: 1, Name: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeUser, ID: 1},
		},
		{
			Scope:    &organization_model.Organization{ID: 2, Name: "TestOrg"},
			Expected: TypeDescriptor{Type: audit_model.TypeOrganization, ID: 2},
		},
		{
			Scope:    &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"},
			Expected: TypeDescriptor{Type: audit_model.TypeRepository, ID: 3},
		},
		{
			ShouldPanic: true,
			Scope:       &organization_model.Team{ID: 345, Name: "Team"},
		},
		{
			ShouldPanic: true,
			Scope:       1234,
		},
	}
	for _, c := range cases {
		if c.Scope != &systemObject {
			c.Expected.Object = c.Scope
		}

		if c.ShouldPanic {
			assert.Panics(t, func() {
				_ = scopeToDescription(c.Scope)
			})
		} else {
			assert.Equal(t, c.Expected, scopeToDescription(c.Scope), "Unexpected descriptor for scope: %T", c.Scope)
		}
	}
}

func TestTypeToDescription(t *testing.T) {
	setting.AppURL = "http://localhost:3000/"

	type Expected struct {
		TypeDescriptor TypeDescriptor
		DisplayName    string
		HTMLURL        string
	}

	cases := []struct {
		ShouldPanic bool
		Type        any
		Expected    Expected
	}{
		{
			Type:        nil,
			ShouldPanic: true,
		},
		{
			Type: &systemObject,
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeSystem, ID: 0},
				DisplayName:    "System",
			},
		},
		{
			Type: &user_model.User{ID: 1, Name: "TestUser"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeUser, ID: 1},
				DisplayName:    "TestUser",
				HTMLURL:        "http://localhost:3000/TestUser",
			},
		},
		{
			Type: &organization_model.Organization{ID: 2, Name: "TestOrg"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeOrganization, ID: 2},
				DisplayName:    "TestOrg",
				HTMLURL:        "http://localhost:3000/TestOrg",
			},
		},
		{
			Type: &user_model.EmailAddress{ID: 3, Email: "user@gitea.com"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeEmailAddress, ID: 3},
				DisplayName:    "user@gitea.com",
			},
		},
		{
			Type: &repository_model.Repository{ID: 3, Name: "TestRepo", OwnerName: "TestUser"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeRepository, ID: 3},
				DisplayName:    "TestUser/TestRepo",
				HTMLURL:        "http://localhost:3000/TestUser/TestRepo",
			},
		},
		{
			Type: &organization_model.Team{ID: 4, Name: "TestTeam"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeTeam, ID: 4},
				DisplayName:    "TestTeam",
			},
		},
		{
			Type: &auth_model.WebAuthnCredential{ID: 6, Name: "TestCredential"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeWebAuthnCredential, ID: 6},
				DisplayName:    "TestCredential",
			},
		},
		{
			Type: &user_model.UserOpenID{ID: 7, URI: "test://uri"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeOpenID, ID: 7},
				DisplayName:    "test://uri",
			},
		},
		{
			Type: &auth_model.AccessToken{ID: 8, Name: "TestToken"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeAccessToken, ID: 8},
				DisplayName:    "TestToken",
			},
		},
		{
			Type: &auth_model.OAuth2Application{ID: 9, Name: "TestOAuth2Application"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeOAuth2Application, ID: 9},
				DisplayName:    "TestOAuth2Application",
			},
		},
		{
			Type: &auth_model.Source{ID: 11, Name: "TestSource"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeAuthenticationSource, ID: 11},
				DisplayName:    "TestSource",
			},
		},
		{
			Type: &asymkey_model.PublicKey{ID: 13, Fingerprint: "TestPublicKey"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypePublicKey, ID: 13},
				DisplayName:    "TestPublicKey",
			},
		},
		{
			Type: &asymkey_model.GPGKey{ID: 14, KeyID: "TestGPGKey"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeGPGKey, ID: 14},
				DisplayName:    "TestGPGKey",
			},
		},
		{
			Type: &secret_model.Secret{ID: 15, Name: "TestSecret"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeSecret, ID: 15},
				DisplayName:    "TestSecret",
			},
		},
		{
			Type: &webhook_model.Webhook{ID: 16, URL: "test://webhook"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeWebhook, ID: 16},
				DisplayName:    "test://webhook",
			},
		},
		{
			Type: &git_model.ProtectedTag{ID: 17, NamePattern: "TestProtectedTag"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeProtectedTag, ID: 17},
				DisplayName:    "TestProtectedTag",
			},
		},
		{
			Type: &git_model.ProtectedBranch{ID: 18, RuleName: "TestProtectedBranch"},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypeProtectedBranch, ID: 18},
				DisplayName:    "TestProtectedBranch",
			},
		},
		{
			Type: &repository_model.PushMirror{ID: 19},
			Expected: Expected{
				TypeDescriptor: TypeDescriptor{Type: audit_model.TypePushMirror, ID: 19},
				DisplayName:    "",
			},
		},
		{
			ShouldPanic: true,
			Type:        1234,
		},
	}
	for _, c := range cases {
		if c.Type != &systemObject {
			c.Expected.TypeDescriptor.Object = c.Type
		}

		if c.ShouldPanic {
			assert.Panics(t, func() {
				_ = typeToDescription(c.Type)
			})
		} else {
			d := typeToDescription(c.Type)

			assert.Equal(t, c.Expected.TypeDescriptor, d, "Unexpected descriptor for type: %T", c.Type)
			assert.Equal(t, c.Expected.DisplayName, d.DisplayName(), "Unexpected display name for type: %T", c.Type)
			assert.Equal(t, c.Expected.HTMLURL, d.HTMLURL(), "Unexpected url for type: %T", c.Type)
		}
	}
}
