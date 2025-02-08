// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/web"
	"code.gitea.io/gitea/services/context"
	"code.gitea.io/gitea/services/contexttest"
	"code.gitea.io/gitea/services/forms"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewReleasePost(t *testing.T) {
	unittest.PrepareTestEnv(t)

	get := func(t *testing.T, tagName string) *context.Context {
		ctx, _ := contexttest.MockContext(t, "user2/repo1/releases/new?tag="+tagName)
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()
		NewRelease(ctx)
		return ctx
	}

	t.Run("NewReleasePage", func(t *testing.T) {
		ctx := get(t, "v1.1")
		assert.Empty(t, ctx.Data["ShowCreateTagOnlyButton"])
		ctx = get(t, "new-tag-name")
		assert.NotEmpty(t, ctx.Data["ShowCreateTagOnlyButton"])
	})

	post := func(t *testing.T, form forms.NewReleaseForm) *context.Context {
		ctx, _ := contexttest.MockContext(t, "user2/repo1/releases/new")
		contexttest.LoadUser(t, ctx, 2)
		contexttest.LoadRepo(t, ctx, 1)
		contexttest.LoadGitRepo(t, ctx)
		defer ctx.Repo.GitRepo.Close()
		web.SetForm(ctx, &form)
		NewReleasePost(ctx)
		return ctx
	}

	loadRelease := func(t *testing.T, tagName string) *repo_model.Release {
		return unittest.GetBean(t, &repo_model.Release{}, unittest.Cond("repo_id=1 AND tag_name=?", tagName))
	}

	t.Run("NewTagRelease", func(t *testing.T) {
		post(t, forms.NewReleaseForm{
			TagName: "newtag",
			Target:  "master",
			Title:   "title",
			Content: "content",
		})
		rel := loadRelease(t, "newtag")
		require.NotNil(t, rel)
		assert.False(t, rel.IsTag)
		assert.Equal(t, "master", rel.Target)
		assert.Equal(t, "title", rel.Title)
		assert.Equal(t, "content", rel.Note)
	})

	t.Run("ReleaseExistsDoUpdate(non-tag)", func(t *testing.T) {
		ctx := post(t, forms.NewReleaseForm{
			TagName: "v1.1",
			Target:  "master",
			Title:   "updated-title",
			Content: "updated-content",
		})
		rel := loadRelease(t, "v1.1")
		require.NotNil(t, rel)
		assert.False(t, rel.IsTag)
		assert.Equal(t, "testing-release", rel.Title)
		assert.NotEmpty(t, ctx.Flash.ErrorMsg)
	})

	t.Run("ReleaseExistsDoUpdate(tag-only)", func(t *testing.T) {
		ctx := post(t, forms.NewReleaseForm{
			TagName: "delete-tag", // a strange name, but it is the only "is_tag=true" fixture
			Target:  "master",
			Title:   "updated-title",
			Content: "updated-content",
			TagOnly: true,
		})
		rel := loadRelease(t, "delete-tag")
		require.NotNil(t, rel)
		assert.True(t, rel.IsTag) // the record should not be updated because the request is "tag-only". TODO: need to improve the logic?
		assert.Equal(t, "delete-tag", rel.Title)
		assert.NotEmpty(t, ctx.Flash.ErrorMsg)
		assert.NotEmpty(t, ctx.Data["ShowCreateTagOnlyButton"]) // still show the "tag-only" button
	})

	t.Run("ReleaseExistsDoUpdate(tag-release)", func(t *testing.T) {
		ctx := post(t, forms.NewReleaseForm{
			TagName: "delete-tag", // a strange name, but it is the only "is_tag=true" fixture
			Target:  "master",
			Title:   "updated-title",
			Content: "updated-content",
		})
		rel := loadRelease(t, "delete-tag")
		require.NotNil(t, rel)
		assert.False(t, rel.IsTag) // the tag has been "updated" to be a real "release"
		assert.Equal(t, "updated-title", rel.Title)
		assert.Empty(t, ctx.Flash.ErrorMsg)
	})

	t.Run("TagOnly", func(t *testing.T) {
		ctx := post(t, forms.NewReleaseForm{
			TagName: "new-tag-only",
			Target:  "master",
			Title:   "title",
			Content: "content",
			TagOnly: true,
		})
		rel := loadRelease(t, "new-tag-only")
		require.NotNil(t, rel)
		assert.True(t, rel.IsTag)
		assert.Empty(t, ctx.Flash.ErrorMsg)
	})

	t.Run("TagOnlyConflict", func(t *testing.T) {
		ctx := post(t, forms.NewReleaseForm{
			TagName: "v1.1",
			Target:  "master",
			Title:   "title",
			Content: "content",
			TagOnly: true,
		})
		rel := loadRelease(t, "v1.1")
		require.NotNil(t, rel)
		assert.False(t, rel.IsTag)
		assert.NotEmpty(t, ctx.Flash.ErrorMsg)
	})
}

func TestCalReleaseNumCommitsBehind(t *testing.T) {
	unittest.PrepareTestEnv(t)
	ctx, _ := contexttest.MockContext(t, "user2/repo-release/releases")
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, 57)
	contexttest.LoadGitRepo(t, ctx)
	t.Cleanup(func() { ctx.Repo.GitRepo.Close() })

	releases, err := db.Find[repo_model.Release](ctx, repo_model.FindReleasesOptions{
		IncludeDrafts: ctx.Repo.CanWrite(unit.TypeReleases),
		RepoID:        ctx.Repo.Repository.ID,
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
