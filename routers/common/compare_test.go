// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package common

import (
	"testing"

	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"

	"github.com/stretchr/testify/assert"
)

func TestCompareRouters(t *testing.T) {
	kases := []struct {
		router        string
		compareRouter *CompareRouter
	}{
		{
			router: "",
			compareRouter: &CompareRouter{
				BaseOriRef: "",
				HeadOriRef: "",
				DotTimes:   3,
			},
		},
		{
			router: "main...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "main..develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				DotTimes:   2,
			},
		},
		{
			router: "main^...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 1,
				DotTimes:   3,
			},
		},
		{
			router: "main^^^^^...develop",
			compareRouter: &CompareRouter{
				BaseOriRef: "main",
				HeadOriRef: "develop",
				CaretTimes: 5,
				DotTimes:   3,
			},
		},
		{
			router: "develop",
			compareRouter: &CompareRouter{
				HeadOriRef: "develop",
				DotTimes:   3,
			},
		},
		{
			router: "lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
			},
		},
		{
			router: "main^...lunny/forked_repo:develop",
			compareRouter: &CompareRouter{
				BaseOriRef:    "main",
				HeadOwnerName: "lunny",
				HeadRepoName:  "forked_repo",
				HeadOriRef:    "develop",
				DotTimes:      3,
				CaretTimes:    1,
			},
		},
		{
			router: "v1.0...v1.1",
			compareRouter: &CompareRouter{
				BaseOriRef: "v1.0",
				HeadOriRef: "v1.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot-patch-1...v0.0.1",
			compareRouter: &CompareRouter{
				BaseOriRef: "teabot-patch-1",
				HeadOriRef: "v0.0.1",
				DotTimes:   3,
			},
		},
		{
			router: "teabot:feature1",
			compareRouter: &CompareRouter{
				HeadOwnerName: "teabot",
				HeadOriRef:    "feature1",
				DotTimes:      3,
			},
		},
		{
			router: "8eb19a5ae19abae15c0666d4ab98906139a7f439...283c030497b455ecfa759d4649f9f8b45158742e",
			compareRouter: &CompareRouter{
				BaseOriRef: "8eb19a5ae19abae15c0666d4ab98906139a7f439",
				HeadOriRef: "283c030497b455ecfa759d4649f9f8b45158742e",
				DotTimes:   3,
			},
		},
	}
	for _, kase := range kases {
		t.Run(kase.router, func(t *testing.T) {
			r, err := parseCompareRouter(kase.router)
			assert.NoError(t, err)
			assert.Equal(t, kase.compareRouter, r)
		})
	}
}

