// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"code.gitea.io/gitea/modules/context"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
	"code.gitea.io/gitea/modules/test"
)

func TestRepoEdit(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadUser(t, ctx, 2)
	ctx.Repo.Owner = ctx.User
	description := "new description"
	website := "http://wwww.newwebsite.com"
	private := true
	enableIssues := false
	enableWiki := false
	defaultBranch := "master"
	enablePullRequests := true
	ignoreWhitespaceConflicts := true
	allowMerge := false
	allowRebase := false
	allowRebaseMerge := false
	allowSquashMerge := false
	archived := true
	opts := api.EditRepoOption{
		Name:                      &ctx.Repo.Repository.Name,
		Description:               &description,
		Website:                   &website,
		Private:                   &private,
		EnableIssues:              &enableIssues,
		EnableWiki:                &enableWiki,
		DefaultBranch:             &defaultBranch,
		EnablePullRequests:        &enablePullRequests,
		IgnoreWhitespaceConflicts: &ignoreWhitespaceConflicts,
		AllowMerge:                &allowMerge,
		AllowRebase:               &allowRebase,
		AllowRebaseMerge:          &allowRebaseMerge,
		AllowSquash:               &allowSquashMerge,
		Archived:                  &archived,
	}

	Edit(&context.APIContext{Context: ctx, Org: nil}, opts)

	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())
	models.AssertExistsAndLoadBean(t, &models.Repository{
		ID: 1,
	}, models.Cond("name = ? AND is_archived = 1", *opts.Name))
}

func TestRepoEditNameChange(t *testing.T) {
	models.PrepareTestEnv(t)

	ctx := test.MockContext(t, "user2/repo1")
	test.LoadRepo(t, ctx, 1)
	test.LoadUser(t, ctx, 2)
	ctx.Repo.Owner = ctx.User
	name := "newname"
	opts := api.EditRepoOption{
		Name: &name,
	}

	Edit(&context.APIContext{Context: ctx, Org: nil}, opts)
	assert.EqualValues(t, http.StatusOK, ctx.Resp.Status())

	models.AssertExistsAndLoadBean(t, &models.Repository{
		ID: 1,
	}, models.Cond("name = ?", opts.Name))
}
