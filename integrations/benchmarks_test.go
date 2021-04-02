// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package integrations

import (
	"fmt"
	"net/http"
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

func BenchmarkRepoBranchCommit(b *testing.B) {
	b.Skip("benchmark broken") // TODO fix
	samples := []int64{1, 15, 16}
	defer prepareTestEnv(b)()
	b.ResetTimer()

	for _, repoID := range samples {
		b.StopTimer()
		repo := models.AssertExistsAndLoadBean(b, &models.Repository{ID: repoID}).(*models.Repository)
		b.StartTimer()
		b.Run(repo.Name, func(b *testing.B) {
			owner := models.AssertExistsAndLoadBean(b, &models.User{ID: repo.OwnerID}).(*models.User)
			session := loginUser(b, owner.LoginName)
			b.ResetTimer()
			b.Run("CreateBranch", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					testCreateBranch(b, session, owner.LoginName, repo.Name, "branch/master", fmt.Sprintf("new_branch_nr%d", i), http.StatusFound)
				}
			})
			b.Run("AccessBranchCommits", func(b *testing.B) {
				var branches []*api.Branch
				req := NewRequestf(b, "GET", "/api/v1/%s/branches", repo.FullName())
				resp := session.MakeRequest(b, req, http.StatusOK)
				DecodeJSON(b, resp, &branches)
				b.ResetTimer() //We measure from here
				if len(branches) != 0 {
					for i := 0; i < b.N; i++ {
						req := NewRequestf(b, "GET", "/api/v1/%s/commits?sha=%s", repo.FullName(), branches[i%len(branches)].Name)
						session.MakeRequest(b, req, http.StatusOK)
					}
				}
			})
		})
	}
}

//TODO list commits /repos/{owner}/{repo}/commits
