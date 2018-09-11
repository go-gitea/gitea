// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"testing"

	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/test"
	api "code.gitea.io/sdk/gitea"
	"github.com/stretchr/testify/assert"
)

func BenchmarkRepo(b *testing.B) {
	samples := []struct {
		url       string
		name      string
		skipShort bool
	}{
		{url: "https://github.com/go-gitea/gitea.git", name: "gitea"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles"},
		{url: "https://github.com/moby/moby.git", name: "moby", skipShort: true},
		{url: "https://github.com/golang/go.git", name: "go", skipShort: true},
		{url: "https://github.com/torvalds/linux.git", name: "linux", skipShort: true},
	}
	prepareTestEnv(b)
	session := loginUser(b, "user2")
	b.ResetTimer()

	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			if testing.Short() && s.skipShort {
				b.Skip("skipping test in short mode.")
			}
			b.Run("Migrate", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					testRepoMigrate(b, session, s.url, s.name)
				}
			})
			b.Run("Access", func(b *testing.B) {
				var branches []*api.Branch
				b.Run("APIBranchList", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/api/v1/repos/%s/%s/branches", "user2", s.name)
						resp := session.MakeRequest(b, req, http.StatusOK)
						b.StopTimer()
						if len(branches) == 0 {
							DecodeJSON(b, resp, &branches) //Store for next phase
						}
						b.StartTimer()
					}
				})
				branchCount := len(branches)
				b.Run("WebViewCommit", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/%s/%s/commit/%s", "user2", s.name, branches[i%branchCount].Commit.ID)
						session.MakeRequest(b, req, http.StatusOK)
					}
				})
			})
		})
	}
}

//StringWithCharset random string (from https://www.calhoun.io/creating-random-strings-in-go/)
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func BenchmarkRepoBranchCommit(b *testing.B) {
	samples := []int64{1, 3, 15, 16}
	prepareTestEnv(b)
	b.ResetTimer()

	for _, repoID := range samples {
		b.StopTimer()
		repo := models.AssertExistsAndLoadBean(b, &models.Repository{ID: repoID}).(*models.Repository)
		b.StartTimer()
		b.Run(repo.Name, func(b *testing.B) {
			owner := models.AssertExistsAndLoadBean(b, &models.User{ID: repo.OwnerID}).(*models.User)
			session := loginUser(b, owner.LoginName)
			b.ResetTimer()
			b.Run("Create", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					branchName := StringWithCharset(5+rand.Intn(10), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
					b.StartTimer()
					testCreateBranch(b, session, owner.LoginName, repo.Name, "branch/master", branchName, http.StatusFound)
				}
			})
			b.Run("Access", func(b *testing.B) {
				var branches []*api.Branch
				req := NewRequestf(b, "GET", "/api/v1/%s/branches", repo.FullName())
				resp := session.MakeRequest(b, req, http.StatusOK)
				DecodeJSON(b, resp, &branches)
				branchCount := len(branches)
				b.ResetTimer() //We measure from here
				for i := 0; i < b.N; i++ {
					req := NewRequestf(b, "GET", "/%s/%s/commits/%s", owner.Name, repo.Name, branches[i%branchCount].Name)
					session.MakeRequest(b, req, http.StatusOK)
				}
			})
		})
	}
}

func benchmarkPullMergeHelper(b *testing.B, style models.MergeStyle) {
	repoName := "gitea"

	prepareTestEnv(b)
	session := loginUser(b, "user2")
	testRepoMigrate(b, session, "https://github.com/go-gitea/gitea.git", repoName)

	req := NewRequest(b, "GET", "/user/logout")
	session.MakeRequest(b, req, http.StatusFound)

	session = loginUser(b, "user1")
	testRepoFork(b, session, "user2", repoName, "user1", repoName)

	b.StopTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		branchName := fmt.Sprintf("pull%d", i)
		testEditFileToNewBranch(b, session, "user1", repoName, "master", branchName, "README.md", fmt.Sprintf("Hello, World (Edited) ver%d\n", i))
		resp := testPullCreate(b, session, "user1", repoName, branchName, fmt.Sprintf("This is a pull title for v%d", i))
		elem := strings.Split(test.RedirectURL(resp), "/")
		assert.EqualValues(b, "pulls", elem[3])
		b.StartTimer()
		testPullMerge(b, session, elem[1], elem[2], elem[4], style)
		b.StopTimer()
	}
}

func BenchmarkPullMerge(b *testing.B) {
	benchmarkPullMergeHelper(b, models.MergeStyleMerge)
}
func BenchmarkPullRebase(b *testing.B) {
	benchmarkPullMergeHelper(b, models.MergeStyleRebase)
}
func BenchmarkPullSquash(b *testing.B) {
	benchmarkPullMergeHelper(b, models.MergeStyleSquash)
}

//TODO list commits /repos/{owner}/{repo}/commits
