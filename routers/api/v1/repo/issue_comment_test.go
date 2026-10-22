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
				ctx.Repo.Permission = access_model.Permission{}
				ctx.Repo.Permission.SetUnitsWithDefaultAccessMode([]*repo_model.RepoUnit{{Type: unit.TypeIssues}}, perm_model.AccessModeRead)
				ctx.SetPathParam("index", index)
				handler(ctx)
				return resp.Code
			}
			assert.Equal(t, http.StatusOK, request("1"))
			assert.Equal(t, http.StatusNotFound, request("2"))
		})
	}
}
