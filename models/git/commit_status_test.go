// Copyright 2017 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package git_test

import (
	"fmt"
	"testing"
	"time"

	"gitea.dev/models/db"
	git_model "gitea.dev/models/git"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/commitstatus"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestGetCommitStatuses(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	sha1 := "1234123412341234123412341234123412341234" // the mocked commit ID in test fixtures

	statuses, maxResults, err := db.FindAndCount[git_model.CommitStatus](t.Context(), &git_model.CommitStatusOptions{
		ListOptions: db.ListOptions{Page: 1, PageSize: 50},
		RepoID:      repo1.ID,
		SHA:         sha1,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, int(maxResults))
	assert.Len(t, statuses, 5)

	assert.Equal(t, "ci/awesomeness", statuses[0].Context)
	assert.Equal(t, commitstatus.CommitStatusPending, statuses[0].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[0].APIURL(t.Context()))

	assert.Equal(t, "cov/awesomeness", statuses[1].Context)
	assert.Equal(t, commitstatus.CommitStatusWarning, statuses[1].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[1].APIURL(t.Context()))

	assert.Equal(t, "cov/awesomeness", statuses[2].Context)
	assert.Equal(t, commitstatus.CommitStatusSuccess, statuses[2].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[2].APIURL(t.Context()))

	assert.Equal(t, "ci/awesomeness", statuses[3].Context)
	assert.Equal(t, commitstatus.CommitStatusFailure, statuses[3].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[3].APIURL(t.Context()))

	assert.Equal(t, "deploy/awesomeness", statuses[4].Context)
	assert.Equal(t, commitstatus.CommitStatusError, statuses[4].State)
	assert.Equal(t, "https://try.gitea.io/api/v1/repos/user2/repo1/statuses/1234123412341234123412341234123412341234", statuses[4].APIURL(t.Context()))

	statuses, maxResults, err = db.FindAndCount[git_model.CommitStatus](t.Context(), &git_model.CommitStatusOptions{
		ListOptions: db.ListOptions{Page: 2, PageSize: 50},
		RepoID:      repo1.ID,
		SHA:         sha1,
	})
	assert.NoError(t, err)
	assert.Equal(t, 5, int(maxResults))
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
					State: commitstatus.CommitStatusPending,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusSuccess,
				},
				{
					State: commitstatus.CommitStatusPending,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusSuccess,
				},
				{
					State: commitstatus.CommitStatusPending,
				},
				{
					State: commitstatus.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusPending,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusError,
				},
				{
					State: commitstatus.CommitStatusPending,
				},
				{
					State: commitstatus.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusFailure,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusWarning,
				},
				{
					State: commitstatus.CommitStatusPending,
				},
				{
					State: commitstatus.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusFailure,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusSuccess,
				},
				{
					State: commitstatus.CommitStatusSuccess,
				},
				{
					State: commitstatus.CommitStatusSuccess,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusSuccess,
			},
		},
		{
			statuses: []*git_model.CommitStatus{
				{
					State: commitstatus.CommitStatusFailure,
				},
				{
					State: commitstatus.CommitStatusError,
				},
				{
					State: commitstatus.CommitStatusWarning,
				},
			},
			expected: &git_model.CommitStatus{
				State: commitstatus.CommitStatusFailure,
			},
		},
	}

	for _, kase := range kases {
		assert.Equal(t, kase.expected, git_model.CalcCommitStatus(kase.statuses), "statuses: %v", kase.statuses)
	}
}

func TestFindRepoRecentCommitStatusContexts(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo2 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	gitRepo, err := git.OpenRepository(t.Context(), repo2)
	assert.NoError(t, err)
	defer gitRepo.Close()

	commit, err := gitRepo.GetBranchCommit(t.Context(), repo2.DefaultBranch)
	assert.NoError(t, err)

	defer func() {
		_, err := db.DeleteByBean(t.Context(), &git_model.CommitStatus{
			RepoID:    repo2.ID,
			CreatorID: user2.ID,
			SHA:       commit.ID.String(),
		})
		assert.NoError(t, err)
	}()

	err = git_model.NewCommitStatus(t.Context(), git_model.NewCommitStatusOptions{
		Repo:    repo2,
		Creator: user2,
		SHA:     commit.ID,
		CommitStatus: &git_model.CommitStatus{
			State:     commitstatus.CommitStatusFailure,
			TargetURL: "https://example.com/tests/",
			Context:   "compliance/lint-backend",
		},
	})
	assert.NoError(t, err)

	err = git_model.NewCommitStatus(t.Context(), git_model.NewCommitStatusOptions{
		Repo:    repo2,
		Creator: user2,
		SHA:     commit.ID,
		CommitStatus: &git_model.CommitStatus{
			State:     commitstatus.CommitStatusSuccess,
			TargetURL: "https://example.com/tests/",
			Context:   "compliance/lint-backend",
		},
	})
	assert.NoError(t, err)

	contexts, err := git_model.FindRepoRecentCommitStatusContexts(t.Context(), repo2.ID, time.Hour)
	assert.NoError(t, err)
	if assert.Len(t, contexts, 1) {
		assert.Equal(t, "compliance/lint-backend", contexts[0])
	}
}

