// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPILockIssueDoer checks that the user who locks or unlocks an issue of an organization repository
// is the author of the timeline comment, not the organization that owns the repository.
func TestAPILockIssueDoer(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 6})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: issue.RepoID})
	org := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})
	require.True(t, org.IsOrganization())
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{Name: "user2"}) // an owner of the organization
	token := getUserToken(t, doer.Name, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)
	urlStr := fmt.Sprintf("/api/v1/repos/%s/%s/issues/%d/lock", org.Name, repo.Name, issue.Index)

	lastLockPoster := func(commentType issues_model.CommentType) int64 {
		comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{IssueID: issue.ID, Type: commentType})
		return comment.PosterID
	}

	req := NewRequestWithJSON(t, "PUT", urlStr, api.LockIssueOption{Reason: "Spam"}).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	assert.Equal(t, doer.ID, lastLockPoster(issues_model.CommentTypeLock))

	req = NewRequest(t, "DELETE", urlStr).AddTokenAuth(token)
	MakeRequest(t, req, http.StatusNoContent)
	assert.Equal(t, doer.ID, lastLockPoster(issues_model.CommentTypeUnlock))
}
