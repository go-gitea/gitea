// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	actions_model "gitea.dev/models/actions"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	api "gitea.dev/modules/structs"
	repo_service "gitea.dev/services/repository"
	files_service "gitea.dev/services/repository/files"

	"github.com/stretchr/testify/require"
)

// No issue event of a confidential issue may start a workflow: a run, its title and its logs are as public as the
// repository's Actions tab.
func TestConfidentialIssueActionsEvents(t *testing.T) {
	onGiteaRun(t, func(t *testing.T, _ *url.URL) {
		user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
		repo, err := repo_service.CreateRepository(t.Context(), user2, user2, repo_service.CreateRepoOptions{
			Name: "confidential-actions", AutoInit: true, Readme: "Default", DefaultBranch: "main",
		})
		require.NoError(t, err)
		_, err = files_service.ChangeRepoFiles(t.Context(), repo, user2, &files_service.ChangeRepoFilesOptions{
			Files: []*files_service.ChangeRepoFile{{
				Operation: "create",
				TreePath:  ".gitea/workflows/issues.yml",
				ContentReader: strings.NewReader("on:\n  issues:\n    types: [opened, labeled, unlabeled]\njobs:\n" +
					"  test:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo hello\n"),
			}},
			Message: "add workflow", OldBranch: "main", NewBranch: "main",
			Author:    &files_service.IdentityOptions{GitUserName: user2.Name, GitUserEmail: user2.Email},
			Committer: &files_service.IdentityOptions{GitUserName: user2.Name, GitUserEmail: user2.Email},
			Dates:     &files_service.CommitDateOptions{Author: time.Now(), Committer: time.Now()},
		})
		require.NoError(t, err)

		token := getUserToken(t, user2.Name, auth_model.AccessTokenScopeWriteIssue, auth_model.AccessTokenScopeWriteRepository)
		base := fmt.Sprintf("/api/v1/repos/%s/%s", user2.Name, repo.Name)
		req := NewRequestWithJSON(t, "POST", base+"/labels", &api.CreateLabelOption{Name: "bug", Color: "#ff0000"}).AddTokenAuth(token)
		label := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Label{})

		runs := func() int {
			return unittest.GetCount(t, &actions_model.ActionRun{RepoID: repo.ID})
		}
		openAndClear := func(confidential bool) {
			req := NewRequestWithJSON(t, "POST", base+"/issues", &api.CreateIssueOption{
				Title: "report", Labels: []int64{label.ID}, Confidential: confidential,
			}).AddTokenAuth(token)
			issue := DecodeJSON(t, MakeRequest(t, req, http.StatusCreated), &api.Issue{})
			req = NewRequest(t, "DELETE", fmt.Sprintf("%s/issues/%d/labels", base, issue.Index)).AddTokenAuth(token)
			MakeRequest(t, req, http.StatusNoContent)
		}

		before := runs()
		openAndClear(true)
		require.Equal(t, before, runs(), "no event of a confidential issue starts a workflow")

		openAndClear(false)
		require.Greater(t, runs(), before, "the same steps on a public issue do start workflows")
	})
}
