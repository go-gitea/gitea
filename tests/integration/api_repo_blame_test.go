// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	repo_service "gitea.dev/services/repository"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoBlame(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	commitID := func(rev string) string {
		id, err := git.GetFullCommitID(t.Context(), repo1, rev)
		require.NoError(t, err)
		return id
	}
	require.NoError(t, git.ForceFastImport(t.Context(), repo1, []git.FastImportCommit{
		{Ref: "refs/heads/blame-test", Files: []git.FastImportFile{{Path: "dir/file.txt", Content: "one\ntwo\nthree\n"}}},
		{
			Ref:       "refs/heads/blame-test",
			Parent:    ":1",
			Message:   "Second",
			Files:     []git.FastImportFile{{Path: "dir/file.txt", Content: "one\nTWO\nthree\n"}},
			Committer: &git.Signature{Name: "Bob", Email: "bob@example.com", When: time.Unix(1714310500, 0).UTC()},
		},
	}))
	first, second := commitID("blame-test~1"), commitID("blame-test")

	getBlame := func(t *testing.T, urlPath string) *api.FileBlame {
		return DecodeJSON(t, MakeRequest(t, NewRequest(t, "GET", urlPath), http.StatusOK), &api.FileBlame{})
	}
	type blameRange struct {
		Start, End int
		SHA        string
	}
	rangesOf := func(blame *api.FileBlame) (ret []blameRange) {
		for _, part := range blame.Ranges {
			ret = append(ret, blameRange{part.StartLine, part.EndLine, part.Commit.SHA})
		}
		return ret
	}
	expectedRanges := []blameRange{{1, 1, first}, {2, 2, second}, {3, 3, first}}

	t.Run("Ranges", func(t *testing.T) {
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+second)
		assert.Equal(t, second, blame.CommitID)
		assert.Equal(t, "dir/file.txt", blame.Path)
		assert.False(t, blame.UsesIgnoreRevs)
		require.Equal(t, expectedRanges, rangesOf(blame))
		commit := blame.Ranges[1].Commit
		assert.Equal(t, setting.AppURL+"api/v1/repos/user2/repo1/git/commits/"+second, commit.URL)
		assert.Equal(t, "Second", commit.Summary)
		assert.Equal(t, "Bob", commit.Author.Name)
		assert.Equal(t, "bob@example.com", commit.Committer.Email)
		assert.Equal(t, "2024-04-28T13:21:40Z", commit.Committer.Date)
	})

	t.Run("IgnoreRevs", func(t *testing.T) {
		require.NoError(t, git.ForceFastImport(t.Context(), repo1, []git.FastImportCommit{{
			Ref:    "refs/heads/blame-test",
			Parent: second,
			Files:  []git.FastImportFile{{Path: ".git-blame-ignore-revs", Content: second + "\n"}},
		}}))
		third := commitID("blame-test")
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+third)
		assert.True(t, blame.UsesIgnoreRevs)
		assert.Equal(t, []blameRange{{1, 3, first}}, rangesOf(blame))

		blame = getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?bypass_ignore_revs=true&ref="+third)
		assert.False(t, blame.UsesIgnoreRevs)
		assert.Equal(t, expectedRanges, rangesOf(blame))
	})

	t.Run("NotFound", func(t *testing.T) {
		for _, urlPath := range []string{
			"/api/v1/repos/user2/repo1/blame/dir/file.txt?ref=0000000000000000000000000000000000000000",
			"/api/v1/repos/user2/repo1/blame/dir/no-such-file.txt?ref=" + second,
			"/api/v1/repos/user2/repo1/blame/dir?ref=" + second,
			"/api/v1/repos/user2/repo1/blame/%2Fdir/..%2Fdir/file.txt?ref=" + second,
		} {
			MakeRequest(t, NewRequest(t, "GET", urlPath), http.StatusNotFound)
		}
	})

	t.Run("DefaultBranch", func(t *testing.T) {
		assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", getBlame(t, "/api/v1/repos/user2/repo1/blame/README.md").CommitID)
	})

	t.Run("FileTooLarge", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.MaxDisplayFileSize, 14)()
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+second), http.StatusUnprocessableEntity)
	})

	t.Run("EmptyRepoWithTag", func(t *testing.T) {
		repo1.IsEmpty = true
		require.NoError(t, repo_model.UpdateRepositoryColsNoAutoTime(t.Context(), repo1, "is_empty"))
		getBlame(t, "/api/v1/repos/user2/repo1/blame/README.md?ref=v1.1")
	})

	t.Run("RequiresCodeReadAccess", func(t *testing.T) {
		require.NoError(t, repo_service.UpdateRepositoryUnits(t.Context(), repo1, nil, []unit.Type{unit.TypeCode}))
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/blame/README.md"), http.StatusForbidden)
	})
}
