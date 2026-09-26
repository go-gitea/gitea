// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"fmt"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setRepoDependencies(t *testing.T, repoID int64, enabled bool) {
	t.Helper()
	repoUnit := unittest.AssertExistsAndLoadBean(t, &repo_model.RepoUnit{RepoID: repoID, Type: unit.TypeIssues})
	repoUnit.IssuesConfig().EnableDependencies = enabled
	require.NoError(t, repo_model.UpdateRepoUnitConfig(t.Context(), repoUnit))
}

func refsOf(issues []*issues_model.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, fmt.Sprintf("%d#%d", issue.RepoID, issue.Index))
	}
	return out
}

func ref(issue *issues_model.Issue) string {
	return fmt.Sprintf("%d#%d", issue.RepoID, issue.Index)
}

func TestLoadVisibleDependencies(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	publicRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})
	privateRepo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 2})
	require.False(t, publicRepo.IsPrivate)
	require.True(t, privateRepo.IsPrivate)
	setRepoDependencies(t, publicRepo.ID, true)
	setRepoDependencies(t, privateRepo.ID, true)

	issue1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 1})
	issue2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 2})
	issue3 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 3})
	privateIssue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: privateRepo.ID, Index: 1})

	admin := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	owner := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: publicRepo.OwnerID})
	outsider := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})

	// issue1 is blocked by issue2, issue3 and privateIssue
	require.NoError(t, issues_model.CreateIssueDependency(t.Context(), admin, issue1, issue2))
	require.NoError(t, issues_model.CreateIssueDependency(t.Context(), admin, issue1, issue3))
	require.NoError(t, issues_model.CreateIssueDependency(t.Context(), admin, issue1, privateIssue))

	t.Run("BothDirectionsForWholeList", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{issue1, issue2, issue3})
		require.NoError(t, err)

		assert.ElementsMatch(t, []string{ref(issue2), ref(issue3), ref(privateIssue)}, refsOf(deps[issue1.ID].BlockedBy))
		assert.Empty(t, deps[issue1.ID].Blocking)

		assert.Empty(t, deps[issue2.ID].BlockedBy)
		assert.Equal(t, []string{ref(issue1)}, refsOf(deps[issue2.ID].Blocking))

		assert.Empty(t, deps[issue3.ID].BlockedBy)
		assert.Equal(t, []string{ref(issue1)}, refsOf(deps[issue3.ID].Blocking))
	})

	t.Run("RepositoriesLoadedOnDependencies", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{issue1})
		require.NoError(t, err)
		for _, dep := range deps[issue1.ID].BlockedBy {
			require.NotNil(t, dep.Repo, "dependency %d must have Repo loaded", dep.Index)
			assert.NotEmpty(t, dep.Repo.OwnerName)
		}
	})

	t.Run("OutsiderCannotSeePrivateRepo", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: outsider}, issues_model.IssueList{issue1})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{ref(issue2), ref(issue3)}, refsOf(deps[issue1.ID].BlockedBy))
	})

	t.Run("AnonymousCannotSeePrivateRepo", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{}, issues_model.IssueList{issue1})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{ref(issue2), ref(issue3)}, refsOf(deps[issue1.ID].BlockedBy))
	})

	t.Run("PublicOnlyHidesPrivateRepoFromDoerWhoCouldRead", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner, PublicOnly: true}, issues_model.IssueList{issue1})
		require.NoError(t, err)
		assert.ElementsMatch(t, []string{ref(issue2), ref(issue3)}, refsOf(deps[issue1.ID].BlockedBy))
	})

	t.Run("DisabledDependenciesHideBlockedByOnly", func(t *testing.T) {
		setRepoDependencies(t, publicRepo.ID, false)
		defer setRepoDependencies(t, publicRepo.ID, true)

		// fresh copies so the cached repo units from earlier subtests do not leak in
		fresh1 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue1.ID})
		fresh2 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: issue2.ID})

		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{fresh1, fresh2})
		require.NoError(t, err)
		assert.Empty(t, deps[fresh1.ID].BlockedBy, "blocked_by follows the repo setting like GET /dependencies")
		assert.Equal(t, []string{ref(issue1)}, refsOf(deps[fresh2.ID].Blocking), "blocking ignores the repo setting like GET /blocks")
	})

	t.Run("SortedSameRepoFirstThenRepoIDThenNewest", func(t *testing.T) {
		pull5 := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 5})
		// the cross-repo blocker is created first, so a stable input order alone would put it first
		require.NoError(t, issues_model.CreateIssueDependency(t.Context(), admin, pull5, privateIssue))
		require.NoError(t, issues_model.CreateIssueDependency(t.Context(), admin, pull5, issue3))

		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{pull5})
		require.NoError(t, err)
		assert.Equal(t, []string{ref(issue3), ref(privateIssue)}, refsOf(deps[pull5.ID].BlockedBy))
	})

	t.Run("DanglingLinkIsSkipped", func(t *testing.T) {
		require.NoError(t, db.Insert(t.Context(), &issues_model.IssueDependency{
			UserID:       admin.ID,
			IssueID:      issue3.ID,
			DependencyID: 99999,
		}))

		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{issue3})
		require.NoError(t, err)
		assert.Empty(t, deps[issue3.ID].BlockedBy)
	})

	t.Run("EmptyListAndNoLinks", func(t *testing.T) {
		deps, err := LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{})
		require.NoError(t, err)
		assert.Empty(t, deps)

		unlinked := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{RepoID: publicRepo.ID, Index: 4})
		deps, err = LoadVisibleDependencies(t.Context(), LoadVisibleDependenciesOptions{Doer: owner}, issues_model.IssueList{unlinked})
		require.NoError(t, err)
		require.Contains(t, deps, unlinked.ID)
		assert.Empty(t, deps[unlinked.ID].BlockedBy)
		assert.Empty(t, deps[unlinked.ID].Blocking)
	})
}
