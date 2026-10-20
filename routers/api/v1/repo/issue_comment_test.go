// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/http"
	"testing"

	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/services/context"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueReadEndpointsRequireUnitPermission(t *testing.T) {
	unittest.PrepareTestEnv(t)

	for name, handler := range map[string]func(*context.APIContext){
		"timeline":    ListIssueCommentsAndTimeline,
		"labels":      ListIssueLabels,
		"subscribers": GetIssueSubscribers,
		"attachments": ListIssueAttachments,
	} {
		t.Run(name, func(t *testing.T) {
			request := func(index string) int {
				ctx, resp := contexttest.MockAPIContext(t, "user2/repo1")
				contexttest.LoadRepo(t, ctx, 1)
				require.NoError(t, ctx.Repo.Repository.LoadUnits(ctx))
				var issuesOnly []*repo_model.RepoUnit
				for _, u := range ctx.Repo.Repository.Units {
					if u.Type == unit.TypeIssues {
						issuesOnly = append(issuesOnly, u)
					}
				}
				ctx.Repo.Permission = access_model.Permission{}
				ctx.Repo.Permission.SetUnitsWithDefaultAccessMode(issuesOnly, perm_model.AccessModeRead)
				ctx.SetPathParam("index", index)
				handler(ctx)
				return resp.Code
			}
			assert.Equal(t, http.StatusOK, request("1"))       // issue
			assert.Equal(t, http.StatusNotFound, request("2")) // pull request
		})
	}
}
