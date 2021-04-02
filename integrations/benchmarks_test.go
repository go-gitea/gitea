// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"math/rand"
	"net/http"
	"net/url"
	"testing"

	"code.gitea.io/gitea/models"
	api "code.gitea.io/gitea/modules/structs"
)

func BenchmarkRepo(b *testing.B) {
	b.Skip("benchmark broken") // TODO fix
	samples := []struct {
		url       string
		name      string
		skipShort bool
	}{
		{url: "https://github.com/go-gitea/test_repo.git", name: "test_repo"},
		{url: "https://github.com/ethantkoenig/manyfiles.git", name: "manyfiles", skipShort: true},
	}
	defer prepareTestEnv(b)()
	session := loginUser(b, "user2")
	b.ResetTimer()

	for _, s := range samples {
		b.Run(s.name, func(b *testing.B) {
			if testing.Short() && s.skipShort {
				b.Skip("skipping test in short mode.")
			}
			b.Run("Migrate "+s.name, func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					req := NewRequestf(b, "DELETE", "/api/v1/repos/%s/%s", "user2", s.name)
					session.MakeRequest(b, req, NoExpectedStatus)
					testRepoMigrate(b, session, s.url, s.name)
				}
			})
			b.Run("Access", func(b *testing.B) {
				var branches []*api.Branch
				b.Run("APIBranchList", func(b *testing.B) {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/api/v1/repos/%s/%s/branches?page=1&limit=1", "user2", s.name)
						resp := session.MakeRequest(b, req, http.StatusOK)
						b.StopTimer()
						if len(branches) == 0 {
							DecodeJSON(b, resp, &branches) //Store for next phase
						}
						b.StartTimer()
					}
				})

				if len(branches) == 1 {
					b.Run("WebViewCommit", func(b *testing.B) {
						for i := 0; i < b.N; i++ {
							req := NewRequestf(b, "GET", "/%s/%s/commit/%s", "user2", s.name, branches[0].Commit.ID)
							session.MakeRequest(b, req, http.StatusOK)
						}
					})
				}
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
	onGiteaRunTB(b, func(t testing.TB, u *url.URL) {
		b := t.(*testing.B)

		samples := []int64{1, 2, 3}
		b.ResetTimer()

		for _, repoID := range samples {
			b.StopTimer()
			repo := models.AssertExistsAndLoadBean(b, &models.Repository{ID: repoID}).(*models.Repository)
			b.StartTimer()
			b.Run(repo.Name, func(b *testing.B) {
				session := loginUser(b, "user2")
				b.ResetTimer()
				b.Run("CreateBranch", func(b *testing.B) {
					b.Skip("benchmark broken") // TODO fix
					b.StopTimer()
					branchName := StringWithCharset(5+rand.Intn(10), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
					b.StartTimer()
					for i := 0; i < b.N; i++ {
						b.Run("new_"+branchName, func(b *testing.B) {
							testAPICreateBranch(b, session, repo.OwnerName, repo.Name, repo.DefaultBranch, "new_"+branchName, http.StatusCreated)
						})
					}
				})
				b.Run("GetBranches", func(b *testing.B) {
					req := NewRequestf(b, "GET", "/api/v1/repos/%s/branches", repo.FullName())
					session.MakeRequest(b, req, http.StatusOK)
				})
				b.Run("AccessCommits", func(b *testing.B) {
					var branches []*api.Branch
					req := NewRequestf(b, "GET", "/api/v1/repos/%s/branches", repo.FullName())
					resp := session.MakeRequest(b, req, http.StatusOK)
					DecodeJSON(b, resp, &branches)
					b.ResetTimer() //We measure from here
					if len(branches) != 0 {
						for i := 0; i < b.N; i++ {
							req := NewRequestf(b, "GET", "/api/v1/repos/%s/commits?sha=%s", repo.FullName(), branches[i%len(branches)].Name)
							session.MakeRequest(b, req, http.StatusOK)
						}
					}
				})
			})
		}
	})
}
