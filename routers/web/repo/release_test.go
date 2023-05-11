// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/test"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
)

func TestNewReleasePost(t *testing.T) {
	for _, testCase := range []struct {
		RepoID  int64
		UserID  int64
		TagName string
		Form    forms.NewReleaseForm
	}{
		{
			RepoID:  1,
			UserID:  2,
			TagName: "v1.1", // pre-existing tag
			Form: forms.NewReleaseForm{
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
			Form: forms.NewReleaseForm{
				TagName: "newtag",
				Target:  "master",
				Title:   "title",
				Content: "content",
			},
		},
	} {
		unittest.PrepareTestEnv(t)

		ctx := test.MockContext(t, "user2/repo1/releases/new")
		test.LoadUser(t, ctx, 2)
		test.LoadRepo(t, ctx, 1)
		test.LoadGitRepo(t, ctx)
		web.SetForm(ctx, &testCase.Form)
		NewReleasePost(ctx)
		unittest.AssertExistsAndLoadBean(t, &repo_model.Release{
			RepoID:      1,
			PublisherID: 2,
			TagName:     testCase.Form.TagName,
			Target:      testCase.Form.Target,
			Title:       testCase.Form.Title,
			Note:        testCase.Form.Content,
		}, unittest.Cond("is_draft=?", len(testCase.Form.Draft) > 0))
		ctx.Repo.GitRepo.Close()
	}
}

func TestNewReleasesList(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx := test.MockContext(t, "user2/repo-release/releases")
	test.LoadUser(t, ctx, 2)
	test.LoadRepo(t, ctx, 57)
	test.LoadGitRepo(t, ctx)
	t.Cleanup(func() { ctx.Repo.GitRepo.Close() })

	Releases(ctx)
	releases := ctx.Data["Releases"].([]*repo_model.Release)
	type computedFields struct {
		NumCommitsBehind int64
		TargetBehind     string
	}
	expectedComputation := map[string]computedFields{
		"v1.0": {
			NumCommitsBehind: 3,
			TargetBehind:     "main",
		},
		"v1.1": {
			NumCommitsBehind: 1,
			TargetBehind:     "main",
		},
		"v2.0": {
			NumCommitsBehind: 0,
			TargetBehind:     "main",
		},
		"non-existing-target-branch": {
			NumCommitsBehind: 1,
			TargetBehind:     "main",
		},
		"empty-target-branch": {
			NumCommitsBehind: 1,
			TargetBehind:     "main",
		},
	}
	for _, r := range releases {
		actual := computedFields{
			NumCommitsBehind: r.NumCommitsBehind,
			TargetBehind:     r.TargetBehind,
		}
		assert.Equal(t, expectedComputation[r.TagName], actual, "wrong computed fields for %s: %#v", r.TagName, r)
	}
}
