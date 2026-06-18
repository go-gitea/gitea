// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type userSearchResults struct {
	Data []*api.User `json:"data"`
}

func TestSearchCandidatesIncludesInactiveUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	session := loginUser(t, "user1")
	req := NewRequest(t, "GET", "/user/search_candidates?q=user9")
	resp := session.MakeRequest(t, req, http.StatusOK)
	results := DecodeJSON(t, resp, &userSearchResults{})
	require.Len(t, results.Data, 1)
	assert.Equal(t, "user9", results.Data[0].UserName)
	assert.False(t, results.Data[0].IsActive)
}

func TestAPIAddCollaboratorAllowsInactiveUsers(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	inactiveUser := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 9})
	token := getUserToken(t, "user2", auth_model.AccessTokenScopeWriteRepository)
	req := NewRequestWithJSON(t, "PUT", fmt.Sprintf("/api/v1/repos/user2/repo1/collaborators/%s", inactiveUser.Name), &api.AddCollaboratorOption{}).
		AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)

	isCollaborator, err := repo_model.IsCollaborator(t.Context(), 1, inactiveUser.ID)
	assert.NoError(t, err)
	assert.True(t, isCollaborator)
}