func TestCommitStatusesApplyDoerPermission(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	// repo4 is public and has the actions unit, repo2 is private and owned by someone else
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 4})
	otherRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	doer := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: repo.OwnerID})

	visibleURL := repo.Link() + "/actions/runs/1/jobs/1"
	statuses := []*git_model.CommitStatus{
		{
			RepoID:    repo.ID,
			TargetURL: visibleURL,
		},
		{
			RepoID:    otherRepo.ID,
			TargetURL: otherRepo.Link() + "/actions/runs/1/jobs/1",
		},
		{
			RepoID:    repo.ID,
			TargetURL: "https://mycicd.org/1",
		},
	}

	git_model.CommitStatusesApplyDoerPermission(t.Context(), doer, statuses)
	assert.Equal(t, visibleURL, statuses[0].TargetURL)
	assert.Empty(t, statuses[1].TargetURL)
	assert.Equal(t, "https://mycicd.org/1", statuses[2].TargetURL)
}

func TestGetCountLatestCommitStatus(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	repo1 := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	sha1 := "1234123412341234123412341234123412341234" // the mocked commit ID in test fixtures

	commitStatuses, err := git_model.GetLatestCommitStatus(t.Context(), repo1.ID, sha1, db.ListOptions{
		Page:     1,
		PageSize: 2,
	})
	assert.NoError(t, err)
	assert.Len(t, commitStatuses, 2)
	assert.Equal(t, commitstatus.CommitStatusFailure, commitStatuses[0].State)
	assert.Equal(t, "ci/awesomeness", commitStatuses[0].Context)
	assert.Equal(t, commitstatus.CommitStatusError, commitStatuses[1].State)
	assert.Equal(t, "deploy/awesomeness", commitStatuses[1].Context)

	count, err := git_model.CountLatestCommitStatus(t.Context(), repo1.ID, sha1)
	assert.NoError(t, err)
	assert.EqualValues(t, 3, count)
}

func TestGetLatestCommitStatusForRepoCommitIDs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	sha1 := "1234123412341234123412341234123412341234" // the mocked commit ID in test fixtures

	var commitIDs []string
	for i := range 60 { // pad so that sha1 lands in a later query batch
		commitIDs = append(commitIDs, fmt.Sprintf("%040d", i))
	}
	commitIDs = append(commitIDs, sha1)

	statusMap, err := git_model.GetLatestCommitStatusForRepoCommitIDs(t.Context(), 1, commitIDs)
	assert.NoError(t, err)
	assert.Len(t, statusMap, 1)

	states := make([]commitstatus.CommitStatusState, 0, 3)
	for _, status := range statusMap[sha1] {
		states = append(states, status.State)
	}
	assert.ElementsMatch(t, []commitstatus.CommitStatusState{
		commitstatus.CommitStatusFailure, // ci/awesomeness, index 4
		commitstatus.CommitStatusSuccess, // cov/awesomeness, index 3
		commitstatus.CommitStatusError,   // deploy/awesomeness, index 5
	}, states)

	pairStatuses, err := git_model.GetLatestCommitStatusForPairs(t.Context(), []git_model.RepoSHA{
		{RepoID: 1, SHA: sha1},
		{RepoID: 2, SHA: sha1},
	})
	assert.NoError(t, err)
	assert.Len(t, pairStatuses, 1)
	assert.Len(t, pairStatuses[1], 3)
}

func TestGetLatestCommitStatusForRepoAndSHAs(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	sha1 := "1234123412341234123412341234123412341234" // the mocked commit ID in test fixtures
	sha2 := "2345234523452345234523452345234523452345"

	// a second repository, plus a decoy status at repo 1 / sha2 that is never asked for:
	// matching repositories and SHAs as a cross product would wrongly return it
	for _, repoID := range []int64{1, 2} {
		assert.NoError(t, db.Insert(t.Context(), &git_model.CommitStatus{
			Index:       6,
			RepoID:      repoID,
			SHA:         sha2,
			State:       commitstatus.CommitStatusSuccess,
			Context:     "ci/awesomeness",
			ContextHash: "c65f4d64a3b14a3eced0c9b36799e66e1bd5ced7",
			CreatorID:   2,
		}))
		assert.NoError(t, git_model.UpdateCommitStatusSummary(t.Context(), repoID, sha2))
	}
	assert.NoError(t, git_model.UpdateCommitStatusSummary(t.Context(), 1, sha1))

	statuses, err := git_model.GetLatestCommitStatusForRepoAndSHAs(t.Context(), []git_model.RepoSHA{
		{RepoID: 1, SHA: sha1},
		{RepoID: 2, SHA: sha2},
		{RepoID: 2, SHA: sha1}, // a repository asked for with two SHAs must still be matched by both
	})
	assert.NoError(t, err)

	pairs := make([]git_model.RepoSHA, 0, len(statuses))
	for _, status := range statuses {
		pairs = append(pairs, git_model.RepoSHA{RepoID: status.RepoID, SHA: status.SHA})
		if status.RepoID == 2 {
			assert.Equal(t, commitstatus.CommitStatusSuccess, status.State)
		}
	}
	assert.ElementsMatch(t, []git_model.RepoSHA{{RepoID: 1, SHA: sha1}, {RepoID: 2, SHA: sha2}}, pairs)

	statuses, err = git_model.GetLatestCommitStatusForRepoAndSHAs(t.Context(), nil)
	assert.NoError(t, err)
	assert.Empty(t, statuses)
}
