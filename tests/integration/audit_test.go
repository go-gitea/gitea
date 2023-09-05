// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	auth_model "code.gitea.io/gitea/models/auth"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"

	"github.com/stretchr/testify/assert"
)

type testAppender struct {
	Events []*audit.Event
}

func (a *testAppender) Record(ctx context.Context, e *audit.Event) {
	a.Events = append(a.Events, e)
}

func (a *testAppender) Close() error {
	return nil
}

func (a *testAppender) ReleaseReopen() error {
	a.Events = nil
	return nil
}

func TestAuditLogging(t *testing.T) {
	a := &testAppender{}
	audit.TestingOnlyAddAppender(a)
	defer audit.TestingOnlyRemoveAppender(a)

	onGiteaRun(t, func(*testing.T, *url.URL) {
		token := getUserToken(t, "user1", auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

		req := NewRequestWithJSON(t, "POST", "/api/v1/orgs?token="+token, &api.CreateOrgOption{
			UserName:    "user1_audit_org",
			FullName:    "User1's organization",
			Description: "This organization created by user1",
			Website:     "https://try.gitea.io",
			Location:    "Shanghai",
			Visibility:  "limited",
		})
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user1_audit_org?token="+token, &api.EditOrgOption{
			Description: "A new description",
			Website:     "https://try.gitea.io/new",
			Location:    "Beijing",
			Visibility:  "private",
		})
		MakeRequest(t, req, http.StatusOK)

		req = NewRequest(t, "DELETE", "/api/v1/orgs/user1_audit_org?token="+token)
		MakeRequest(t, req, http.StatusNoContent)

		req = NewRequestWithJSON(t, "POST", "/api/v1/user/repos?token="+token, &api.CreateRepoOption{
			Name: "audit_repo",
		})
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user1/audit_repo?token="+token, &api.EditRepoOption{
			Description: util.ToPointer("A new description"),
			Private:     util.ToPointer(true),
		})
		MakeRequest(t, req, http.StatusOK)

		req = NewRequestWithJSON(t, "PUT", "/api/v1/repos/user1/audit_repo/actions/secrets/audit_secret?token="+token, &api.CreateOrUpdateSecretOption{
			Data: "my secret",
		})
		MakeRequest(t, req, http.StatusCreated)

		req = NewRequest(t, "DELETE", "/api/v1/repos/user1/audit_repo?token="+token)
		MakeRequest(t, req, http.StatusNoContent)

		cases := []struct {
			Action audit.Action
			Scope  audit.TypeDescriptor
			Target audit.TypeDescriptor
		}{
			{
				Action: audit.UserAccessTokenAdd,
				Scope:  audit.TypeDescriptor{Type: "user", FriendlyName: "user1"},
				Target: audit.TypeDescriptor{Type: "access_token"}, // can't test name because it depends on other tests
			},
			{
				Action: audit.OrganizationCreate,
				Scope:  audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
				Target: audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
			},
			{
				Action: audit.OrganizationUpdate,
				Scope:  audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
				Target: audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
			},
			{
				Action: audit.OrganizationVisibility,
				Scope:  audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
				Target: audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
			},
			{
				Action: audit.OrganizationDelete,
				Scope:  audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
				Target: audit.TypeDescriptor{Type: "organization", FriendlyName: "user1_audit_org"},
			},
			{
				Action: audit.RepositoryCreate,
				Scope:  audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
				Target: audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
			},
			{
				Action: audit.RepositoryUpdate,
				Scope:  audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
				Target: audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
			},
			{
				Action: audit.RepositoryVisibility,
				Scope:  audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
				Target: audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
			},
			{
				Action: audit.UserSecretAdd,
				Scope:  audit.TypeDescriptor{Type: "user", FriendlyName: "user1"},
				Target: audit.TypeDescriptor{Type: "secret", FriendlyName: "AUDIT_SECRET"},
			},
			{
				Action: audit.RepositoryDelete,
				Scope:  audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
				Target: audit.TypeDescriptor{Type: "repository", FriendlyName: "user1/audit_repo"},
			},
		}

		assert.Len(t, a.Events, len(cases))
		for i, c := range cases {
			e := a.Events[i]

			assert.Equal(t, c.Action, e.Action)

			assert.Equal(t, "user", e.Doer.Type)
			assert.EqualValues(t, int64(1), e.Doer.PrimaryKey)

			// Can't test PrimaryKey because it depends on other tests

			assert.Equal(t, c.Scope.Type, e.Scope.Type)
			if c.Scope.FriendlyName != "" {
				assert.Equal(t, c.Scope.FriendlyName, e.Scope.FriendlyName)
			}

			assert.Equal(t, c.Target.Type, e.Target.Type)
			if c.Target.FriendlyName != "" {
				assert.Equal(t, c.Target.FriendlyName, e.Target.FriendlyName)
			}
		}
	})

	audit.ReleaseReopen()
}