func Test_ParseComparePathParams(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	assert.NotNil(t, repo1)
	assert.NoError(t, repo1.LoadOwner(t.Context()))
	gitRepo1, err := gitrepo.OpenRepository(t.Context(), repo1)
	assert.NoError(t, err)
	defer gitRepo1.Close()

	repo10 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 10})
	assert.NotNil(t, repo10)
	assert.NoError(t, repo10.LoadOwner(t.Context()))
	gitRepo10, err := gitrepo.OpenRepository(t.Context(), repo10)
	assert.NoError(t, err)
	defer gitRepo10.Close()

	repo11 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 11})
	assert.NotNil(t, repo11)
	assert.NoError(t, repo11.LoadOwner(t.Context()))
	gitRepo11, err := gitrepo.OpenRepository(t.Context(), repo11)
	assert.NoError(t, err)
	defer gitRepo11.Close()
	assert.True(t, repo11.IsFork) // repo11 is a fork of repo10

	kases := []struct {
		repoName    string
		hasClose    bool
		router      string
		compareInfo *CompareInfo
	}{
		{
			repoName: "repo1",
			router:   "",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "master",
					BaseFullRef:   git.RefNameFromBranch("master"),
					HeadOriRef:    "master",
					HeadFullRef:   git.RefNameFromBranch("master"),
					HeadOwnerName: repo1.OwnerName,
					HeadRepoName:  repo1.Name,
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo1",
			router:   "master...branch2",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "master",
					BaseFullRef:   git.RefNameFromBranch("master"),
					HeadOriRef:    "branch2",
					HeadFullRef:   git.RefNameFromBranch("branch2"),
					HeadOwnerName: repo1.OwnerName,
					HeadRepoName:  repo1.Name,
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo1",
			router:   "DefaultBranch..branch2",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "DefaultBranch",
					BaseFullRef:   git.RefNameFromBranch("DefaultBranch"),
					HeadOriRef:    "branch2",
					HeadFullRef:   git.RefNameFromBranch("branch2"),
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					DotTimes:      2,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo1",
			router:   "DefaultBranch^...branch2",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "DefaultBranch",
					BaseFullRef:   git.RefNameFromBranch("DefaultBranch"),
					HeadOriRef:    "branch2",
					HeadFullRef:   git.RefNameFromBranch("branch2"),
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					CaretTimes:    1,
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo1",
			router:   "branch2",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    repo1.DefaultBranch,
					BaseFullRef:   git.RefNameFromBranch(repo1.DefaultBranch),
					HeadOriRef:    "branch2",
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					HeadFullRef:   git.RefNameFromBranch("branch2"),
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo10",
			hasClose: true,
			router:   "user13/repo11:develop",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    repo10.DefaultBranch,
					BaseFullRef:   git.RefNameFromBranch(repo10.DefaultBranch),
					HeadOwnerName: "user13",
					HeadRepoName:  "repo11",
					HeadOriRef:    "develop",
					HeadFullRef:   git.RefNameFromBranch("develop"),
					DotTimes:      3,
				},
				BaseRepo:    repo10,
				HeadUser:    repo11.Owner,
				HeadRepo:    repo11,
				HeadGitRepo: gitRepo11,
			},
		},
		{
			repoName: "repo10",
			hasClose: true,
			router:   "master...user13/repo11:develop",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "master",
					BaseFullRef:   git.RefNameFromBranch("master"),
					HeadOwnerName: "user13",
					HeadRepoName:  "repo11",
					HeadOriRef:    "develop",
					HeadFullRef:   git.RefNameFromBranch("develop"),
					DotTimes:      3,
				},
				BaseRepo:    repo10,
				HeadUser:    repo11.Owner,
				HeadRepo:    repo11,
				HeadGitRepo: gitRepo11,
			},
		},
		{
			repoName: "repo10",
			hasClose: true,
			router:   "DefaultBranch^...user13/repo11:develop",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "DefaultBranch",
					BaseFullRef:   git.RefNameFromBranch("DefaultBranch"),
					HeadOwnerName: "user13",
					HeadRepoName:  "repo11",
					HeadOriRef:    "develop",
					HeadFullRef:   git.RefNameFromBranch("develop"),
					DotTimes:      3,
					CaretTimes:    1,
				},
				BaseRepo:    repo10,
				HeadUser:    repo11.Owner,
				HeadRepo:    repo11,
				HeadGitRepo: gitRepo11,
			},
		},
		{
			repoName: "repo11",
			hasClose: true,
			router:   "user12/repo10:master",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    repo11.DefaultBranch,
					BaseFullRef:   git.RefNameFromBranch(repo11.DefaultBranch),
					HeadOwnerName: "user12",
					HeadRepoName:  "repo10",
					HeadOriRef:    "master",
					HeadFullRef:   git.RefNameFromBranch("master"),
					DotTimes:      3,
				},
				BaseRepo:    repo11,
				HeadUser:    repo10.Owner,
				HeadRepo:    repo10,
				HeadGitRepo: gitRepo10,
			},
		},
		{
			repoName: "repo1",
			router:   "master...v1.1",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "master",
					BaseFullRef:   git.RefNameFromBranch("master"),
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					HeadOriRef:    "v1.1",
					HeadFullRef:   git.RefNameFromTag("v1.1"),
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo1.Owner,
				HeadRepo:    repo1,
				HeadGitRepo: gitRepo1,
			},
		},
		{
			repoName: "repo10",
			hasClose: true,
			router:   "user13:develop",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    repo10.DefaultBranch,
					BaseFullRef:   git.RefNameFromBranch(repo10.DefaultBranch),
					HeadOwnerName: "user13",
					HeadOriRef:    "develop",
					HeadFullRef:   git.RefNameFromBranch("develop"),
					DotTimes:      3,
				},
				BaseRepo:    repo10,
				HeadUser:    repo11.Owner,
				HeadRepo:    repo11,
				HeadGitRepo: gitRepo11,
			},
		},
		{
			repoName: "repo1",
			router:   "65f1bf27bc3bf70f64657658635e66094edbcb4d...90c1019714259b24fb81711d4416ac0f18667dfa",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "65f1bf27bc3bf70f64657658635e66094edbcb4d",
					BaseFullRef:   git.RefName("65f1bf27bc3bf70f64657658635e66094edbcb4d"),
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					HeadOriRef:    "90c1019714259b24fb81711d4416ac0f18667dfa",
					HeadFullRef:   git.RefName("90c1019714259b24fb81711d4416ac0f18667dfa"),
					DotTimes:      3,
				},
				BaseRepo:     repo1,
				HeadUser:     repo1.Owner,
				HeadRepo:     repo1,
				HeadGitRepo:  gitRepo1,
				IsBaseCommit: true,
				IsHeadCommit: true,
			},
		},
		{
			repoName: "repo1",
			router:   "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2^...985f0301dba5e7b34be866819cd15ad3d8f508ee",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    "5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2",
					BaseFullRef:   git.RefName("5c050d3b6d2db231ab1f64e324f1b6b9a0b181c2"),
					HeadOwnerName: repo1.Owner.Name,
					HeadRepoName:  repo1.Name,
					HeadOriRef:    "985f0301dba5e7b34be866819cd15ad3d8f508ee",
					HeadFullRef:   git.RefName("985f0301dba5e7b34be866819cd15ad3d8f508ee"),
					DotTimes:      3,
					CaretTimes:    1,
				},
				BaseRepo:     repo1,
				HeadUser:     repo1.Owner,
				HeadRepo:     repo1,
				HeadGitRepo:  gitRepo1,
				IsBaseCommit: true,
				IsHeadCommit: true,
			},
		},
		{
			repoName: "repo1",
			hasClose: true,
			router:   "user12/repo10:master",
			compareInfo: &CompareInfo{
				CompareRouter: &CompareRouter{
					BaseOriRef:    repo11.DefaultBranch,
					BaseFullRef:   git.RefNameFromBranch(repo11.DefaultBranch),
					HeadOwnerName: "user12",
					HeadRepoName:  "repo10",
					HeadOriRef:    "master",
					HeadFullRef:   git.RefNameFromBranch("master"),
					DotTimes:      3,
				},
				BaseRepo:    repo1,
				HeadUser:    repo10.Owner,
				HeadRepo:    repo10,
				HeadGitRepo: gitRepo10,
			},
		},
	}

	for _, kase := range kases {
		t.Run(kase.router, func(t *testing.T) {
			var baseRepo *repo_model.Repository
			var baseGitRepo *git.Repository
			if kase.repoName == "repo1" {
				baseRepo = repo1
				baseGitRepo = gitRepo1
			} else if kase.repoName == "repo10" {
				baseRepo = repo10
				baseGitRepo = gitRepo10
			} else if kase.repoName == "repo11" {
				baseRepo = repo11
				baseGitRepo = gitRepo11
			} else {
				t.Fatalf("unknown repo name: %s", kase.router)
			}
			r, err := ParseComparePathParams(t.Context(), kase.router, baseRepo, baseGitRepo)
			assert.NoError(t, err)
			if kase.hasClose {
				assert.NotNil(t, r.close)
				r.close = nil // close is a function, so we can't compare it
			}
			assert.Equal(t, *kase.compareInfo.CompareRouter, *r.CompareRouter)
			assert.Equal(t, *kase.compareInfo.BaseRepo, *r.BaseRepo)
			assert.Equal(t, *kase.compareInfo.HeadUser, *r.HeadUser)
			assert.Equal(t, *kase.compareInfo.HeadRepo, *r.HeadRepo)
			assert.Equal(t, kase.compareInfo.HeadGitRepo.Path, r.HeadGitRepo.Path)
			assert.Equal(t, kase.compareInfo.IsBaseCommit, r.IsBaseCommit)
			assert.Equal(t, kase.compareInfo.IsHeadCommit, r.IsHeadCommit)
		})
	}
}
