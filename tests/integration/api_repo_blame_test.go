// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strings"
	"testing"

	auth_model "gitea.dev/models/auth"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"
	api "gitea.dev/modules/structs"
	"gitea.dev/modules/test"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIRepoBlame(t *testing.T) {
	defer tests.PrepareTestEnv(t)()

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	fastImport := func(stream string) {
		_, _, err := gitcmd.NewCommand("fast-import").WithRepo(repo1).WithStdinBytes([]byte(stream)).RunStdString(t.Context())
		require.NoError(t, err)
	}
	revParse := func(rev string) string {
		stdout, _, err := gitcmd.NewCommand("rev-parse").AddDynamicArguments(rev).WithRepo(repo1).RunStdString(t.Context())
		require.NoError(t, err)
		return strings.TrimSpace(stdout)
	}
	fastImport(`commit refs/heads/blame-test
committer Alice <alice@example.com> 1714310400 +0000
data 5
First
M 100644 inline dir/file.txt
data 14
one
two
three

commit refs/heads/blame-test
committer Bob <bob@example.com> 1714310500 +0000
data 6
Second
M 100644 inline dir/file.txt
data 14
one
TWO
three
`)
	// the branch exists only in git, not in the database, so it is referred to by commit ID
	first, second := revParse("blame-test~1"), revParse("blame-test")

	getBlame := func(t *testing.T, urlPath string) *api.FileBlame {
		resp := MakeRequest(t, NewRequest(t, "GET", urlPath), http.StatusOK)
		return DecodeJSON(t, resp, &api.FileBlame{})
	}
	type blameRange struct {
		Start, End int
		SHA        string
		Lines      []string
	}
	rangesOf := func(blame *api.FileBlame) (ret []blameRange) {
		for _, r := range blame.Ranges {
			ret = append(ret, blameRange{r.StartLine, r.EndLine, r.Commit.SHA, r.Lines})
		}
		return ret
	}

	t.Run("Ranges", func(t *testing.T) {
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+second)
		assert.Equal(t, second, blame.CommitID)
		assert.Equal(t, "dir/file.txt", blame.Path)
		assert.False(t, blame.UsesIgnoreRevs)
		assert.Equal(t, []blameRange{
			{1, 1, first, []string{"one"}},
			{2, 2, second, []string{"TWO"}},
			{3, 3, first, []string{"three"}},
		}, rangesOf(blame))
		commit := blame.Ranges[1].Commit
		assert.Equal(t, "Second", commit.Summary)
		assert.Equal(t, "Bob", commit.Author.Name)
		assert.Equal(t, "bob@example.com", commit.Committer.Email)
		assert.Equal(t, "2024-04-28T13:21:40Z", commit.Committer.Date)

		blame = getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+first)
		assert.Equal(t, []blameRange{{1, 3, first, []string{"one", "two", "three"}}}, rangesOf(blame))
	})

	t.Run("IgnoreRevs", func(t *testing.T) {
		fastImport(`commit refs/heads/blame-test
committer Carol <carol@example.com> 1714310600 +0000
data 6
Ignore
from refs/heads/blame-test^0
M 100644 inline .git-blame-ignore-revs
data 41
` + second + `
`)
		third := revParse("blame-test")
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+third)
		assert.True(t, blame.UsesIgnoreRevs)
		assert.Equal(t, []blameRange{{1, 3, first, []string{"one", "TWO", "three"}}}, rangesOf(blame))

		blame = getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/file.txt?bypass_ignore_revs=true&ref="+third)
		assert.False(t, blame.UsesIgnoreRevs)
		assert.Len(t, blame.Ranges, 3)
		assert.Equal(t, second, blame.Ranges[1].Commit.SHA)
	})

	t.Run("NotFound", func(t *testing.T) {
		for _, urlPath := range []string{
			"/api/v1/repos/user2/repo1/blame/dir/file.txt?ref=no-such-branch",
			"/api/v1/repos/user2/repo1/blame/dir/file.txt?ref=0000000000000000000000000000000000000000",
			"/api/v1/repos/user2/repo1/blame/dir/no-such-file.txt?ref=" + second,
			"/api/v1/repos/user2/repo1/blame/dir?ref=" + second,
		} {
			MakeRequest(t, NewRequest(t, "GET", urlPath), http.StatusNotFound)
		}
	})

	t.Run("DefaultBranch", func(t *testing.T) {
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/README.md")
		assert.Equal(t, "65f1bf27bc3bf70f64657658635e66094edbcb4d", blame.CommitID)
		assert.Equal(t, []blameRange{{1, 3, "65f1bf27bc3bf70f64657658635e66094edbcb4d", []string{"# repo1", "", "Description for repo1"}}}, rangesOf(blame))
		assert.Equal(t, "Initial commit", blame.Ranges[0].Commit.Summary)
		assert.Equal(t, blame.CommitID, getBlame(t, "/api/v1/repos/user2/repo1/blame/README.md?ref=master").CommitID)
	})

	t.Run("PathTraversal", func(t *testing.T) {
		for _, urlPath := range []string{
			"/api/v1/repos/user2/repo1/blame/dir/..%2F..%2Fconfig",
			"/api/v1/repos/user2/repo1/blame/..%2F..%2F..%2Fetc%2Fpasswd",
			"/api/v1/repos/user2/repo1/blame/%2E%2E/%2E%2E/description",
		} {
			MakeRequest(t, NewRequest(t, "GET", urlPath), http.StatusNotFound)
		}
		// a path that stays inside the tree resolves to the same file
		blame := getBlame(t, "/api/v1/repos/user2/repo1/blame/dir/..%2Fdir/file.txt?ref="+second)
		assert.Equal(t, "dir/file.txt", blame.Path)
		assert.Len(t, blame.Ranges, 3)
	})

	t.Run("FileTooLarge", func(t *testing.T) {
		defer test.MockVariableValue(&setting.UI.MaxDisplayFileSize, 8)()
		MakeRequest(t, NewRequest(t, "GET", "/api/v1/repos/user2/repo1/blame/dir/file.txt?ref="+second), http.StatusUnprocessableEntity)
	})

	t.Run("Permissions", func(t *testing.T) {
		const privateURL = "/api/v1/repos/user2/repo16/blame/readme.md"
		MakeRequest(t, NewRequest(t, "GET", privateURL), http.StatusNotFound)

		token := getUserToken(t, "user2", auth_model.AccessTokenScopeReadUser)
		MakeRequest(t, NewRequest(t, "GET", privateURL).AddTokenAuth(token), http.StatusForbidden)

		token = getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository, auth_model.AccessTokenScopePublicOnly)
		MakeRequest(t, NewRequest(t, "GET", privateURL).AddTokenAuth(token), http.StatusNotFound)

		token = getUserToken(t, "user2", auth_model.AccessTokenScopeReadRepository)
		resp := MakeRequest(t, NewRequest(t, "GET", privateURL).AddTokenAuth(token), http.StatusOK)
		blame := DecodeJSON(t, resp, &api.FileBlame{})
		require.NotEmpty(t, blame.Ranges)
		assert.Equal(t, 1, blame.Ranges[0].StartLine)

		// user5 has no access to the private repository
		token = getUserToken(t, "user5", auth_model.AccessTokenScopeReadRepository)
		MakeRequest(t, NewRequest(t, "GET", privateURL).AddTokenAuth(token), http.StatusNotFound)
	})
}
