// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestBlockUser(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	countStars := func(t *testing.T, repoOwnerID, starrerID int64) int64 {
		count, err := db.Count[repo_model.Repository](t.Context(), &repo_model.StarredReposOptions{
			StarrerID:      starrerID,
			RepoOwnerID:    repoOwnerID,
			IncludePrivate: true,
		})
		assert.NoError(t, err)
		return count
	}

	countWatches := func(t *testing.T, repoOwnerID, watcherID int64) int64 {
		count, err := db.Count[repo_model.Repository](t.Context(), &repo_model.WatchedReposOptions{
			WatcherID:   watcherID,
			RepoOwnerID: repoOwnerID,
		})
		assert.NoError(t, err)
		return count
	}

	countRepositoryTransfers := func(t *testing.T, senderID, recipientID int64) int64 {
		transfers, err := repo_model.GetPendingRepositoryTransfers(t.Context(), &repo_model.PendingRepositoryTransferOptions{
			SenderID:    senderID,
			RecipientID: recipientID,
		})
		assert.NoError(t, err)
		return int64(len(transfers))
	}

	countAssignedIssues := func(t *testing.T, repoOwnerID, assigneeID int64) int64 {
		_, count, err := issues_model.GetAssignedIssues(t.Context(), &issues_model.AssignedIssuesOptions{
			AssigneeID:  assigneeID,
			RepoOwnerID: repoOwnerID,
		})
		assert.NoError(t, err)
		return count
	}

	countCollaborations := func(t *testing.T, repoOwnerID, collaboratorID int64) int64 {
		count, err := db.Count[repo_model.Collaboration](t.Context(), &repo_model.FindCollaborationOptions{
			CollaboratorID: collaboratorID,
			RepoOwnerID:    repoOwnerID,
		})
		assert.NoError(t, err)
		return count
	}

	for _, blocker := range []struct {
		name     string
		id       int64
		username string
	}{
		{"User", 16, "user16"},
		{"Organization", 3, "org3"},
	} {
		t.Run(blocker.name, func(t *testing.T) {
			var blockerToken string
			blocksURL := "/api/v1/user/blocks"
			counts := []func(*testing.T, int64, int64) int64{countStars, countWatches, countRepositoryTransfers}
			if blocker.name == "User" {
				blockerToken = getUserToken(t, blocker.username, auth_model.AccessTokenScopeWriteUser)
			} else {
				blockerToken = getUserToken(t, "user2", auth_model.AccessTokenScopeWriteUser, auth_model.AccessTokenScopeWriteOrganization)
				blocksURL = fmt.Sprintf("/api/v1/orgs/%s/blocks", blocker.username)
				counts = append(counts, countAssignedIssues)
			}
			counts = append(counts, countCollaborations)

			var blockeeID int64 = 10
			blockeeName := "user10"

			t.Run("Block", func(t *testing.T) {
				req := NewRequest(t, "PUT", blocksURL+"/"+blockeeName)
				MakeRequest(t, req, http.StatusUnauthorized)

				if blocker.name == "Organization" {
					req = NewRequest(t, "PUT", blocksURL+"/user4").
						AddTokenAuth(blockerToken)
					MakeRequest(t, req, http.StatusBadRequest) // can't block member
				}

				for _, count := range counts {
					assert.EqualValues(t, 1, count(t, blocker.id, blockeeID))
				}

				req = NewRequest(t, "GET", blocksURL+"/"+blockeeName).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusNotFound)

				req = NewRequest(t, "PUT", fmt.Sprintf("%s/%s?reason=test", blocksURL, blockeeName)).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusNoContent)

				for _, count := range counts {
					assert.EqualValues(t, 0, count(t, blocker.id, blockeeID))
				}

				req = NewRequest(t, "GET", blocksURL+"/"+blockeeName).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusNoContent)

				req = NewRequest(t, "PUT", blocksURL+"/"+blockeeName).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusBadRequest) // can't block blocked user

				req = NewRequest(t, "PUT", blocksURL+"/org3").
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusBadRequest) // can't block organization

				req = NewRequest(t, "GET", blocksURL)
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequest(t, "GET", blocksURL).
					AddTokenAuth(blockerToken)
				resp := MakeRequest(t, req, http.StatusOK)

				users := DecodeJSON(t, resp, []api.User{})

				assert.Len(t, users, 1)
				assert.Equal(t, blockeeName, users[0].UserName)
			})

			t.Run("Unblock", func(t *testing.T) {
				req := NewRequest(t, "DELETE", blocksURL+"/"+blockeeName)
				MakeRequest(t, req, http.StatusUnauthorized)

				req = NewRequest(t, "DELETE", blocksURL+"/"+blockeeName).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusNoContent)

				req = NewRequest(t, "DELETE", blocksURL+"/"+blockeeName).
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusBadRequest)

				req = NewRequest(t, "DELETE", blocksURL+"/org3").
					AddTokenAuth(blockerToken)
				MakeRequest(t, req, http.StatusBadRequest)

				req = NewRequest(t, "GET", blocksURL).
					AddTokenAuth(blockerToken)
				resp := MakeRequest(t, req, http.StatusOK)

				users := DecodeJSON(t, resp, []api.User{})

				assert.Empty(t, users)
			})
		})
	}
}
