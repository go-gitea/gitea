// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	audit_model "code.gitea.io/gitea/models/audit"
	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/audit"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAuditLogging(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	assert.NoError(t, db.TruncateBeans(db.DefaultContext, &audit_model.Event{}))

	actor := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	token := getUserToken(t, actor.Name, auth_model.AccessTokenScopeWriteOrganization, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

	req := NewRequestWithJSON(t, "POST", "/api/v1/orgs", &api.CreateOrgOption{
		UserName:    "user1_audit_org",
		FullName:    "User1's organization",
		Description: "This organization created by user1",
		Website:     "https://try.gitea.io",
		Location:    "Universe",
		Visibility:  "limited",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, "PATCH", "/api/v1/orgs/user1_audit_org", &api.EditOrgOption{
		Visibility: "private",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithJSON(t, "POST", "/api/v1/user/repos", &api.CreateRepoOption{
		Name: "audit_repo",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	req = NewRequestWithJSON(t, "PATCH", "/api/v1/repos/user1/audit_repo", &api.EditRepoOption{
		Private: util.ToPointer(true),
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusOK)

	req = NewRequestWithJSON(t, "PUT", "/api/v1/repos/user1/audit_repo/actions/secrets/audit_secret", &api.CreateOrUpdateSecretOption{
		Data: "my secret",
	}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusCreated)

	type TestTypeDescriptor struct {
		Type        audit_model.ObjectType
		DisplayName string
		HTMLURL     string
	}

	cases := []struct {
		Action audit_model.Action
		Scope  TestTypeDescriptor
		Target TestTypeDescriptor
	}{
		{
			Action: audit_model.UserAccessTokenAdd,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeUser, DisplayName: "user1", HTMLURL: setting.AppURL + "user1"},
			Target: TestTypeDescriptor{Type: audit_model.TypeAccessToken, DisplayName: fmt.Sprintf("api-testing-token-%d", atomic.LoadInt64(&tokenCounter))},
		},
		{
			Action: audit_model.OrganizationCreate,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeOrganization, DisplayName: "user1_audit_org", HTMLURL: setting.AppURL + "user1_audit_org"},
			Target: TestTypeDescriptor{Type: audit_model.TypeOrganization, DisplayName: "user1_audit_org", HTMLURL: setting.AppURL + "user1_audit_org"},
		},
		{
			Action: audit_model.OrganizationVisibility,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeOrganization, DisplayName: "user1_audit_org", HTMLURL: setting.AppURL + "user1_audit_org"},
			Target: TestTypeDescriptor{Type: audit_model.TypeOrganization, DisplayName: "user1_audit_org", HTMLURL: setting.AppURL + "user1_audit_org"},
		},
		{
			Action: audit_model.RepositoryCreate,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeRepository, DisplayName: "user1/audit_repo", HTMLURL: setting.AppURL + "user1/audit_repo"},
			Target: TestTypeDescriptor{Type: audit_model.TypeRepository, DisplayName: "user1/audit_repo", HTMLURL: setting.AppURL + "user1/audit_repo"},
		},
		{
			Action: audit_model.RepositoryVisibility,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeRepository, DisplayName: "user1/audit_repo", HTMLURL: setting.AppURL + "user1/audit_repo"},
			Target: TestTypeDescriptor{Type: audit_model.TypeRepository, DisplayName: "user1/audit_repo", HTMLURL: setting.AppURL + "user1/audit_repo"},
		},
		{
			Action: audit_model.UserSecretAdd,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeUser, DisplayName: "user1", HTMLURL: setting.AppURL + "user1"},
			Target: TestTypeDescriptor{Type: audit_model.TypeSecret, DisplayName: "AUDIT_SECRET"},
		},
	}

	events, total, err := audit.FindEvents(db.DefaultContext, &audit_model.EventSearchOptions{Sort: audit_model.SortTimestampAsc})
	assert.NoError(t, err)
	assert.EqualValues(t, len(cases), total)
	assert.Len(t, events, int(total))

	for i, c := range cases {
		e := events[i]

		assert.Equal(t, c.Action, e.Action)

		assert.Equal(t, audit_model.TypeUser, e.Actor.Type)
		assert.NotNil(t, e.Actor.Object)
		assert.Equal(t, actor.ID, e.Actor.ID)

		assert.Equal(t, c.Scope.Type, e.Scope.Type)
		assert.NotNil(t, e.Scope.Object)
		assert.Equal(t, c.Scope.DisplayName, e.Scope.DisplayName())
		assert.Equal(t, c.Scope.HTMLURL, e.Scope.HTMLURL())

		assert.Equal(t, c.Target.Type, e.Target.Type)
		assert.NotNil(t, e.Target.Object)
		assert.Equal(t, c.Target.DisplayName, e.Target.DisplayName())
		assert.Equal(t, c.Target.HTMLURL, e.Target.HTMLURL())
	}

	// Deleted objects don't have display names anymore

	assert.NoError(t, db.TruncateBeans(db.DefaultContext, &audit_model.Event{}))

	req = NewRequest(t, "DELETE", "/api/v1/orgs/user1_audit_org").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	req = NewRequest(t, "DELETE", "/api/v1/repos/user1/audit_repo").
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	cases = []struct {
		Action audit_model.Action
		Scope  TestTypeDescriptor
		Target TestTypeDescriptor
	}{
		{
			Action: audit_model.OrganizationDelete,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeOrganization},
			Target: TestTypeDescriptor{Type: audit_model.TypeOrganization},
		},
		{
			Action: audit_model.RepositoryDelete,
			Scope:  TestTypeDescriptor{Type: audit_model.TypeRepository},
			Target: TestTypeDescriptor{Type: audit_model.TypeRepository},
		},
	}

	events, total, err = audit.FindEvents(db.DefaultContext, &audit_model.EventSearchOptions{Sort: audit_model.SortTimestampAsc})
	assert.NoError(t, err)
	assert.EqualValues(t, len(cases), total)
	assert.Len(t, events, int(total))

	for i, c := range cases {
		e := events[i]

		assert.Equal(t, c.Action, e.Action)

		assert.Equal(t, audit_model.TypeUser, e.Actor.Type)
		assert.NotNil(t, e.Actor.Object)
		assert.Equal(t, actor.ID, e.Actor.ID)

		assert.Equal(t, c.Scope.Type, e.Scope.Type)
		assert.Nil(t, e.Scope.Object)
		assert.Empty(t, e.Scope.DisplayName())
		assert.Empty(t, e.Scope.HTMLURL())

		assert.Equal(t, c.Target.Type, e.Target.Type)
		assert.Nil(t, e.Target.Object)
		assert.Empty(t, e.Target.DisplayName())
		assert.Empty(t, e.Target.HTMLURL())
	}
}
