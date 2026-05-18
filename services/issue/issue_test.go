// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	project_model "code.gitea.io/gitea/models/project"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unit"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setRepoProjectsConfig writes a ProjectsConfig onto repo's projects unit.
func setRepoProjectsConfig(t *testing.T, repo *repo_model.Repository, cfg *repo_model.ProjectsConfig) {
	t.Helper()
	repoUnit, err := repo.GetUnit(t.Context(), unit.TypeProjects)
	require.NoError(t, err)
	repoUnit.Config = cfg
	_, err = db.GetEngine(t.Context()).ID(repoUnit.ID).Cols("config").Update(repoUnit)
	require.NoError(t, err)
}

func newRepoProjectFor(t *testing.T, repo *repo_model.Repository, title string) *project_model.Project {
	t.Helper()
	p := &project_model.Project{
		Title:        title,
		RepoID:       repo.ID,
		OwnerID:      repo.OwnerID,
		Type:         project_model.TypeRepository,
		TemplateType: project_model.TemplateTypeBasicKanban,
		CreatorID:    repo.OwnerID,
	}
	require.NoError(t, project_model.NewProject(t.Context(), p))
	return p
}

// GetDefaultProjectID is the single source of truth behind the new-issue/PR
// page pre-selection (there is no server-side auto-assignment). These are the
// behaviors that matter, tested at the fast service-unit layer rather than via
// a fragile compare-page integration test.
func TestGetDefaultProjectID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	issuesProj := newRepoProjectFor(t, repo, "Issues Default")
	prsProj := newRepoProjectFor(t, repo, "PRs Default")

	t.Run("issue and PR defaults are independent", func(t *testing.T) {
		setRepoProjectsConfig(t, repo, &repo_model.ProjectsConfig{
			ProjectsMode:                    repo_model.ProjectsModeRepo,
			DefaultProjectIDForIssues:       issuesProj.ID,
			DefaultProjectIDForPullRequests: prsProj.ID,
		})
		assert.Equal(t, issuesProj.ID, GetDefaultProjectID(t.Context(), repo, false))
		assert.Equal(t, prsProj.ID, GetDefaultProjectID(t.Context(), repo, true))
	})

	t.Run("unconfigured default resolves to 0", func(t *testing.T) {
		setRepoProjectsConfig(t, repo, &repo_model.ProjectsConfig{
			ProjectsMode:              repo_model.ProjectsModeRepo,
			DefaultProjectIDForIssues: 0,
		})
		assert.EqualValues(t, 0, GetDefaultProjectID(t.Context(), repo, false))
	})

	t.Run("closed default project resolves to 0", func(t *testing.T) {
		setRepoProjectsConfig(t, repo, &repo_model.ProjectsConfig{
			ProjectsMode:              repo_model.ProjectsModeRepo,
			DefaultProjectIDForIssues: issuesProj.ID,
		})
		require.NoError(t, project_model.ChangeProjectStatusByRepoIDAndID(t.Context(), repo.ID, issuesProj.ID, true))
		assert.EqualValues(t, 0, GetDefaultProjectID(t.Context(), repo, false),
			"a closed default must not be pre-selected")
	})
}

func TestGetRefEndNamesAndURLs(t *testing.T) {
	issues := []*issues_model.Issue{
		{ID: 1, Ref: "refs/heads/branch1"},
		{ID: 2, Ref: "refs/tags/tag1"},
		{ID: 3, Ref: "c0ffee"},
	}
	repoLink := "/foo/bar"

	endNames, urls := GetRefEndNamesAndURLs(issues, repoLink)
	assert.Equal(t, map[int64]string{1: "branch1", 2: "tag1", 3: "c0ffee"}, endNames)
	assert.Equal(t, map[int64]string{
		1: repoLink + "/src/branch/branch1",
		2: repoLink + "/src/tag/tag1",
		3: repoLink + "/src/commit/c0ffee",
	}, urls)
}

func TestIssue_DeleteIssue(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	issueIDs, err := issues_model.GetIssueIDsByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 5)

	issue := &issues_model.Issue{
		RepoID: 1,
		ID:     issueIDs[2],
	}

	_, err = deleteIssue(t.Context(), issue)
	assert.NoError(t, err)
	issueIDs, err = issues_model.GetIssueIDsByRepoID(t.Context(), 1)
	assert.NoError(t, err)
	assert.Len(t, issueIDs, 4)

	// check attachment removal
	attachments, err := repo_model.GetAttachmentsByIssueID(t.Context(), 4)
	assert.NoError(t, err)
	issue, err = issues_model.GetIssueByID(t.Context(), 4)
	assert.NoError(t, err)
	_, err = deleteIssue(t.Context(), issue)
	assert.NoError(t, err)
	assert.Len(t, attachments, 2)
	for i := range attachments {
		attachment, err := repo_model.GetAttachmentByUUID(t.Context(), attachments[i].UUID)
		assert.Error(t, err)
		assert.True(t, repo_model.IsErrAttachmentNotExist(err))
		assert.Nil(t, attachment)
	}

	// check issue dependencies
	user, err := user_model.GetUserByID(t.Context(), 1)
	assert.NoError(t, err)
	issue1, err := issues_model.GetIssueByID(t.Context(), 1)
	assert.NoError(t, err)
	issue2, err := issues_model.GetIssueByID(t.Context(), 2)
	assert.NoError(t, err)
	err = issues_model.CreateIssueDependency(t.Context(), user, issue1, issue2)
	assert.NoError(t, err)
	left, err := issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.False(t, left)

	_, err = deleteIssue(t.Context(), issue2)
	assert.NoError(t, err)
	left, err = issues_model.IssueNoDependenciesLeft(t.Context(), issue1)
	assert.NoError(t, err)
	assert.True(t, left)
}
