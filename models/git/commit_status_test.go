// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/db"
	git_model "code.gitea.io/gitea/models/git"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/gitrepo"
	"code.gitea.io/gitea/modules/structs"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	sha1 := "1234123412341234123412341234123412341234"

	statuses, maxResults, err := db.FindAndCount[git_model.CommitStatus](db.DefaultContext, &git_model.CommitStatusOptions{
		ListOptions: db.ListOptions{Page: 1, PageSize: 50},
		RepoID:      repo1.ID,
		SHA:         sha1,
	})
	assert.NoError(t, err)
	assert.Equal(t, int(maxResults), 5)
	assert.Len(t, statuses, 5)

	assert.Equal(t, "ci/awesomeness", statuses[0].Context)
	assert.Equal(t, structs.CommitStatusPending, statuses[0].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[0].APIURL(db.DefaultContext))

	assert.Equal(t, "cov/awesomeness", statuses[1].Context)
	assert.Equal(t, structs.CommitStatusWarning, statuses[1].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[1].APIURL(db.DefaultContext))

	assert.Equal(t, "cov/awesomeness", statuses[2].Context)
	assert.Equal(t, structs.CommitStatusSuccess, statuses[2].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[2].APIURL(db.DefaultContext))

	assert.Equal(t, "ci/awesomeness", statuses[3].Context)
	assert.Equal(t, structs.CommitStatusFailure, statuses[3].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[3].APIURL(db.DefaultContext))

	assert.Equal(t, "deploy/awesomeness", statuses[4].Context)
	assert.Equal(t, structs.CommitStatusError, statuses[4].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[4].APIURL(db.DefaultContext))

	statuses, maxResults, err = db.FindAndCount[git_model.CommitStatus](db.DefaultContext, &git_model.CommitStatusOptions{
		ListOptions: db.ListOptions{Page: 2, PageSize: 50},
		RepoID:      repo1.ID,
		SHA:         sha1,
	})
	assert.NoError(t, err)
	assert.Equal(t, int(maxResults), 5)
	assert.Empty(t, statuses)
}

func Test_CalcCommitStatus(t *testing.T) {
	kases := []struct {
		statuses []*git_model.CommitStatus
		expected *git_model.CommitStatus
	}{
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusPending,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusSuccess,
				},
				{
					State: structs.CommitStatusPending,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusSuccess,
				},
				{
					State: structs.CommitStatusPending,
				},
				{
					State: structs.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusError,
				},
				{
					State: structs.CommitStatusPending,
				},
				{
					State: structs.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusError,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusWarning,
				},
				{
					State: structs.CommitStatusPending,
				},
				{
					State: structs.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusWarning,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusSuccess,
				},
				{
					State: structs.CommitStatusSuccess,
				},
				{
					State: structs.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusSuccess,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: structs.CommitStatusFailure,
				},
				{
					State: structs.CommitStatusError,
				},
				{
					State: structs.CommitStatusWarning,
				},
			},
			expected: &git_model.CommitStatus{
				State: structs.CommitStatusError,
			},
		},
	}

	for _, kase := range kases {
		assert.Equal(t, kase.expected, git_model.CalcCommitStatus(kase.statuses))
	}
}

func TestFindRepoRecentCommitStatusContexts(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	gitRepo, err := gitrepo.OpenRepository(git.DefaultContext, repo2)
	assert.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(repo2.DefaultBranch)
	assert.NoError(t, err)

	defer func() {
		_, err := db.DeleteByBean(db.DefaultContext, &git_model.CommitStatus{
			RepoID:    repo2.ID,
			CreatorID: user2.ID,
			SHA:       commit.ID.String(),
		})
		assert.NoError(t, err)
	}()

	err = git_model.NewCommitStatus(db.DefaultContext, git_model.NewCommitStatusOptions{
		Repo:    repo2,
		Creator: user2,
		SHA:     commit.ID,
		CommitStatus: &git_model.CommitStatus{
			State:     structs.CommitStatusFailure,
			TargetURL: "https://example.com/tests/",
			Context:   "compliance/lint-backend",
		},
	})
	assert.NoError(t, err)

	err = git_model.NewCommitStatus(db.DefaultContext, git_model.NewCommitStatusOptions{
		Repo:    repo2,
		Creator: user2,
		SHA:     commit.ID,
		CommitStatus: &git_model.CommitStatus{
			State:     structs.CommitStatusSuccess,
			TargetURL: "https://example.com/tests/",
			Context:   "compliance/lint-backend",
		},
	})
	assert.NoError(t, err)

	contexts, err := git_model.FindRepoRecentCommitStatusContexts(db.DefaultContext, repo2.ID, time.Hour)
	assert.NoError(t, err)
	if assert.Len(t, contexts, 1) {
		assert.Equal(t, "compliance/lint-backend", contexts[0])
	}
}
