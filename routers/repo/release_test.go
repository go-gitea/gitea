// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/auth"
	"code.gitea.io/gitea/modules/test"
)

func TestNewReleasePost(t *testing.T) {
	for _, testCase := range []struct {
		RepoID  int64
		UserID  int64
		TagName string
		Form    auth.NewReleaseForm
	}{
		{
			RepoID:  1,
			UserID:  2,
			TagName: "v1.1", // pre-existing tag
			Form: auth.NewReleaseForm{
				TagName: "newtag",
				Target:  "master",
				Title:   "title",
				Content: "content",
			},
		},
		{
			RepoID:  1,
			UserID:  2,
			TagName: "newtag",
			Form: auth.NewReleaseForm{
				TagName: "newtag",
				Target:  "master",
				Title:   "title",
				Content: "content",
			},
		},
	} {
		models.PrepareTestEnv(t)

		ctx := test.MockContext(t, "user2/repo1/releases/new")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		test.LoadGitRepo(t, ctx)
		NewReleasePost(ctx, testCase.Form)
		models.AssertExistsAndLoadBean(t, &models.Release{
			RepoID:      1,
			PublisherID: 2,
			TagName:     testCase.Form.TagName,
			Target:      testCase.Form.Target,
			Title:       testCase.Form.Title,
			Note:        testCase.Form.Content,
		}, models.Cond("is_draft=?", len(testCase.Form.Draft) > 0))
		ctx.Repo.GitRepo.Close()
	}
}
