// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"net/url"
	"slices"
	"testing"

	"gitea.dev/models/db"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/models/organization"
	"gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	project_model "gitea.dev/models/project"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/base"
	"gitea.dev/modules/templates"
	"gitea.dev/services/contexttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetrieveRepoIssueMetaDataWithPrefill(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	require.NoError(t, db.Insert(t.Context(),
		[]*issues_model.Label{
			{ID: 100, RepoID: 1, Name: "Bug"},
			{ID: 101, RepoID: 1, Name: "Feature Request"},
			{ID: 102, RepoID: 2, Name: "Other Label"},
		},
		[]*issues_model.Milestone{
			{ID: 100, RepoID: 1, Name: "Release One"},
			{ID: 101, RepoID: 1, Name: "Release Two"},
			{ID: 102, RepoID: 1, Name: "Closed Milestone", IsClosed: true},
			{ID: 103, RepoID: 2, Name: "Other Milestone"},
		},
		[]*project_model.Project{
			{ID: 100, RepoID: 1, Title: "Open Project", Type: project_model.TypeRepository},
			{ID: 101, RepoID: 1, Title: "Closed Project", Type: project_model.TypeRepository, IsClosed: true},
			{ID: 102, RepoID: 2, Title: "Other Project", Type: project_model.TypeRepository},
		},
		[]*user_model.User{
			{ID: 100, Name: "Alice", LowerName: "alice", IsActive: true},
			{ID: 101, Name: "Bob", LowerName: "bob", IsActive: true},
			{ID: 102, Name: "OtherUser", LowerName: "otheruser", IsActive: true},
		},
		[]*access_model.Access{
			{RepoID: 1, UserID: 100, Mode: perm.AccessModeWrite},
			{RepoID: 1, UserID: 101, Mode: perm.AccessModeWrite},
		},
	))

	type testCase struct {
		name        string
		query       url.Values
		labelIDs    []int64
		milestoneID int64
		projectIDs  []int64
		assigneeIDs []int64
	}
	for _, tc := range []testCase{
		{
			name: "empty",
		},
		{
			name: "case-insensitive names",
			query: url.Values{
				"label":     {"bUG,fEATURE rEQUEST"},
				"milestone": {"rELEASE oNE"},
				"project":   {"cLOSED pROJECT,oPEN pROJECT"},
				"assignee":  {"bOB,aLICE"},
			},
			labelIDs:    []int64{100, 101},
			milestoneID: 100,
			projectIDs:  []int64{101, 100},
			assigneeIDs: []int64{101, 100},
		},
		{
			name: "mixed IDs and names with invalid values",
			query: url.Values{
				"label":     {"missing,102,Other Label,101,Bug,100,999"},
				"milestone": {"missing,102,Closed Milestone,103,Other Milestone,101,Release One,999"},
				"project":   {"missing,102,Other Project,101,Open Project,999"},
				"assignee":  {"missing,102,OtherUser,101,Alice,999"},
			},
			labelIDs:    []int64{100, 101},
			milestoneID: 102,
			projectIDs:  []int64{101, 100},
			assigneeIDs: []int64{101, 100},
		},
		{
			name:     "labels fallback",
			query:    url.Values{"labels": {"101,Bug"}},
			labelIDs: []int64{100, 101},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, _ := contexttest.MockContext(t, "/dummy?"+tc.query.Encode())
			contexttest.LoadUser(t, ctx, 2)
			contexttest.LoadRepo(t, ctx, 1)

			data := retrieveRepoIssueMetaDataWithPrefill(ctx, ctx.Repo.Repository, false)
			require.False(t, ctx.Written())
			assert.Same(t, data, ctx.Data["IssuePageMetaData"])
			assert.True(t, data.CanModifyIssueOrPull)
			assert.Equal(t, base.JoinInt64s(tc.labelIDs, ","), data.LabelsData.SelectedLabelIDs)
			for _, label := range data.LabelsData.AllLabels {
				assert.Equal(t, slices.Contains(tc.labelIDs, label.ID), label.IsChecked, label.Name)
			}
			assert.Equal(t, tc.milestoneID, data.MilestonesData.SelectedMilestoneID)
			assert.Equal(t, tc.projectIDs, data.ProjectsData.SelectedProjectIDs)
			var projectIDs []int64
			for _, card := range data.ProjectsData.ProjectCards {
				projectIDs = append(projectIDs, card.Project.ID)
			}
			assert.Equal(t, tc.projectIDs, projectIDs)
			assert.Equal(t, base.JoinInt64s(tc.assigneeIDs, ","), data.AssigneesData.SelectedAssigneeIDs)
		})
	}
}

