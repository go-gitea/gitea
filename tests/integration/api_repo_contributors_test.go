// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	contribution_model "code.gitea.io/gitea/models/repo/contribution"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/tests"

	"github.com/stretchr/testify/assert"
)

func TestAPIRepoContributorsIncludesStats(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NoError(t, repo.LoadOwner(t.Context()))

	user := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	assert.NoError(t, contribution_model.DeleteRepoContributorDailyStats(t.Context(), repo.ID))

	weekStart := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	dayStart := contribution_model.NewContributorDayStart(weekStart)
	_, err := db.GetEngine(t.Context()).Insert([]*contribution_model.ContributorDaily{
		{
			RepoID:       repo.ID,
			DayStart:     dayStart,
			UserID:       user.ID,
			Email:        user.GetEmail(),
			AuthorName:   user.DisplayName(),
			Additions:    7,
			Deletions:    2,
			Commits:      3,
			ChangedFiles: 4,
			UpdatedUnix:  timeutil.TimeStampNow(),
		},
		{
			RepoID:       repo.ID,
			DayStart:     dayStart + 86400000,
			UserID:       user.ID,
			Email:        user.GetEmail(),
			AuthorName:   user.DisplayName(),
			Additions:    5,
			Deletions:    1,
			Commits:      2,
			ChangedFiles: 3,
			UpdatedUnix:  timeutil.TimeStampNow(),
		},
	})
	assert.NoError(t, err)

	token := getUserToken(t, user.Name, auth_model.AccessTokenScopeReadRepository)
	req := NewRequest(t, "GET", fmt.Sprintf("/api/v1/repos/%s/%s/contributors", repo.OwnerName, repo.Name)).
		AddTokenAuth(token)
	resp := MakeRequest(t, req, http.StatusOK)

	var contributors []*api.Contributor
	DecodeJSON(t, resp, &contributors)

	if assert.NotEmpty(t, contributors) {
		var found *api.Contributor
		for _, contributor := range contributors {
			if contributor != nil && contributor.ID == user.ID {
				found = contributor
				break
			}
		}
		if assert.NotNil(t, found) {
			assert.Equal(t, int64(5), found.Commits)
			assert.Equal(t, int64(5), found.Contributions)
			assert.Equal(t, int64(12), found.Additions)
			assert.Equal(t, int64(3), found.Deletions)
			assert.Equal(t, int64(7), found.FilesChanged)
		}
	}
}
