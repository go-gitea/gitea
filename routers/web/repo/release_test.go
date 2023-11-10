// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/contexttest"
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

		ctx, _ := contexttest.MockContext(t, "user2/repo1/releases/new")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadGitRepo(t, ctx)
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

func TestCalReleaseNumCommitsBehind(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo-release/releases")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 57)
	contexttest.LoadGitRepo(t, ctx)
	t.Cleanup(func() { ctx.Repo.GitRepo.Close() })

	releases, err := repo_model.GetReleasesByRepoID(ctx, ctx.Repo.Repository.ID, repo_model.FindReleasesOptions{
		IncludeDrafts: ctx.Repo.CanWrite(unit.TypeReleases),
	})
	assert.NoError(t, err)

	countCache := make(map[string]int64)
	for _, release := range releases {
		err := calReleaseNumCommitsBehind(ctx.Repo, release, countCache)
		assert.NoError(t, err)
	}

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