func TestRetrieveReviewersDataCrossOrgTeam(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	org1 := &user_model.User{ID: 501, Name: "org1", LowerName: "org1", Type: user_model.UserTypeOrganization, IsActive: true}
	org2 := &user_model.User{ID: 502, Name: "org2", LowerName: "org2", Type: user_model.UserTypeOrganization, IsActive: true}
	team1 := &organization.Team{ID: 501, OrgID: org1.ID, Name: "admins", LowerName: "admins", AccessMode: perm.AccessModeAdmin}
	team2 := &organization.Team{ID: 502, OrgID: org2.ID, Name: "admins", LowerName: "admins", AccessMode: perm.AccessModeAdmin}
	repo := &repo_model.Repository{ID: 501, OwnerID: org1.ID, OwnerName: org1.Name, Name: "cross-org-repo", LowerName: "cross-org-repo"}
	issue := &issues_model.Issue{ID: 501, RepoID: repo.ID, Index: 1, PosterID: 2, IsPull: true, Title: "test pr"}
	pr := &issues_model.PullRequest{ID: 501, IssueID: issue.ID, BaseRepoID: repo.ID, HeadRepoID: repo.ID}
	review1 := &issues_model.Review{ID: 501, IssueID: issue.ID, Type: issues_model.ReviewTypeRequest, ReviewerTeamID: team1.ID}
	review2 := &issues_model.Review{ID: 502, IssueID: issue.ID, Type: issues_model.ReviewTypeRequest, ReviewerTeamID: team2.ID}

	require.NoError(t, db.Insert(t.Context(),
		[]*user_model.User{org1, org2},
		[]*organization.Team{team1, team2},
		[]*organization.TeamUser{{OrgID: org1.ID, TeamID: team1.ID, UID: 2}},
		[]*organization.TeamRepo{{OrgID: org1.ID, TeamID: team1.ID, RepoID: repo.ID}},
		repo,
		issue,
		pr,
		[]*issues_model.Review{review1, review2},
	))

	ctx, _ := contexttest.MockContext(t, "/org1/cross-org-repo/pulls/1", contexttest.MockContextOption{Render: templates.PageRenderer()})
	contexttest.LoadUser(t, ctx, 2)
	contexttest.LoadRepo(t, ctx, repo.ID)
	require.NoError(t, issue.LoadAttributes(ctx))

	data := retrieveRepoIssueMetaData(ctx, ctx.Repo.Repository, issue, true)
	require.False(t, ctx.Written())

	teamOrgByID := map[int64]string{}
	for _, item := range data.ReviewersData.CurrentPullReviewers {
		if item.Team != nil {
			teamOrgByID[item.Team.ID] = item.TeamOrgName
		}
	}
	assert.Equal(t, map[int64]string{
		team1.ID: "org1",
		team2.ID: "org2",
	}, teamOrgByID)

	rendered, err := ctx.RenderToHTML("repo/issue/sidebar/reviewer_list", data)
	require.NoError(t, err)
	assert.Contains(t, string(rendered), "org1/admins")
	assert.Contains(t, string(rendered), "org2/admins")
	assert.Equal(t, map[int64]string{
		team1.ID: "org1",
		team2.ID: "org2",
	}, func() map[int64]string {
		m := map[int64]string{}
		for _, item := range data.ReviewersData.TeamReviewers {
			if item.Team != nil {
				m[item.Team.ID] = item.TeamOrgName
			}
		}
		return m
	}())
}
