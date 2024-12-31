// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"math/rand/v2"
	"net/http"
	"net/url"
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	api "code.gitea.io/gitea/modules/structs"
)

// StringWithCharset random string (from https://www.calhoun.io/creating-random-strings-in-go/)
func StringWithCharset(length int, charset string) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

func BenchmarkRepoBranchCommit(b *testing.B) {
	onGiteaRun(b, func(b *testing.B, u *url.URL) {
		samples := []int64{1, 2, 3}
		b.ResetTimer()

		for _, repoID := range samples {
			b.StopTimer()
			repo := unittest.AssertExistsAndLoadBean(b, &repo_model.Repository{ID: repoID})
			b.StartTimer()
			b.Run(repo.Name, func(b *testing.B) {
				session := loginUser(b, "user2")
				b.ResetTimer()
				b.Run("CreateBranch", func(b *testing.B) {
					b.StopTimer()
					branchName := StringWithCharset(5+rand.IntN(10), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
					b.StartTimer()
					for i := 0; i < b.N; i++ {
						b.Run("new_"+branchName, func(b *testing.B) {
							b.Skip("benchmark broken") // TODO fix
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
					b.ResetTimer() // We measure from here
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
